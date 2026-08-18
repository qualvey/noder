# -*- coding: utf-8 -*-
"""用户侧订阅 API：Token 验证与多节点导出。"""
from typing import List

from fastapi import APIRouter, Depends, HTTPException, Query, Response, status
from sqlmodel import Session, select

from app.database import get_session
from app.exporters.mihomo import build_mihomo_config_yaml
from app.models import User
from app.services.singbox import build_singbox_outbound, generate_singbox_config

router = APIRouter(tags=["subscription"])


def _get_active_user(session: Session, token: str) -> User:
    user = session.exec(select(User).where(User.token == token)).first()
    if not user or not user.is_active:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid or inactive user token")
    return user


@router.get("/sub", summary="获取 Sing-Box 订阅配置")
def get_singbox_config(token: str = Query(..., description="用户鉴权 Token"), session: Session = Depends(get_session)):
    user = _get_active_user(session, token)
    active_nodes = [n for n in user.nodes if n.is_active]
    if not active_nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="No active nodes associated with this user")
    return generate_singbox_config(active_nodes, user)


@router.get("/node", summary="获取用户绑定的动态拼接节点")
def get_user_nodes(token: str = Query(..., description="用户鉴权 Token"), session: Session = Depends(get_session)):
    user = _get_active_user(session, token)
    active_nodes = [n for n in user.nodes if n.is_active]
    if not active_nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="No active nodes associated with this user")
    return [
        {
            "id": node.id,
            "node_name": node.node_name,
            "protocol": node.protocol,
            "server_address": node.server_address,
            "server_port": node.server_port,
            "outbound": build_singbox_outbound(node, user),
        }
        for node in active_nodes
    ]


@router.get("/mihomo", summary="获取 Mihomo 完整配置 (YAML)")
def get_mihomo_config(token: str = Query(..., description="用户鉴权 Token"), session: Session = Depends(get_session)):
    """返回完整 mihomo 配置 YAML：data/mihomo.yml 模板 + 节点注入 + 策略组接线。"""
    user = _get_active_user(session, token)
    active_nodes = [n for n in user.nodes if n.is_active]
    if not active_nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="No active nodes associated with this user")
    return Response(
        content=build_mihomo_config_yaml(active_nodes, user),
        media_type="text/yaml; charset=utf-8",
    )


@router.get("/api/user/verify", summary="用户 Token 验证与节点数据查询")
def verify_user_token(token: str = Query(..., description="用户 Token"), session: Session = Depends(get_session)):
    """用户侧 API：验证 Token、查库验证有效性，并返回所有绑定的节点字段与配置。"""
    user = _get_active_user(session, token)
    active_nodes = [n for n in user.nodes if n.is_active]
    if not active_nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="用户绑定的节点不存在或已被禁用")
    node_config = generate_singbox_config(active_nodes, user)
    return {
        "valid": True,
        "user_name": user.name,
        "token": user.token,
        "node_ids": [n.id for n in active_nodes],
        "nodes": [{"id": n.id, "node_name": n.node_name, "protocol": n.protocol} for n in active_nodes],
        "singbox_config": node_config,
    }
