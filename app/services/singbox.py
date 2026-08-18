# -*- coding: utf-8 -*-
"""Sing-Box 配置生成核心逻辑 (对齐 doc/vless.md 规格)。"""
import json
from typing import List

from app.config import ALLOWED_OVERRIDE_KEYS, TEMPLATE_PATH
from app.contracts import assert_protocol_supported, get_core, validate_node_contract
from app.models import Node, User, parse_config_override


def build_singbox_outbound(node: Node, user: User) -> dict:
    protocol = (node.protocol or "vless").lower()
    # 导出守卫：核心不支持该协议时明确报错（singbox 当前支持全部协议）
    assert_protocol_supported(get_core("singbox"), protocol)
    # 老数据兜底：tag 为空时用 node_name（新数据契约强制 tag 必填）
    tag = node.tag or node.node_name

    # 导出守卫：节点 + 用户凭证合并视图按契约全量校验
    # （uuid/password 属用户级凭证，节点 CRUD 阶段不校验，导出时统一把关）
    user_uuid = user.uuid or user.token
    user_password = user.password or user.token
    merged = {
        "tag": tag,
        "server_address": node.server_address,
        "server_port": node.server_port,
        "uuid": user_uuid,
        "password": user_password,
        "security": (node.security or "").lower(),
        "node_name": node.node_name,
        "flow": node.flow,
        "public_key": node.public_key,
        "short_id": node.short_id,
        "fingerprint": node.fingerprint,
        "congestion_control": node.congestion_control,
    }
    validate_node_contract(merged, protocol)

    outbound = {
        "type": protocol,
        "tag": tag,
        "server": node.server_address,
        "server_port": node.server_port,
    }

    if protocol == "tuic":
        outbound["uuid"] = user_uuid
        outbound["password"] = user_password
        outbound["congestion_control"] = node.congestion_control or "bbr"
        outbound["zero_rtt_handshake"] = False
        tls_config = {"enabled": True}
        if node.sni:
            tls_config["server_name"] = node.sni
        tls_config["alpn"] = ["h3"]
        outbound["tls"] = tls_config
    elif protocol == "vless":
        outbound["uuid"] = user_uuid
        outbound["flow"] = node.flow or "xtls-rprx-vision"

        if node.security in ["tls", "reality"]:
            tls_config = {"enabled": True}
            if node.sni:
                tls_config["server_name"] = node.sni

            # utls 开启
            tls_config["utls"] = {
                "enabled": True,
                "fingerprint": node.fingerprint or "chrome",
            }

            if node.security == "reality":
                tls_config["reality"] = {
                    "enabled": True,
                    "public_key": node.public_key or "",
                    "short_id": node.short_id or "",
                }
            outbound["tls"] = tls_config
    elif protocol == "anytls":
        outbound["uuid"] = user_uuid
        outbound["password"] = user_password
        if node.security in ["tls", "reality"]:
            tls_config = {"enabled": True}
            if node.sni:
                tls_config["server_name"] = node.sni
            if node.security == "reality":
                tls_config["reality"] = {
                    "enabled": True,
                    "public_key": node.public_key or "",
                    "short_id": node.short_id or "",
                }
            outbound["tls"] = tls_config

    if node.transport_type and node.transport_type != "direct":
        transport = {"type": node.transport_type}
        if node.path:
            transport["path"] = node.path
        outbound["transport"] = transport

    return outbound


def load_singbox_template() -> dict:
    """读取外置 template.json 模板文件。"""
    if TEMPLATE_PATH.exists():
        try:
            with open(TEMPLATE_PATH, "r", encoding="utf-8") as f:
                return json.load(f)
        except Exception as e:
            print(f"Error loading template.json ({e}), falling back to basic template.")

    # 基本降级模板
    return {
        "log": {"level": "info", "timestamp": True},
        "outbounds": [
            {"type": "direct", "tag": "direct", "routing_mark": 1024},
            {"tag": "Proxy", "type": "selector", "outbounds": ["urltest"]},
            {"tag": "urltest", "type": "urltest", "outbounds": []},
        ],
    }


def generate_singbox_config(nodes: List[Node], user: User) -> dict:
    active_nodes = [n for n in nodes if n.is_active]
    node_outbounds = [build_singbox_outbound(n, user) for n in active_nodes]
    node_tags = [n.node_name for n in active_nodes]

    # 读取外置 template.json
    config = load_singbox_template()
    outbounds = config.get("outbounds", [])

    # 1. 遍历模板 outbounds，将节点 tag 注入到 Proxy (selector) 与 urltest
    for ob in outbounds:
        if isinstance(ob, dict):
            tag = ob.get("tag", "")
            ob_type = ob.get("type", "")

            # selector 分组 (如 tag == "Proxy" 或 type == "selector")
            if tag == "Proxy" or ob_type == "selector":
                existing = ob.get("outbounds", [])
                for n_tag in node_tags:
                    if n_tag not in existing:
                        existing.append(n_tag)
                ob["outbounds"] = existing

            # urltest 分组 (如 tag == "urltest" 或 type == "urltest")
            elif tag == "urltest" or ob_type == "urltest":
                ob["outbounds"] = node_tags

    # 2. 将该用户的节点对象直接补充到 outbounds 数组末尾
    outbounds.extend(node_outbounds)
    config["outbounds"] = outbounds

    # 3. 应用该用户的 config_override (白名单字段整体覆盖，目前 route / dns)
    override = parse_config_override(user.config_override)
    if override:
        for key in ALLOWED_OVERRIDE_KEYS:
            if key in override:
                config[key] = override[key]

    return config
