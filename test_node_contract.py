# -*- coding: utf-8 -*-
"""
节点结构契约测试 (test_node_contract.py)
1. 固定取值: tuic 必须 tls / vless 必须 reality
2. 条件依赖: vless reality 必须有 public_key + short_id
3. 必填字段 / 未知协议 / 两阶段校验 (节点级 vs 全量)
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
    # 节点级视图: 节点自有字段 (tag/地址/端口/security/...)
    node = {"tag": "n1", "node_name": "n", "server_address": "1.2.3.4", "server_port": 8443, "security": "tls", "is_active": True}
    # 全量视图: 节点 + 用户凭证 (uuid/password)
    full = {**node, "uuid": "11111111-2222-3333-4444-555555555555", "password": "secret"}

    # --- 节点级校验 (node_level=True) ---
    expect_400("节点级: tuic 缺 tag 拒绝", lambda: validate_node_contract({**node, "protocol": "tuic", "tag": None}, node_level=True), "tag")
    expect_400("节点级: tuic security!=tls 拒绝", lambda: validate_node_contract({**node, "protocol": "tuic", "security": "reality"}, node_level=True))
    validate_node_contract({**node, "protocol": "tuic", "security": "tls", "sni": "t.example.com"}, node_level=True)
    check("节点级: tuic 完整通过 (无需 uuid/password)", True)

    expect_400("节点级: vless security=tls 拒绝", lambda: validate_node_contract({**node, "protocol": "vless", "security": "tls"}, node_level=True))
    expect_400("节点级: vless reality 缺 public_key 拒绝", lambda: validate_node_contract({**node, "protocol": "vless", "security": "reality", "short_id": "1a91"}, node_level=True))
    expect_400("节点级: vless reality 缺 short_id 拒绝", lambda: validate_node_contract({**node, "protocol": "vless", "security": "reality", "public_key": "KEY"}, node_level=True))
    validate_node_contract({**node, "protocol": "vless", "security": "reality", "public_key": "KEY", "short_id": "1a91"}, node_level=True)
    check("节点级: vless reality 字段齐全通过", True)

    expect_400("节点级: anytls 缺 server_address 拒绝", lambda: validate_node_contract({**node, "protocol": "anytls", "server_address": None}, node_level=True))

    # --- 全量校验 (导出视图, node_level=False) ---
    expect_400("全量: tuic 缺 uuid/password 拒绝", lambda: validate_node_contract({**full, "protocol": "tuic", "uuid": None, "password": None}))
    validate_node_contract({**full, "protocol": "tuic", "security": "tls"})
    check("全量: tuic 凭证齐全通过", True)

    expect_400("全量: vless 缺 uuid 拒绝", lambda: validate_node_contract({**full, "protocol": "vless", "security": "reality", "public_key": "KEY", "short_id": "1a91", "uuid": None}))
    validate_node_contract({**full, "protocol": "vless", "security": "reality", "public_key": "KEY", "short_id": "1a91"})
    check("全量: vless 凭证齐全通过", True)

    # utls 分支说明: vless fixed=reality, security=utls 会被固定取值检查先行拦截
    expect_400("全量: vless security=utls 被 fixed=reality 拦截", lambda: validate_node_contract({**full, "protocol": "vless", "security": "utls", "public_key": "KEY", "short_id": "1a91"}), "reality")

    expect_400("缺少必填 server_address 拒绝", lambda: validate_node_contract({**node, "protocol": "tuic", "server_address": None}))
    expect_400("未知协议拒绝", lambda: validate_node_contract({**node, "protocol": "hysteria2"}))
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
        "tag": "bad-tuic", "node_name": "Bad TUIC", "protocol": "tuic", "server_address": "x.com",
        "server_port": 8443, "security": "none",
    })
    check("API 拒绝非法 tuic (400)", r.status_code == 400, r.text)
    check("错误信息含协议上下文", "tuic" in r.text)

    r = client.post("/api/nodes", headers=ADMIN, json={
        "node_name": "No Tag", "protocol": "tuic", "server_address": "x.com",
        "server_port": 8443, "security": "tls",
    })
    check("API 拒绝缺 tag 的 tuic (400)", r.status_code == 400, r.text)

    r = client.post("/api/nodes", headers=ADMIN, json={
        "tag": "bad-vless", "node_name": "Bad VLESS", "protocol": "vless", "server_address": "x.com",
        "server_port": 8443, "security": "reality",
    })
    check("API 拒绝缺公钥的 vless reality (400)", r.status_code == 400, r.text)

    r = client.post("/api/nodes", headers=ADMIN, json={
        "tag": "good-vless", "node_name": "Good VLESS", "protocol": "vless", "server_address": "x.com",
        "server_port": 8443, "security": "reality", "sni": "aws.amazon.com",
        "public_key": "99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo", "short_id": "1a91",
    })
    check("API 接受合规 vless reality (200)", r.status_code == 200, r.text)

    r = client.post("/api/nodes", headers=ADMIN, json={
        "tag": "good-tuic", "node_name": "Good TUIC", "protocol": "tuic", "server_address": "x.com",
        "server_port": 8443, "security": "tls", "congestion_control": "cubic",
    })
    check("API 接受合规 tuic (200, 无需 uuid/password)", r.status_code == 200, r.text)
    check("API 保存 congestion_control", r.status_code == 200 and r.json().get("congestion_control") == "cubic", r.text)

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
