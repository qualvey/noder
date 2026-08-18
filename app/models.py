# -*- coding: utf-8 -*-
"""SQLModel 数据库模型与 Pydantic Schema。

方案 3：节点单独分表 (不存凭证)，用户表存储凭证，支持用户关联多个节点。
"""
from datetime import datetime
import json
from typing import List, Optional

from fastapi import HTTPException, status
from sqlmodel import Field, Relationship, SQLModel

from app.config import ALLOWED_OVERRIDE_KEYS, ALLOWED_PROTOCOLS


class UserNodeLink(SQLModel, table=True):
    __table_args__ = {"extend_existing": True}
    user_id: Optional[int] = Field(default=None, foreign_key="user.id", primary_key=True)
    node_id: Optional[int] = Field(default=None, foreign_key="node.id", primary_key=True)


class NodeBase(SQLModel):
    node_name: str                              # 节点名称
    protocol: str = Field(default="vless")       # 仅限: tuic, vless, anytls
    server_address: str                         # 上游 IP / 域名
    server_port: int                            # 端口
    method: Optional[str] = None                 # 加密方式
    security: Optional[str] = "tls"              # auto, tls, reality, none
    sni: Optional[str] = None                    # TLS SNI / ServerName
    transport_type: Optional[str] = "direct"     # direct, ws, grpc, http
    path: Optional[str] = None                   # 传输路径
    is_active: bool = True                       # 是否启用

    # VLESS REALITY 专属与扩展属性
    public_key: Optional[str] = None             # REALITY 公钥
    short_id: Optional[str] = None               # REALITY Short ID
    fingerprint: Optional[str] = "chrome"        # uTLS 指纹 (默认 chrome)
    flow: Optional[str] = "xtls-rprx-vision"     # 流控 (默认 xtls-rprx-vision)
    remark: Optional[str] = Field(default=None)  # 管理员备注 (仅管理员可见)


class Node(NodeBase, table=True):
    __table_args__ = {"extend_existing": True}
    id: Optional[int] = Field(default=None, primary_key=True)
    users: List["User"] = Relationship(back_populates="nodes", link_model=UserNodeLink)


class NodeCreate(NodeBase):
    pass


class NodeRead(NodeBase):
    id: int


class NodeUpdate(SQLModel):
    node_name: Optional[str] = None
    protocol: Optional[str] = None
    server_address: Optional[str] = None
    server_port: Optional[int] = None
    method: Optional[str] = None
    security: Optional[str] = None
    sni: Optional[str] = None
    transport_type: Optional[str] = None
    path: Optional[str] = None
    is_active: Optional[bool] = None
    public_key: Optional[str] = None
    short_id: Optional[str] = None
    fingerprint: Optional[str] = None
    flow: Optional[str] = None
    remark: Optional[str] = None


class UserBase(SQLModel):
    name: str
    is_active: bool = True
    uuid: Optional[str] = Field(default=None)      # 用户专属 UUID (用于 VLESS / TUIC / AnyTLS)
    password: Optional[str] = Field(default=None)  # 用户专属密码 (用于 TUIC / AnyTLS 等)
    remark: Optional[str] = Field(default=None)    # 管理员备注 (仅管理员可见)
    config_override: Optional[str] = Field(default=None)  # JSON 文本：该用户专属配置覆盖 (目前支持 route/dns)


class User(UserBase, table=True):
    __table_args__ = {"extend_existing": True}
    id: Optional[int] = Field(default=None, primary_key=True)
    token: str = Field(index=True, unique=True)
    nodes: List[Node] = Relationship(back_populates="users", link_model=UserNodeLink)


class UserCreate(UserBase):
    token: Optional[str] = None     # 未传则自动生成 UUID
    uuid: Optional[str] = None      # 未传则自动生成 UUID
    password: Optional[str] = None  # 未传则自动生成随机密码
    node_ids: List[int] = Field(default_factory=list)


class UserRead(UserBase):
    id: int
    token: str
    uuid: Optional[str] = None
    password: Optional[str] = None
    node_ids: List[int] = Field(default_factory=list)
    config_override: Optional[str] = None


class UserUpdate(SQLModel):
    name: Optional[str] = None
    token: Optional[str] = None
    uuid: Optional[str] = None
    password: Optional[str] = None
    is_active: Optional[bool] = None
    remark: Optional[str] = None
    config_override: Optional[str] = None
    node_ids: Optional[List[int]] = None


class DistFileBase(SQLModel):
    name: str                                  # 显示名称
    file_type: str = "apk"                     # apk | zip | text
    template_name: Optional[str] = None         # ZIP 内模板文件相对路径 (仅 zip 使用)
    original_name: str = ""                    # 原始上传文件名 (下载时还原)
    size: int = 0                              # 字节数
    is_active: bool = True                     # 是否允许下载
    remark: Optional[str] = None               # 管理员备注
    source_url: Optional[str] = None           # 远程源链接 (远程拉取模式)
    cached_at: Optional[str] = None            # 最近一次成功拉取时间 (ISO 格式)
    created_at: str = Field(default_factory=lambda: datetime.now().strftime("%Y-%m-%d %H:%M:%S"))


class DistFile(DistFileBase, table=True):
    __table_args__ = {"extend_existing": True}
    id: Optional[int] = Field(default=None, primary_key=True)
    stored_name: str = Field(index=True)       # 磁盘存储文件名 (uuid_原名)


class DistFileUpdate(SQLModel):
    name: Optional[str] = None
    template_name: Optional[str] = None
    is_active: Optional[bool] = None
    remark: Optional[str] = None


# ------------------------------------------------------------------
# 校验辅助函数
# ------------------------------------------------------------------
def validate_node_protocol_and_security(protocol: str, security: Optional[str], public_key: Optional[str] = None, short_id: Optional[str] = None):
    proto = protocol.lower() if protocol else "vless"
    sec = (security or "").lower()

    if proto not in ALLOWED_PROTOCOLS:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Protocol '{protocol}' is invalid. Allowed protocols: {', '.join(ALLOWED_PROTOCOLS)}"
        )
    if proto == "tuic" and sec != "tls":
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="TUIC protocol strictly requires security mode to be 'tls'."
        )
    if proto == "vless":
        if sec != "reality":
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="VLESS protocol strictly requires security mode to be 'reality'."
            )
        if not public_key or not short_id:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="VLESS REALITY protocol strictly requires both 'public_key' and 'short_id'."
            )


def parse_config_override(raw: Optional[str]) -> Optional[dict]:
    """校验并规范化用户 config_override (JSON 字符串)。返回规范化后的 dict；为空/None 返回 None。"""
    if raw is None:
        return None
    s = (raw or "").strip()
    if not s:
        return None
    try:
        data = json.loads(s)
    except json.JSONDecodeError as e:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"config_override 必须是合法 JSON: {e}"
        )
    if not isinstance(data, dict):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="config_override 的 JSON 必须是对象 (object)"
        )
    # 只允许白名单内的键，防止覆盖 outbounds / log 等动态注入或系统字段
    for key in data.keys():
        if key not in ALLOWED_OVERRIDE_KEYS:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"config_override 不允许覆盖字段 '{key}'。允许的字段: {', '.join(sorted(ALLOWED_OVERRIDE_KEYS))}"
            )
    return data
