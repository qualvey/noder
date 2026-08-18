import os
import secrets
import sys
import tempfile
import uuid
from pathlib import Path

# Windows 控制台 UTF-8 输出，避免 cp1252 打印中文报错
if sys.stdout.encoding and sys.stdout.encoding.lower() not in ("utf-8", "utf8"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

from fastapi import HTTPException
from sqlmodel import Session
import app.config as cfg
import app.database as database
from app.models import Node, NodeCreate, User, UserCreate
from app.routers.nodes import create_node
from app.routers.subscription import get_singbox_config, get_user_nodes, verify_user_token
from app.routers.users import create_user

def test_full_workflow():
    # 临时库，避免污染/锁定 data.db（dev server 运行时也可跑）
    tmp_db = tempfile.mktemp(suffix=".db")
    cfg.DB_PATH = Path(tmp_db)
    database.DB_PATH = cfg.DB_PATH
    database.engine = database.create_engine(f"sqlite:///{tmp_db}", connect_args={"check_same_thread": False})
    database.create_db_and_tables()

    with Session(database.engine) as session:
        # 1. 测试 TUIC 协议非 TLS 被拦截
        invalid_tuic_data = NodeCreate(
            tag="invalid-tuic",
            node_name="Invalid TUIC Node",
            protocol="tuic",
            server_address="invalid.com",
            server_port=8443,
            security="none",
            is_active=True
        )
        try:
            create_node(invalid_tuic_data, session)
            assert False, "TUIC with non-tls security should throw 400 Exception"
        except HTTPException as e:
            assert e.status_code == 400
            print("Successfully caught invalid non-tls TUIC error:", e.detail)

        # 2. 测试 VLESS 非 REALITY (如 tls) 被拦截
        invalid_vless_sec = NodeCreate(
            tag="invalid-vless",
            node_name="Invalid VLESS TLS",
            protocol="vless",
            server_address="vless.example.com",
            server_port=443,
            security="tls",
            is_active=True
        )
        try:
            create_node(invalid_vless_sec, session)
            assert False, "VLESS with non-reality security should throw 400 Exception"
        except HTTPException as e:
            assert e.status_code == 400
            print("Successfully caught invalid non-reality VLESS error:", e.detail)

        # 3. 测试 VLESS REALITY 缺少 public_key / short_id 被拦截
        invalid_vless_data = NodeCreate(
            tag="invalid-vless-reality",
            node_name="Invalid VLESS REALITY",
            protocol="vless",
            server_address="vless.example.com",
            server_port=443,
            security="reality",
            is_active=True
        )
        try:
            create_node(invalid_vless_data, session)
            assert False, "VLESS REALITY without public_key/short_id should throw 400 Exception"
        except HTTPException as e:
            assert e.status_code == 400
            print("Successfully caught invalid VLESS REALITY missing key error:", e.detail)

        # 3. 创建合规的 TUIC 节点 (带管理员备注)
        tuic_data = NodeCreate(
            tag="hk-tuic-01",
            node_name="HK TUIC Node 01",
            protocol="tuic",
            server_address="hk.example.com",
            server_port=8443,
            security="tls",
            sni="hk.example.com",
            remark="测试机房 2026到期",
            is_active=True
        )
        tuic_node = create_node(tuic_data, session)
        assert tuic_node.remark == "测试机房 2026到期"
        
        # 4. 创建完全符合 doc/vless.md 规格的 VLESS REALITY 节点
        vless_data = NodeCreate(
            tag="bella-reality",
            node_name="bella-reality",
            protocol="vless",
            server_address="example.com",
            server_port=8443,
            security="reality",
            sni="aws.amazon.com",
            public_key="99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo",
            short_id="1a91",
            fingerprint="chrome",
            flow="xtls-rprx-vision",
            is_active=True
        )
        vless_node = create_node(vless_data, session)
        
        # 5. 创建 AnyTLS 节点
        anytls_data = NodeCreate(
            tag="sg-anytls-01",
            node_name="SG AnyTLS Node 01",
            protocol="anytls",
            server_address="sg.example.com",
            server_port=8443,
            security="tls",
            sni="sg.example.com",
            is_active=True
        )
        anytls_node = create_node(anytls_data, session)

        # 6. 创建用户并绑定节点
        user_uuid = "d1833187-177a-4ed3-ab68-e911a89d2d28"
        user_pwd = secrets.token_hex(8)
        user_token = f"multinode-token-{secrets.token_hex(4)}"
        
        user_data = UserCreate(
            name="ZhangSan",
            token=user_token,
            uuid=user_uuid,
            password=user_pwd,
            node_ids=[tuic_node.id, vless_node.id, anytls_node.id],
            is_active=True
        )
        user = create_user(user_data, session)
        
        # 7. 调用 /sub 函数验证生成的 VLESS REALITY Outbound 是否完全符合 doc/vless.md 定义
        config = get_singbox_config(user_token, session)
        outbounds = config.get("outbounds", [])
        
        vless_ob = next(o for o in outbounds if o["tag"] == "bella-reality")
        print("\nGenerated VLESS REALITY Outbound JSON:")
        print(vless_ob)

        assert vless_ob["type"] == "vless"
        assert vless_ob["server"] == "example.com"
        assert vless_ob["server_port"] == 8443
        assert vless_ob["uuid"] == user_uuid
        assert vless_ob["flow"] == "xtls-rprx-vision"

        tls = vless_ob["tls"]
        assert tls["enabled"] is True
        assert tls["server_name"] == "aws.amazon.com"
        
        # 验证 utls 结构
        assert tls["utls"]["enabled"] is True
        assert tls["utls"]["fingerprint"] == "chrome"

        # 8. 测试 /node 端点逻辑
        user_nodes = get_user_nodes(user_token, session)
        assert len(user_nodes) == 3
        print("\nGET /node API output count:", len(user_nodes))
        print("First node outbound tag:", user_nodes[0]["outbound"]["tag"])

        print("\nALL VLESS REALITY AND /node API SPECIFICATION TESTS PASSED SUCCESSFULLY!")

    try:
        os.remove(tmp_db)
    except OSError:
        pass

if __name__ == "__main__":
    test_full_workflow()
