# -*- coding: utf-8 -*-
"""Mihomo (Clash.Meta) 代理导出器。

字段映射对齐 mihomo wiki 规范:
- VLESS + REALITY + vision: https://wiki.metacubex.one/config/proxies/vless/
- TUIC v5: https://wiki.metacubex.one/config/proxies/tuic/

导出守卫：mihomo 不支持的协议（当前 anytls）明确报错，不静默跳过。
完整配置以 data/mihomo.yml 为模板（与 sing-box 的 template.json 同构）：
- 模板中节点型 proxies（vless/tuic/...）视为示例占位，替换为真实生成的节点
- 特殊型 proxies（type: dns 等）保留
- proxy-groups：select 组保留组引用 + 追加节点；url-test/fallback 组替换为节点列表
"""
from typing import List

import yaml

import app.config as cfg
from app.contracts import assert_protocol_supported, get_core, validate_node_contract
from app.models import Node, User

# 节点型协议（模板中视为示例占位，生成时替换）；特殊型（dns/direct/reject 等）保留
_NODE_PROXY_TYPES = {
    "vless", "tuic", "anytls", "vmess", "ss", "ssr", "trojan",
    "hysteria", "hysteria2", "wireguard", "snell", "ssh", "shadowtls",
}


def load_mihomo_template() -> dict:
    """读取 data/mihomo.yml 模板；缺失/损坏时降级为基本模板。"""
    path = cfg.MIHOMO_TEMPLATE_PATH
    if path.exists():
        try:
            with open(path, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f)
            if isinstance(data, dict):
                return data
        except Exception as e:
            print(f"Error loading mihomo template ({e}), falling back to basic template.")

    # 基本降级模板
    return {
        "mixed-port": 7890,
        "log-level": "info",
        "mode": "rule",
        "proxies": [],
        "proxy-groups": [
            {"name": "Proxy", "type": "select", "proxies": ["auto"]},
            {
                "name": "auto", "type": "url-test", "proxies": [],
                "url": "http://www.gstatic.com/generate_204", "interval": 300,
            },
        ],
        "rules": ["MATCH,Proxy"],
    }


def generate_mihomo_config(nodes: List[Node], user: User) -> dict:
    """生成完整 mihomo 配置：模板 + 真实节点注入 + 策略组接线。"""
    config = load_mihomo_template()
    proxies = build_mihomo_proxies(nodes, user)  # 契约校验 + 导出守卫
    proxy_names = [p["name"] for p in proxies]

    # 1. proxies: 模板节点型示例替换为真实节点；特殊型保留；按 name 去重（生成值优先）
    kept = [p for p in (config.get("proxies") or []) if p.get("type") not in _NODE_PROXY_TYPES]
    merged_proxies: dict = {}
    for p in kept:
        merged_proxies.setdefault(p.get("name"), p)
    for p in proxies:
        merged_proxies[p["name"]] = p  # 生成的真实节点覆盖模板同名占位
    config["proxies"] = list(merged_proxies.values())

    # 2. proxy-groups: 组引用保留，示例节点引用剔除，真实节点接入
    groups = config.get("proxy-groups")
    if isinstance(groups, list):
        group_names = {g.get("name") for g in groups if isinstance(g, dict)}
        for g in groups:
            if not isinstance(g, dict):
                continue
            gname, gtype = g.get("name", ""), g.get("type", "")
            # 只保留「组引用」或「真实节点名」，示例引用（如 ss1/ss2）剔除
            members = [
                m for m in (g.get("proxies") or [])
                if m in group_names or m in proxy_names
            ]
            if gname == "Proxy" or gtype == "select":
                for n in proxy_names:
                    if n not in members and n != gname:
                        members.append(n)
            elif gtype in ("url-test", "fallback", "load-balance"):
                members = list(proxy_names)  # 测试组整体替换为节点列表
            g["proxies"] = members

    return config


def build_mihomo_config_yaml(nodes: List[Node], user: User) -> str:
    """生成完整 mihomo 配置 YAML 文本（订阅端点使用）。"""
    return yaml.safe_dump(
        generate_mihomo_config(nodes, user),
        allow_unicode=True, sort_keys=False, default_flow_style=False,
    ).rstrip()


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
