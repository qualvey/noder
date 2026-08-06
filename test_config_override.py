import io, json, secrets, sys, uuid
from pathlib import Path

# 控制台 UTF-8 输出，避免 Windows cp1252 打印中文报错
if hasattr(sys.stdout, 'reconfigure'):
    sys.stdout.reconfigure(encoding='utf-8')
    sys.stderr.reconfigure(encoding='utf-8')

from fastapi import HTTPException
from sqlmodel import Session
from main import (
    engine, create_db_and_tables, Node, User,
    create_node, create_user, get_singbox_config,
    parse_config_override, ALLOWED_OVERRIDE_KEYS,
    NodeCreate, UserCreate
)

def reset_db():
    db = Path(__file__).parent / "data.db"
    if db.exists():
        db.unlink()
    create_db_and_tables()

def test_config_override_workflow():
    reset_db()
    with Session(engine) as session:
        # 建一个节点
        node = create_node(NodeCreate(
            node_name="HK TUIC Node",
            protocol="tuic", server_address="hk.example.com",
            server_port=8443, security="tls", sni="hk.example.com",
            is_active=True
        ), session)

        token = "override-token-test"
        user = create_user(UserCreate(
            name="OverrideUser", token=token,
            uuid=str(uuid.uuid4()), password=secrets.token_hex(8),
            node_ids=[node.id], is_active=True
        ), session)
        assert user.config_override is None, "default config_override should be None"

        # 1. 非法 JSON 应被拒
        try:
            parse_config_override("{not valid json")
            assert False, "invalid json should raise"
        except HTTPException as e:
            assert e.status_code == 400
            print("OK: invalid json rejected ->", e.detail)

        # 2. 非白名单键应被拒
        try:
            parse_config_override(json.dumps({"outbounds": []}))
            assert False, "outbounds should be rejected"
        except HTTPException as e:
            assert e.status_code == 400
            print("OK: outbounds rejected ->", e.detail)

        # 3. 合法 override (route+dns) 应通过
        override = {
            "route": {"final": "Proxy", "rules": [{"port": 853, "action": "reject"}]},
            "dns": {"servers": [{"tag": "google", "type": "https", "server": "8.8.8.8"}]}
        }
        parsed = parse_config_override(json.dumps(override))
        assert set(parsed.keys()) == {"route", "dns"}
        print("OK: valid override parsed:", set(parsed.keys()))

        # 4. 通过 create_user 传 override，验证返回与落库
        user2 = create_user(UserCreate(
            name="OverrideUser2", token="override-token-test-2",
            uuid=str(uuid.uuid4()), password=secrets.token_hex(8),
            node_ids=[node.id], is_active=True,
            config_override=json.dumps(override)
        ), session)
        assert user2.config_override is not None
        saved = json.loads(user2.config_override)
        assert set(saved.keys()) == {"route", "dns"}
        print("OK: create_user persisted override keys:", set(saved.keys()))

        # 5. /sub 生成配置应应用 override (route/dns 被整体替换)
        config = get_singbox_config("override-token-test-2", session)
        assert config["route"]["final"] == "Proxy", "route.final should be overridden"
        assert config["route"]["rules"][0]["port"] == 853, "route.rules should be overridden"
        assert config["dns"]["servers"][0]["server"] == "8.8.8.8", "dns.servers should be overridden"
        # 模板的其他键 (log/services/inbounds/outbounds) 保留
        assert "log" in config and "inbounds" in config
        # outbounds 节点注入仍生效
        assert any(o.get("tag") == "HK TUIC Node" for o in config["outbounds"])
        print("OK: /sub applied route+dns override, preserved template + node injection")

        # 6. 无 override 的用户不改变模板
        config_none = get_singbox_config(token, session)
        assert config_none["route"]["final"] == "Proxy"  # 模板默认 final 本来就是 Proxy
        print("OK: user without override keeps template (route.final=%s)" % config_none["route"]["final"])

        print("\nALL CONFIG_OVERRIDE TESTS PASSED SUCCESSFULLY!")

if __name__ == "__main__":
    test_config_override_workflow()
