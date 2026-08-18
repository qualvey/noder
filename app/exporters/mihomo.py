# -*- coding: utf-8 -*-
"""Mihomo (Clash.Meta) 代理导出器。

字段映射对齐 mihomo wiki 规范:
- VLESS + REALITY + vision: https://wiki.metacubex.one/config/proxies/vless/
- TUIC v5: https://wiki.metacubex.one/config/proxies/tuic/

导出守卫：mihomo 不支持的协议（当前 anytls）明确报错，不静默跳过。
"""
from typing import List

import yaml

from app.contracts import assert_protocol_supported, get_core, validate_node_contract
from app.models import Node, User


def build_mihomo_proxy(node: Node, user: User) -> dict:
    protocol = (node.protocol or "vless").lower()
    # 导出守卫：核心不支持该协议时明确报错（mihomo 当前不支持 anytls）
    assert_protocol_supported(get_core("mihomo"), protocol)
    # 老数据兜底：tag 为空时用 node_name（新数据契约强制 tag 必填）
    tag = node.tag or node.node_name

    # 导出守卫：节点 + 用户凭证合并视图按契约全量校验
    # （uuid/password 属用户级凭证，节点 CRUD 阶段不校验，导出时统一把关）
    user_uuid = user.uuid or user.token
    user_password = user.password or user.token
    merged = {
        **node.model_dump(),
        "uuid": user_uuid,
        "password": user_password,
        "tag": tag,
    }
    validate_node_contract(merged, protocol)

    proxy = {
        "name": tag,
        "type": protocol,
        "server": node.server_address,
        "port": node.server_port,
    }

    if protocol == "vless":
        proxy.update({
            "uuid": user_uuid,
            "network": "tcp",
            "udp": True,
            "tls": True,
            "servername": node.sni or node.server_address,
            "flow": node.flow or "xtls-rprx-vision",
            "reality-opts": {
                "public-key": node.public_key or "",
                "short-id": node.short_id or "",
            },
            "client-fingerprint": node.fingerprint or "chrome",
        })
    elif protocol == "tuic":
        proxy.update({
            "uuid": user_uuid,
            "password": user_password,
            "alpn": ["h3"],
            "congestion-controller": node.congestion_control or "bbr",
            "udp-relay-mode": "native",
            # 与 sing-box 导出一致: zero_rtt_handshake=False
            "reduce-rtt": False,
            "tuic-version": 5,
        })

    return proxy


def build_mihomo_proxies(nodes: List[Node], user: User) -> List[dict]:
    """按节点顺序生成 mihomo proxies 列表（只含启用节点）。"""
    return [build_mihomo_proxy(n, user) for n in nodes if n.is_active]


def build_mihomo_proxies_yaml(nodes: List[Node], user: User) -> str:
    """生成 mihomo proxies 段 YAML 文本（订阅端点与模板占位符共用）。"""
    proxies = build_mihomo_proxies(nodes, user)
    return yaml.safe_dump(
        proxies, allow_unicode=True, sort_keys=False, default_flow_style=False
    ).rstrip()
