import json, os, sys, urllib.request, urllib.error

# Windows 控制台 UTF-8 输出，避免 cp1252 打印中文报错
if sys.stdout.encoding and sys.stdout.encoding.lower() not in ("utf-8", "utf8"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

# 可用环境变量 E2E_BASE 指向临时测试 server (默认 127.0.0.1:8000)
BASE = os.getenv("E2E_BASE", "http://127.0.0.1:8000")
ADMIN = {"X-Admin-Token": "admin-secret"}

def req(method, path, body=None, headers=None, expect_fail=False):
    h = dict(headers or {})
    data = None
    if body is not None:
        data = json.dumps(body).encode('utf-8')
        h['Content-Type'] = 'application/json'
    r = urllib.request.Request(BASE + path, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(r) as resp:
            return resp.status, json.loads(resp.read().decode('utf-8'))
    except urllib.error.HTTPError as e:
        if expect_fail:
            return e.code, json.loads(e.read().decode('utf-8'))
        raise

def main():
    # 1. 列节点，取一个 id
    st, nodes = req("GET", "/api/nodes", headers=ADMIN)
    assert st == 200 and nodes, "need at least one node"
    node_id = nodes[0]["id"]
    print(f"[1] nodes OK, using node id={node_id}")

    # 2. 非法 override: 覆盖 outbounds 应被拒 (400)
    st, err = req("POST", "/api/users", {
        "name": "Bad", "token": "bad-override-user",
        "node_ids": [node_id], "is_active": True,
        "config_override": json.dumps({"outbounds": []})
    }, headers=ADMIN, expect_fail=True)
    assert st == 400, f"expected 400, got {st}"
    print(f"[2] outbounds override rejected (400): {err.get('detail')}")

    # 3. 合法 override: route+dns，创建用户
    override = {
        "route": {"final": "Proxy", "rules": [{"port": 853, "action": "reject"}]},
        "dns": {"servers": [{"tag": "google", "server": "8.8.8.8", "type": "https"}]}
    }
    token = "e2e-override-user-token"
    st, user = req("POST", "/api/users", {
        "name": "E2E Override", "token": token, "node_ids": [node_id],
        "is_active": True, "config_override": json.dumps(override)
    }, headers=ADMIN)
    assert st == 200
    print("[3] created user with config_override, saved keys:", set(json.loads(user['config_override']).keys()))

    # 4. /sub 应返回覆盖后的 route/dns，且保留节点 outbound
    st, conf = req("GET", f"/sub?token={token}")
    assert st == 200
    assert conf["route"]["final"] == "Proxy", "route.final should be overridden"
    assert conf["route"]["rules"][0]["port"] == 853
    assert conf["dns"]["servers"][0]["server"] == "8.8.8.8"
    assert 'inbounds' in conf and 'log' in conf, "template sections preserved"
    assert any(o.get("tag") == nodes[0]["tag"] for o in conf["outbounds"]), "node injection preserved"
    print("[4] /sub applied route+dns override, preserved template + node injection. Final:", conf["route"]["final"])

    # 5. 更新用户：清空 override (传空) 应生效
    st, user2 = req("PUT", f"/api/users/{user['id']}", {
        "config_override": ""
    }, headers=ADMIN)
    assert st == 200 and user2["config_override"] is None
    print("[5] clearing config_override via PUT worked ->", user2["config_override"])

    # 清理测试用户
    req("DELETE", f"/api/users/{user['id']}", headers=ADMIN)
    print("[cleanup] deleted test user")

    print("\nALL END-TO-END HTTP TESTS PASSED!")

if __name__ == "__main__":
    main()
