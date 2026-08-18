# -*- coding: utf-8 -*-
"""
节点结构契约测试 (test_node_contract.py)
1. 固定取值: tuic 必须 tls / vless 必须 reality
2. 条件依赖: vless reality 必须有 public_key + short_id
3. 必填字段 / 未知协议
4. API 层创建节点校验 (TestClient)
5. 核心注册表: 不支持协议明确报错
运行: uv run python test_node_contract.py
"""
import sys
import tempfile

# Windows 控制台 UTF-8 输出
if sys.stdout.encoding and sys.stdout.encoding.lower() not in ("utf-8", "utf8"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

import app.config as cfg  # noqa: E402
import app.database as database  # noqa: E402
from app.contracts import (  # noqa: E402
    PROTOCOL_CONTRACTS,
    assert_protocol_supported,
    get_core,
    validate_node_contract,
)
from fastapi import HTTPException  # noqa: E402
from fastapi.testclient import TestClient  # noqa: E402
import main as app_module  # noqa: E402

ADMIN = {"X-Admin-Token": "admin-secret"}
PASS = 0
FAIL = 0


def check(name, cond, extra=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  ✅ {name}")
    else:
        FAIL += 1
        print(f"  ❌ {name} {extra}")


def expect_400(name, fn, keyword=""):
    try:
        fn()
        check(name, False, "expected HTTPException 400 but none raised")
    except HTTPException as e:
        ok = e.status_code == 400 and (not keyword or keyword in str(e.detail))
        check(name, ok, f"status={e.status_code} detail={e.detail}")


def main():
    # ---------- 1. 契约单元校验 ----------
    print("== 1. 契约单元校验 ==")
    base = {"node_name": "n", "server_address": "1.2.3.4", "server_port": 8443, "security": "tls", "is_active": True}

    expect_400("tuic security!=tls 拒绝", lambda: validate_node_contract({**base, "protocol": "tuic", "security": "reality"}))
    validate_node_contract({**base, "protocol": "tuic", "security": "tls", "sni": "t.example.com"})
    check("tuic security=tls 通过", True)

    expect_400("vless security=tls 拒绝", lambda: validate_node_contract({**base, "protocol": "vless", "security": "tls"}))
    expect_400("vless reality 缺 public_key 拒绝", lambda: validate_node_contract({**base, "protocol": "vless", "security": "reality", "short_id": "1a91"}))
    expect_400("vless reality 缺 short_id 拒绝", lambda: validate_node_contract({**base, "protocol": "vless", "security": "reality", "public_key": "KEY"}))
    validate_node_contract({**base, "protocol": "vless", "security": "reality", "public_key": "KEY", "short_id": "1a91"})
    check("vless reality 字段齐全通过", True)

    expect_400("缺少必填 server_address 拒绝", lambda: validate_node_contract({**base, "protocol": "tuic", "server_address": None}))
    expect_400("未知协议拒绝", lambda: validate_node_contract({**base, "protocol": "hysteria2"}))
    check("契约表包含全部协议", set(PROTOCOL_CONTRACTS) == {"tuic", "vless", "anytls"})

    print("== 2. API 层创建节点校验 ==")
    # 临时库
    tmp_db = tempfile.mktemp(suffix=".db")
    tmp_db_path = cfg.BASE_DIR / tmp_db
    cfg.DB_PATH = tmp_db_path
    database.DB_PATH = tmp_db_path
    database.engine = database.create_engine(f"sqlite:///{tmp_db_path}", connect_args={"check_same_thread": False})
    database.create_db_and_tables()
    client = TestClient(app_module.app)

    r = client.post("/api/nodes", headers=ADMIN, json={
        "node_name": "Bad TUIC", "protocol": "tuic", "server_address": "x.com",
        "server_port": 8443, "security": "none",
    })
    check("API 拒绝非法 tuic (400)", r.status_code == 400, r.text)
    check("错误信息含协议上下文", "tuic" in r.text)

    r = client.post("/api/nodes", headers=ADMIN, json={
        "node_name": "Bad VLESS", "protocol": "vless", "server_address": "x.com",
        "server_port": 8443, "security": "reality",
    })
    check("API 拒绝缺公钥的 vless reality (400)", r.status_code == 400, r.text)

    r = client.post("/api/nodes", headers=ADMIN, json={
        "node_name": "Good VLESS", "protocol": "vless", "server_address": "x.com",
        "server_port": 8443, "security": "reality", "sni": "aws.amazon.com",
        "public_key": "99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo", "short_id": "1a91",
    })
    check("API 接受合规 vless reality (200)", r.status_code == 200, r.text)

    print("== 3. 核心注册表与导出守卫 ==")
    check("singbox 核心存在且支持全部协议", get_core("singbox").supported_protocols == {"tuic", "vless", "anytls"})
    expect_400("未知核心拒绝", lambda: get_core("nonsense"))

    # 模拟 mihomo 不支持 anytls 的场景
    from app.contracts import CoreInfo, CORE_REGISTRY
    CORE_REGISTRY["mihomo_test"] = CoreInfo("mihomo_test", "Mihomo", {"vless", "tuic"}, "text/yaml")
    expect_400("核心不支持协议明确报错", lambda: assert_protocol_supported(get_core("mihomo_test"), "anytls"), "Mihomo")
    assert_protocol_supported(get_core("mihomo_test"), "vless")
    check("核心支持协议放行", True)
    del CORE_REGISTRY["mihomo_test"]

    # 清理
    try:
        import os
        os.remove(tmp_db_path)
    except OSError:
        pass

    print(f"\n结果: {PASS} 通过, {FAIL} 失败")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
