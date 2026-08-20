# -*- coding: utf-8 -*-
"""管理员 CRUD API - 用户管理 (/api/users)。"""
import json
import secrets
from typing import List
import uuid as uuid_lib

from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session, select

from app.database import get_session
from app.deps import verify_admin_token
from app.models import Node, User, UserCreate, UserRead, UserUpdate, parse_config_override

router = APIRouter(
    prefix="/api/users",
    tags=["admin-users"],
    dependencies=[Depends(verify_admin_token)],
)

def _to_read(user: User) -> UserRead:
    assert user.id is not None, "User has not been committed to the database."
    return UserRead(
        id=user.id,
        name=user.name,
        is_active=user.is_active,
        token=user.token,
        uuid=user.uuid,
        password=user.password,
        node_ids=[n.id for n in user.nodes if n.id is not None],
        config_override=user.config_override,
    )


@router.post("", response_model=UserRead, summary="创建用户")
def create_user(user_data: UserCreate, session: Session = Depends(get_session)):
    token = user_data.token or str(uuid_lib.uuid4())
    user_uuid = user_data.uuid or str(uuid_lib.uuid4())
    user_pwd = user_data.password or secrets.token_hex(8)

    existing = session.exec(select(User).where(User.token == token)).first()
    if existing:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="Token already exists")

    bound_nodes = []
    if user_data.node_ids:
        bound_nodes = session.exec(select(Node).where(Node.id.in_(user_data.node_ids))).all()

    # 校验并规范化 config_override (合法 JSON 且仅含白名单键)
    override_json = None
    if user_data.config_override is not None:
        override_json = json.dumps(parse_config_override(user_data.config_override), ensure_ascii=False)

    user = User(
        name=user_data.name,
        token=token,
        uuid=user_uuid,
        password=user_pwd,
        is_active=user_data.is_active,
        config_override=override_json,
        nodes=bound_nodes,
    )
    session.add(user)
    session.commit()
    session.refresh(user)
    return _to_read(user)


@router.get("", response_model=List[UserRead], summary="获取用户列表")
def list_users(session: Session = Depends(get_session)):
    users = session.exec(select(User)).all()
    return [_to_read(u) for u in users]


@router.get("/{user_id}", response_model=UserRead, summary="获取单个用户")
def get_user(user_id: int, session: Session = Depends(get_session)):
    user = session.get(User, user_id)
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")
    return _to_read(user)

@router.put("/{user_id}", response_model=UserRead, summary="更新用户")
def update_user(user_id: int, user_data: UserUpdate, session: Session = Depends(get_session)):
    user = session.get(User, user_id)
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")

    update_dict = user_data.model_dump(exclude_unset=True)

    if "token" in update_dict and update_dict["token"] != user.token:
        existing = session.exec(select(User).where(User.token == update_dict["token"])).first()
        if existing:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="Token already exists")

    if "node_ids" in update_dict:
        node_ids = update_dict.pop("node_ids")
        if node_ids is not None:
            bound_nodes = session.exec(select(Node).where(Node.id.in_(node_ids))).all()
            user.nodes = bound_nodes

    # 校验并规范化 config_override；传空字符串/None 表示清除覆盖
    if "config_override" in update_dict:
        raw = update_dict.pop("config_override")
        override_json = None
        if raw is not None and (raw or "").strip():
            override_json = json.dumps(parse_config_override(raw), ensure_ascii=False)
        update_dict["config_override"] = override_json

    for key, value in update_dict.items():
        setattr(user, key, value)

    session.add(user)
    session.commit()
    session.refresh(user)
    return _to_read(user)


@router.delete("/{user_id}", summary="删除用户")
def delete_user(user_id: int, session: Session = Depends(get_session)):
    user = session.get(User, user_id)
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")

    session.delete(user)
    session.commit()
    return {"message": f"User {user_id} deleted successfully"}
