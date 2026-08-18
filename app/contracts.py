# -*- coding: utf-8 -*-
"""节点结构契约 (Node Contract)。

定义每个协议节点的字段结构：哪些必须有、哪些可选、字段间依赖/互斥/取值域。
节点在创建/更新（核心读取阶段）时按契约严格校验，不合规直接拒绝。

未来接入新代理核心 (mihomo/xray 等) 时：
- 每个核心声明支持的协议列表 (CORE_REGISTRY)
- 导出阶段遇到不支持的协议 -> 明确报错，不静默跳过
- 数据层字段是超集，契约按协议维度组织，与核心无关
"""
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Tuple

from fastapi import HTTPException, status

from app.config import ALLOWED_PROTOCOLS


@dataclass
class NodeContract:
    """单协议字段契约。"""

    protocol: str
    # 必须存在的字段（值不能为空 None/''）
    required: set = field(default_factory=set)
    # 可选字段（仅用于文档/类型约束，校验不强制）
    optional: set = field(default_factory=set)
    # 条件依赖: {字段: {取值: [该取值下必须存在的字段]}}
    # 例: {"security": {"reality": ["public_key", "short_id"]}}
    deps: Dict[str, Dict[str, List[str]]] = field(default_factory=dict)
    # 固定取值: {字段: 必须等于的值}，例: {"security": "tls"}
    fixed: Dict[str, str] = field(default_factory=dict)
    # 取值白名单: {字段: [合法值]}，例: {"flow": ["", "xtls-rprx-vision"]}
    enum: Dict[str, List[str]] = field(default_factory=dict)
    # 互斥: [(字段A, 取值A, 字段B, 取值B)] 表示 A==取值A 时 B 不能等于取值B
    conflicts: List[Tuple[str, str, str, str]] = field(default_factory=list)


# ------------------------------------------------------------------
# 节点自有字段：Node 表单/表可直接提供的字段。
# uuid/password 属于用户级凭证（每个用户独立，见 models.py 方案 3），
# 节点 CRUD 校验只看自有字段；导出时由导出器合并用户凭证后全量校验。
# ------------------------------------------------------------------
NODE_OWNED_FIELDS = {
    "tag", "node_name", "server_address", "server_port", "security", "sni",
    "method", "transport_type", "path", "public_key", "short_id",
    "fingerprint", "flow", "remark", "congestion_control",
}

# 取值域常量（对齐 sing-box 定义，见 docs/configuration/shared/tls.zh.md、
# docs/configuration/outbound/vless.zh.md）
UTLS_FINGERPRINTS = [
    "chrome", "firefox", "edge", "safari", "360", "qq", "ios",
    "android", "random", "randomized",
]
VLESS_FLOWS = ["", "xtls-rprx-vision"]  # sing-vmess: 仅这两个合法
TUIC_CONGESTION_CONTROLS = ["bbr", "cubic", "new_reno"]

# ------------------------------------------------------------------
# 协议契约定义
# ------------------------------------------------------------------
PROTOCOL_CONTRACTS: Dict[str, NodeContract] = {
    "tuic": NodeContract(
        protocol="tuic",
        required={"tag","server_address", "server_port", "uuid","password"},
        optional={"congestion_control"},
        fixed={"security": "tls"},  # TUIC 严格要求 tls
        enum={"congestion_control": TUIC_CONGESTION_CONTROLS},
    ),
    "vless": NodeContract(
        protocol="vless",
        required={"tag", "server_address", "server_port", "uuid"},
        optional={"node_name", "flow"},
        fixed={"security": "reality"},  # VLESS 严格要求 reality
        deps={
            "security": {
                # REALITY 强制: 公钥 + short_id + SNI(伪装域名) + fingerprint
                # (sing-box: reality 客户端强制 utls -> fingerprint 必填,
                #  server_name 为空时 fallback 到 server 地址, REALITY 场景必然连不通)
                "reality": ["public_key", "short_id", "sni", "fingerprint"],
            },
        },
        enum={
            "flow": VLESS_FLOWS,
            "fingerprint": UTLS_FINGERPRINTS,
        },
    ),
    "anytls": NodeContract(
        protocol="anytls",
        required={"server_address", "server_port"},
        optional={"node_name", "method", "sni", "transport_type", "path", "remark"},
        deps={
            "security": {"tls": [], "reality": ["public_key", "short_id"]},
        },
    ),
}


def get_contract(protocol: str) -> NodeContract:
    """获取协议契约；未知协议抛 400。"""
    proto = (protocol or "").lower()
    if proto not in PROTOCOL_CONTRACTS:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Protocol '{protocol}' is invalid. Allowed protocols: {', '.join(ALLOWED_PROTOCOLS)}",
        )
    return PROTOCOL_CONTRACTS[proto]


def _is_empty(value) -> bool:
    return value is None or (isinstance(value, str) and not value.strip())


def validate_node_contract(
    values: dict, protocol: str, node_level: bool = False
) -> None:
    """按契约校验节点字段。

    :param values: 节点字段 dict（创建=全部；更新=现有值合并更新值；导出=节点+用户凭证合并视图）
    :param protocol: 协议名，必传（创建/更新/导出调用方都有；不依赖 values['protocol'] 兜底）
    :param node_level: True=节点创建/更新场景，必填字段只查节点自有字段
        （uuid/password 等用户级凭证由导出阶段合并后全量校验）
    校验失败抛 HTTPException 400。
    """
    contract = get_contract(protocol)

    # 1. 必填字段（node_level 时只查节点自有字段；用户级凭证在导出合并视图校验）
    required = contract.required if not node_level else contract.required & NODE_OWNED_FIELDS
    missing = [f for f in required if _is_empty(values.get(f))]
    if missing:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"[{protocol}] 缺少必填字段: {', '.join(missing)}",
        )

    # 2. 固定取值
    for f, must_be in contract.fixed.items():
        if str(values.get(f) or "").lower() != must_be:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"[{protocol}] 字段 '{f}' 必须为 '{must_be}'（当前: {values.get(f)}）",
            )

    # 3. 条件依赖
    for field, value_map in contract.deps.items():
        val = values.get(field)
        if val in value_map:
            missing_deps = [d for d in value_map[val] if _is_empty(values.get(d))]
            if missing_deps:
                raise HTTPException(
                    status_code=status.HTTP_400_BAD_REQUEST,
                    detail=(
                        f"[{protocol}] 字段 '{field}={val}' 时，必须同时提供: "
                        f"{', '.join(missing_deps)}"
                    ),
                )

    # 4. 枚举取值
    for f, allowed in contract.enum.items():
        val = values.get(f)
        if val is not None and str(val) not in allowed:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=(
                    f"[{protocol}] 字段 '{f}' 取值无效: '{val}'，"
                    f"允许: {', '.join(allowed)}"
                ),
            )

    # 5. 互斥
    for a, av, b, bv in contract.conflicts:
        if values.get(a) == av and values.get(b) == bv:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"[{protocol}] 字段 '{a}={av}' 与 '{b}={bv}' 互斥，不能同时存在",
            )


# ------------------------------------------------------------------
# 代理核心注册表 (导出器插件挂载点)
# 每个核心声明支持的协议；导出时遇到不支持的协议 -> 明确报错
# ------------------------------------------------------------------
class CoreInfo:
    def __init__(self, key: str, name: str, supported_protocols: set, content_type: str):
        self.key = key
        self.name = name
        self.supported_protocols = supported_protocols
        self.content_type = content_type


CORE_REGISTRY: Dict[str, CoreInfo] = {
    "singbox": CoreInfo(
        key="singbox",
        name="Sing-Box",
        supported_protocols=set(ALLOWED_PROTOCOLS),
        content_type="application/json",
    ),
    "mihomo": CoreInfo(
        key="mihomo",
        name="Mihomo",
        # 当前只支持 vless/tuic（anytls 等协议 mihomo 尚未支持，
        # 导出时由 assert_protocol_supported 明确拒绝；未来支持后加入此集合）
        supported_protocols={"vless", "tuic"},
        content_type="text/yaml",
    ),
}


def get_core(core: str) -> CoreInfo:
    key = (core or "singbox").lower()
    if key not in CORE_REGISTRY:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Unsupported core '{core}'. Available cores: {', '.join(CORE_REGISTRY)}",
        )
    return CORE_REGISTRY[key]


def assert_protocol_supported(core: CoreInfo, protocol: str) -> None:
    """导出守卫：核心不支持该协议时明确报错（不静默跳过）。"""
    if protocol not in core.supported_protocols:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=(
                f"核心 {core.name} 不支持协议 '{protocol}'"
                f"（该核心支持的协议: {', '.join(sorted(core.supported_protocols))}）。"
                "请修改或移除该节点。"
            ),
        )
