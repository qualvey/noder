# -*- coding: utf-8 -*-
"""管理员 CRUD API - 节点管理 (/api/nodes)。"""
from typing import List

from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session, select

from app.database import get_session
from app.deps import verify_admin_token
from app.models import Node, NodeCreate, NodeRead, NodeUpdate, validate_node_protocol_and_security

router = APIRouter(
    prefix="/api/nodes",
    tags=["admin-nodes"],
    dependencies=[Depends(verify_admin_token)],
)


@router.post("", response_model=NodeRead, summary="创建节点")
def create_node(node_data: NodeCreate, session: Session = Depends(get_session)):
    validate_node_protocol_and_security(node_data.protocol, node_data.security, node_data.public_key, node_data.short_id)
    node = Node.model_validate(node_data)
    session.add(node)
    session.commit()
    session.refresh(node)
    return node


@router.get("", response_model=List[NodeRead], summary="获取节点列表")
def list_nodes(session: Session = Depends(get_session)):
    return session.exec(select(Node)).all()


@router.get("/{node_id}", response_model=NodeRead, summary="获取单个节点")
def get_node(node_id: int, session: Session = Depends(get_session)):
    node = session.get(Node, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    return node


@router.put("/{node_id}", response_model=NodeRead, summary="更新节点")
def update_node(node_id: int, node_data: NodeUpdate, session: Session = Depends(get_session)):
    node = session.get(Node, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")

    update_dict = node_data.model_dump(exclude_unset=True)
    target_proto = update_dict.get("protocol", node.protocol)
    target_sec = update_dict.get("security", node.security)
    target_pbk = update_dict.get("public_key", node.public_key)
    target_sid = update_dict.get("short_id", node.short_id)
    validate_node_protocol_and_security(target_proto, target_sec, target_pbk, target_sid)

    for key, value in update_dict.items():
        setattr(node, key, value)

    session.add(node)
    session.commit()
    session.refresh(node)
    return node


@router.delete("/{node_id}", summary="删除节点")
def delete_node(node_id: int, session: Session = Depends(get_session)):
    node = session.get(Node, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")

    session.delete(node)
    session.commit()
    return {"message": f"Node {node_id} deleted successfully"}
