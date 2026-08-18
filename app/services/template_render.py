# -*- coding: utf-8 -*-
"""模板渲染引擎：占位符替换 + 硬编码凭证智能替换 + ZIP 渲染。"""
import io
import json
import re
from pathlib import Path
from typing import Optional
import zipfile

import yaml

from app.models import User
from app.services.singbox import build_singbox_outbound


def build_template_context(user: User) -> dict:
    """构建模板占位符 -> 渲染值映射。未知占位符原样保留。"""
    nodes = [n for n in (user.nodes or []) if n.is_active]
    node_meta = [
        {
            "node_name": n.node_name,
            "protocol": n.protocol,
            "server": n.server_address,
            "server_port": n.server_port,
        }
        for n in nodes
    ]
    outbounds = [build_singbox_outbound(n, user) for n in nodes]

    def _yaml(obj) -> str:
        return yaml.safe_dump(obj, allow_unicode=True, sort_keys=False, default_flow_style=False).rstrip()

    return {
        "uuid": user.uuid or "",
        "password": user.password or "",
        "token": user.token,
        "name": user.name,
        "user_name": user.name,
        "node_list_yaml": _yaml(node_meta),
        "node_list_json": json.dumps(node_meta, ensure_ascii=False, indent=2),
        "outbounds_yaml": _yaml(outbounds),
        "outbounds_json": json.dumps(outbounds, ensure_ascii=False, indent=2),
    }


def render_template_text(text: str, user: User, known_tokens: Optional[set] = None) -> str:
    """将模板文本中的 {{占位符}} 替换为用户专属内容，并智能替换硬编码凭证。

    替换优先级：
    1. 显式 {{占位符}} (uuid/password/token/name/node_list/outbounds)
    2. 键名感知：token:/uuid:/password: 键后的 UUID 值按键替换 (兼容 yaml/json 写法)
    3. 兜底：任何等于已知用户 token 的 UUID 值替换为当前用户 token
    """
    ctx = build_template_context(user)
    for key, value in ctx.items():
        text = text.replace("{{" + key + "}}", str(value))

    # 2. 键名感知替换：token:/uuid: 等键后的硬编码 UUID 值
    def _keyed_repl(m):
        key_part, q_l, value, q_r = m.group(1), m.group(2), m.group(3), m.group(4)
        key_name = key_part.strip().strip('"\'').rstrip(":").strip().lower()
        if key_name.endswith("token"):
            return f"{key_part}{q_l}{user.token}{q_r}"
        if key_name.endswith("uuid"):
            return f"{key_part}{q_l}{user.uuid or ''}{q_r}"
        if key_name.endswith("password"):
            return f"{key_part}{q_l}{user.password or ''}{q_r}"
        return m.group(0)

    keyed_pattern = re.compile(
        r'(["\']?[\w.\-]*?(?:token|uuid|password)["\']?\s*[:=]\s*)(["\']?)([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})(["\']?)',
        re.IGNORECASE,
    )
    text = keyed_pattern.sub(_keyed_repl, text)

    # 3. 兜底：硬编码值若等于任一已知用户 token，替换为当前用户 token
    if known_tokens:
        for t in known_tokens:
            if t in text:
                text = text.replace(t, user.token)
    return text


def render_zip_for_user(zip_path: Path, template_name: Optional[str], user: User, known_tokens: Optional[set] = None) -> bytes:
    """读取 ZIP，渲染模板文件，其余文件原样，重新打包返回 bytes。"""
    target = Path(template_name).name if template_name else None
    output = io.BytesIO()
    with zipfile.ZipFile(zip_path, "r") as zin, zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as zout:
        for item in zin.infolist():
            data = zin.read(item.filename)
            if target and Path(item.filename).name == target:
                try:
                    data = render_template_text(data.decode("utf-8"), user, known_tokens).encode("utf-8")
                except UnicodeDecodeError:
                    pass  # 非文本模板文件，保持原样
            zout.writestr(item, data)
    output.seek(0)
    return output.getvalue()
