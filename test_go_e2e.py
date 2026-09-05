# -*- coding: utf-8 -*-
"""对 Go 后端 (noder.exe) 的全量 E2E 黑盒接口与行为契约回归测试。"""
import io
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
import zipfile
import time
import uuid

if sys.stdout.encoding and sys.stdout.encoding.lower() not in ("utf-8", "utf8"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

BASE = "http://127.0.0.1:8000"
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


def http_req(method, path, body=None, headers=None, raw_body=None, content_type="application/json"):
    h = dict(headers or {})
    data = None
    if raw_body is not None:
        data = raw_body
        h["Content-Type"] = content_type
    elif body is not None:
        data = json.dumps(body).encode("utf-8")
        h["Content-Type"] = "application/json"

    req = urllib.request.Request(BASE + path, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            content = resp.read()
            ct = resp.headers.get("Content-Type", "")
            cd = resp.headers.get("Content-Disposition", "")
            if "application/json" in ct:
                return resp.status, json.loads(content.decode("utf-8")), ct, cd
            return resp.status, content, ct, cd
    except urllib.error.HTTPError as e:
        content = e.read()
        try:
            return e.code, json.loads(content.decode("utf-8")), "", ""
        except Exception:
            return e.code, content, "", ""


def test_admin_auth():
    print("\n== 1. 管理员鉴权测试 ==")
    code, data, _, _ = http_req("GET", "/api/nodes")
    check("未提供 token 返回 401", code == 401)
    code, data, _, _ = http_req("GET", "/api/nodes", headers={"X-Admin-Token": "wrong"})
    check("错误 token 返回 401", code == 401)
    code, data, _, _ = http_req("GET", "/api/nodes", headers=ADMIN)
    check("正确 token 返回 200", code == 200 and isinstance(data, list))


def test_node_and_user_order():
    print("\n== 2. 节点与用户排序 (node_order) 测试 ==")
    rand_tag = uuid.uuid4().hex[:6]
    # 创建 2 个合规节点
    code, node_1, _, _ = http_req("POST", "/api/nodes", {
        "tag": f"hk-go-{rand_tag}",
        "protocol": "tuic",
        "server_address": "hk.go.com",
        "server_port": 8443,
        "security": "tls",
        "is_active": True
    }, headers=ADMIN)
    check("创建 TUIC 节点成功", code == 200)

    code, node_2, _, _ = http_req("POST", "/api/nodes", {
        "tag": f"jp-go-{rand_tag}",
        "protocol": "vless",
        "server_address": "jp.go.com",
        "server_port": 443,
        "security": "reality",
        "public_key": "pk123",
        "short_id": "sid123",
        "sni": "sni.jp.com",
        "is_active": True
    }, headers=ADMIN)
    check("创建 VLESS REALITY 节点成功", code == 200)

    # 节点契约阻断测试
    code, err, _, _ = http_req("POST", "/api/nodes", {
        "tag": "bad-vless",
        "protocol": "vless",
        "server_address": "jp.go.com",
        "server_port": 443,
        "security": "reality",
    }, headers=ADMIN)
    check("VLESS 缺少 reality 字段被契约阻断 (400)", code == 400)

    # 创建用户并将 node_2 排在首位
    u_tok = f"user-go-{rand_tag}"
    code, user, _, _ = http_req("POST", "/api/users", {
        "name": f"Go User {rand_tag}",
        "token": u_tok,
        "is_active": True,
        "node_ids": [node_2["id"], node_1["id"]]
    }, headers=ADMIN)
    check("创建用户并绑定自定义节点顺序", code == 200)
    check("node_ids 保持排定顺序 [node_2, node_1]", user.get("node_ids") == [node_2["id"], node_1["id"]])

    # /sub 测试：首位节点为 jp-go-rand_tag
    jp_tag = f"jp-go-{rand_tag}"
    code, sb_conf, _, _ = http_req("GET", f"/sub?token={u_tok}")
    check("获取 Sing-Box 订阅 200", code == 200)
    proxy_ob = next(ob for ob in sb_conf["outbounds"] if ob.get("tag") == "Proxy")
    check(f"Sing-Box Proxy 策略组首个节点为 {jp_tag}", jp_tag in proxy_ob["outbounds"])
    urltest_ob = next(ob for ob in sb_conf["outbounds"] if ob.get("tag") == "urltest")
    check(f"Sing-Box urltest 首位为 {jp_tag}", urltest_ob["outbounds"][0] == jp_tag)

    # /mihomo 测试
    code, mh_raw, _, _ = http_req("GET", f"/mihomo?token={u_tok}")
    check("获取 Mihomo 订阅 200", code == 200)
    check(f"Mihomo YAML 包含 {jp_tag}", jp_tag.encode("utf-8") in mh_raw)

    # /node 测试
    code, nodes_list, _, _ = http_req("GET", f"/node?token={u_tok}")
    check("获取动态拼接节点 200", code == 200)
    check(f"/node 顺序首位为 {jp_tag}", nodes_list[0]["outbound"]["tag"] == jp_tag)

    # /api/user/verify 测试
    code, verify_res, _, _ = http_req("GET", f"/api/user/verify?token={u_tok}")
    check("/api/user/verify 有效且 node_ids 顺序对齐", code == 200 and verify_res.get("node_ids") == [node_2["id"], node_1["id"]])

    return user, node_1, node_2


def test_file_distribution_and_zip(user):
    print("\n== 3. 文件分发、ZIP 模板渲染与 Loading 引导页测试 ==")
    u_tok = user["token"]
    u_name = user["name"]
    # 共享 Token
    code, tok_data, _, _ = http_req("GET", "/dl/token", headers=ADMIN)
    check("获取共享 Token 成功", code == 200 and "token" in tok_data)
    shared_tok = tok_data["token"]

    code, rst_data, _, _ = http_req("POST", "/dl/token/reset", headers=ADMIN)
    check("重置共享 Token 成功", code == 200 and rst_data["token"] != shared_tok)
    shared_tok = rst_data["token"]

    # 上传普通文本文件
    code, text_file, _, _ = http_req("POST", "/api/files/text", {
        "name": f"sample_note_{uuid.uuid4().hex[:6]}.txt",
        "content": "Hello Noder Go",
        "download_name": "custom_note.txt"
    }, headers=ADMIN)
    check("创建文本文件成功", code == 200)
    fid = text_file["id"]

    # 共享 token 下载文本文件
    code, content, _, cd = http_req("GET", f"/dl/{fid}?token={shared_tok}")
    check("共享 Token 下载文本文件 200", code == 200)
    check("内容原样输出", content == b"Hello Noder Go")
    check("文件名自定义生效", "custom_note.txt" in cd)

    # 创建带有模板的 ZIP
    zip_buf = io.BytesIO()
    with zipfile.ZipFile(zip_buf, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("config.json", '{"token": "{{token}}", "user": "{{name}}", "secret": "00000000-0000-0000-0000-000000000000"}')
        z.writestr("fixed.txt", "Static content")
    zip_bytes = zip_buf.getvalue()

    # 构造 multipart 请求上传 ZIP
    boundary = f"----WebKitFormBoundary{uuid.uuid4().hex[:12]}"
    body = bytearray()
    body.extend(f"--{boundary}\r\n".encode())
    body.extend(b'Content-Disposition: form-data; name="name"\r\n\r\nConfigPack\r\n')
    body.extend(f"--{boundary}\r\n".encode())
    body.extend(b'Content-Disposition: form-data; name="file_type"\r\n\r\nzip\r\n')
    body.extend(f"--{boundary}\r\n".encode())
    body.extend(b'Content-Disposition: form-data; name="template_name"\r\n\r\nconfig.json\r\n')
    body.extend(f"--{boundary}\r\n".encode())
    body.extend(b'Content-Disposition: form-data; name="file"; filename="pack.zip"\r\nContent-Type: application/zip\r\n\r\n')
    body.extend(zip_bytes)
    body.extend(f"\r\n--{boundary}--\r\n".encode())

    code, zip_dist, _, _ = http_req("POST", "/api/files/upload", raw_body=bytes(body),
                                    content_type=f"multipart/form-data; boundary={boundary}", headers=ADMIN)
    check("上传 ZIP 配置包成功", code == 200)
    zip_id = zip_dist["id"]

    # 测试防重复上传
    code, dup_err, _, _ = http_req("POST", "/api/files/upload", raw_body=bytes(body),
                                   content_type=f"multipart/form-data; boundary={boundary}", headers=ADMIN)
    check("防重复上传生效 (409)", code == 409)

    # 浏览器打开（不带 download=1）测试 Loading 引导页
    code, html_content, ct, _ = http_req("GET", f"/dl/{zip_id}?token={u_tok}")
    check("浏览器打开返回 Loading 页面 (200)", code == 200)
    check("返回 text/html", "text/html" in ct)
    check("包含 Loading 标题", "正在生成" in html_content.decode("utf-8"))
    check("包含用户名", u_name in html_content.decode("utf-8"))
    check("包含异步 download=1 调用脚本", "download" in html_content.decode("utf-8"))

    # 带 download=1 直接下载并校验模板渲染
    code, rendered_zip_data, ct, _ = http_req("GET", f"/dl/{zip_id}?token={u_tok}&download=1")
    check("download=1 返回真实 ZIP 200", code == 200)
    check("Content-Type 为 application/zip", "application/zip" in ct)

    with zipfile.ZipFile(io.BytesIO(rendered_zip_data), "r") as z:
        conf_data = json.loads(z.read("config.json").decode("utf-8"))
        check("ZIP 中 {{token}} 替换为当前用户 token", conf_data["token"] == u_tok)
        check("ZIP 中 {{name}} 替换为当前用户名", conf_data["user"] == u_name)
        check("ZIP 中固定文件保持原样", z.read("fixed.txt") == b"Static content")


def test_templates_and_settings():
    print("\n== 4. 模板管理与系统配置测试 ==")
    code, tpls, _, _ = http_req("GET", "/api/templates", headers=ADMIN)
    check("获取模板列表 200", code == 200 and len(tpls) >= 2)
    t_id = tpls[0]["id"]
    cur_ver = tpls[0]["version"]

    code, updated_tpl, _, _ = http_req("PUT", f"/api/templates/{t_id}", {
        "content": '{"test": "updated"}',
        "remark": "更新测试版本"
    }, headers=ADMIN)
    check("更新模板成功且版本递增", code == 200 and updated_tpl["version"] == cur_ver + 1)

    code, history, _, _ = http_req("GET", f"/api/templates/{t_id}/history", headers=ADMIN)
    check("查询模板历史记录成功", code == 200 and len(history) >= 2)

    # 回滚到 v1
    code, rolled_tpl, _, _ = http_req("POST", f"/api/templates/{t_id}/rollback/1", headers=ADMIN)
    check("回滚到历史版本成功", code == 200 and rolled_tpl["version"] == cur_ver + 2)

    # 系统设置测试
    code, settings, _, _ = http_req("GET", "/api/settings", headers=ADMIN)
    check("获取系统设置 200", code == 200)

    code, res, _, _ = http_req("PUT", "/api/settings", {"site_name": "Noder Go Edition"}, headers=ADMIN)
    check("更新系统设置 200", code == 200)

    code, settings2, _, _ = http_req("GET", "/api/settings", headers=ADMIN)
    check("设置已持久化生效", settings2.get("site_name") == "Noder Go Edition")


def main():
    print("🚀 开始对 Go 版 Noder 进行黑盒全量测试...")
    test_admin_auth()
    user, node_1, node_2 = test_node_and_user_order()
    test_file_distribution_and_zip(user)
    test_templates_and_settings()

    print(f"\n======================================")
    print(f"📊 测试结果: {PASS} 项通过, {FAIL} 项失败")
    print(f"======================================")
    if FAIL > 0:
        sys.exit(1)


if __name__ == "__main__":
    main()
