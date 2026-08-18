# -*- coding: utf-8 -*-
"""管理员 CRUD API - 节点管理 (/api/nodes)。"""
from typing import List

from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session, select

from app.contracts import validate_node_contract
from app.database import get_session
from app.deps import verify_admin_token
from app.models import Node, NodeCreate, NodeRead, NodeUpdate

router = APIRouter(
    prefix="/api/nodes",
    tags=["admin-nodes"],
    dependencies=[Depends(verify_admin_token)],
)


@router.post("", response_model=NodeRead, summary="创建节点")
def create_node(node_data: NodeCreate, session: Session = Depends(get_session)):
    validate_node_contract(node_data.model_dump(), node_data.protocol)
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
    # 合并后的完整字段用于契约校验（更新只传部分字段）
    merged = {**node.model_dump(), **update_dict}
    validate_node_contract(merged, target_proto)

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
