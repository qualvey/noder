# -*- coding: utf-8 -*-
"""数据库：engine、session、建表迁移、生命周期种子数据。"""
from contextlib import asynccontextmanager
import secrets
import uuid as uuid_lib

from fastapi import FastAPI
from sqlalchemy import text
from sqlmodel import Session, SQLModel, create_engine, select

from app.config import DB_PATH
from app.models import Node, User

sqlite_url = f"sqlite:///{DB_PATH}"
engine = create_engine(sqlite_url, connect_args={"check_same_thread": False})


def create_db_and_tables():
    """建表 + 兼容旧库的 ALTER 迁移（幂等，失败忽略）。"""
    SQLModel.metadata.create_all(engine)
    with engine.connect() as conn:
        for col in ["public_key", "short_id", "fingerprint", "flow", "remark"]:
            try:
                conn.execute(text(f"ALTER TABLE node ADD COLUMN {col} VARCHAR"))
                conn.commit()
            except Exception:
                pass
        for col in ["remark", "config_override"]:
            try:
                conn.execute(text(f"ALTER TABLE user ADD COLUMN {col} VARCHAR"))
                conn.commit()
            except Exception:
                pass
        for col in ["source_url", "cached_at"]:
            try:
                conn.execute(text(f"ALTER TABLE distfile ADD COLUMN {col} VARCHAR"))
                conn.commit()
            except Exception:
                pass


def get_session():
    with Session(engine) as session:
        yield session


def seed_default_data():
    """空库时预置演示节点与测试用户。"""
    with Session(engine) as session:
        if session.exec(select(Node)).first():
            return

        tuic_node = Node(
            node_name="TUIC 高速专线 01",
            protocol="tuic",
            server_address="tuic.example.com",
            server_port=8443,
            security="tls",
            sni="tuic.example.com",
            is_active=True,
        )
        vless_node = Node(
            node_name="VLESS REALITY 专线 02",
            protocol="vless",
            server_address="example.com",
            server_port=8443,
            security="reality",
            sni="aws.amazon.com",
            public_key="99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo",
            short_id="1a91",
            fingerprint="chrome",
            flow="xtls-rprx-vision",
            is_active=True,
        )
        anytls_node = Node(
            node_name="AnyTLS 极速专线 03",
            protocol="anytls",
            server_address="anytls.example.com",
            server_port=8443,
            security="tls",
            sni="anytls.example.com",
            is_active=True,
        )
        session.add_all([tuic_node, vless_node, anytls_node])
        session.commit()

        test_user = User(
            name="张三",
            token="my-secret-token",
            uuid=str(uuid_lib.uuid4()),
            password=secrets.token_hex(8),
            is_active=True,
            nodes=[tuic_node, vless_node, anytls_node],
        )
        session.add(test_user)
        session.commit()


@asynccontextmanager
async def lifespan(app: FastAPI):
    create_db_and_tables()
    seed_default_data()
    yield
