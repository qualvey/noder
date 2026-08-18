# -*- coding: utf-8 -*-
"""数据库：engine、session、建表迁移、生命周期种子数据。"""
from contextlib import asynccontextmanager
import secrets
import uuid as uuid_lib

from fastapi import FastAPI
from sqlalchemy import text
from sqlmodel import Session, SQLModel, create_engine, select

from app.config import DB_PATH
from app.models import AppSetting, Node, User

sqlite_url = f"sqlite:///{DB_PATH}"
engine = create_engine(sqlite_url, connect_args={"check_same_thread": False})


def create_db_and_tables():
    """建表 + 兼容旧库的 ALTER 迁移（幂等，失败忽略）。"""
    SQLModel.metadata.create_all(engine)
    with engine.connect() as conn:
        for col in ["public_key", "short_id", "fingerprint", "flow", "remark", "tag", "congestion_control"]:
            try:
                conn.execute(text(f"ALTER TABLE node ADD COLUMN {col} VARCHAR"))
                conn.commit()
            except Exception:
                pass
        # 老数据回填：tag 为空时用 node_name 兜底（契约要求 tag 必填）
        try:
            conn.execute(text("UPDATE node SET tag = node_name WHERE tag IS NULL OR tag = ''"))
            conn.commit()
        except Exception:
            pass
        for col in ["remark", "config_override"]:
            try:
                conn.execute(text(f"ALTER TABLE user ADD COLUMN {col} VARCHAR"))
                conn.commit()
            except Exception:
                pass
        for col in ["source_url", "cached_at", "download_name"]:
            try:
                conn.execute(text(f"ALTER TABLE distfile ADD COLUMN {col} VARCHAR"))
                conn.commit()
            except Exception:
                pass


def get_session():
    with Session(engine) as session:
        yield session


def seed_default_data():
    """空库时预置演示节点与测试用户；共享下载 token 始终确保存在。"""
    with Session(engine) as session:
        # 共享下载 token（普通文件/文本文件鉴权；缺失时生成，管理员可重置）
        if not session.get(AppSetting, "shared_download_token"):
            session.add(AppSetting(key="shared_download_token", value=secrets.token_hex(16)))
            session.commit()

        if session.exec(select(Node)).first():
            return

        tuic_node = Node(
            node_name="TUIC 高速专线 01",
            tag="tuic-01",
            protocol="tuic",
            server_address="tuic.example.com",
            server_port=8443,
            security="tls",
            sni="tuic.example.com",
            congestion_control="bbr",
            is_active=True,
        )
        vless_node = Node(
            node_name="VLESS REALITY 专线 02",
            tag="vless-02",
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
            tag="anytls-03",
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
