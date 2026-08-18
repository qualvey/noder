# -*- coding: utf-8 -*-
"""
文件分发功能集成测试：
1. 上传 APK (静态分发) -> 下载校验原样返回
2. 上传 ZIP (含 yaml 模板 + 公共文件) -> 下载校验模板按用户渲染、公共文件原样
3. 无效 token / 停用文件 的拒绝逻辑
运行: uv run python test_file_dist.py
"""
import io
import json
import os
import sys
import tempfile
import zipfile

# Windows 控制台 UTF-8 输出
if sys.stdout.encoding and sys.stdout.encoding.lower() not in ("utf-8", "utf8"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, BASE_DIR)

import main as app_module  # noqa: E402
import app.config as cfg  # noqa: E402
import app.database as database  # noqa: E402
import app.services.dist as dist_svc  # noqa: E402
import app.routers.files as files_router  # noqa: E402
import app.routers.download as download_router  # noqa: E402
from fastapi.testclient import TestClient  # noqa: E402

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


def make_test_zip(template_text: str) -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("config/template.yaml", template_text)
        zf.writestr("assets/logo.png", b"\x89PNG-fake-binary-content")
        zf.writestr("README.txt", "这是公共说明文件，所有用户一致。")
    return buf.getvalue()


def main():
    # 使用临时数据库，避免污染 data.db
    tmp_db = tempfile.mktemp(suffix=".db")
    tmp_db_path = cfg.BASE_DIR / tmp_db
    cfg.DB_PATH = tmp_db_path
    database.DB_PATH = tmp_db_path
    database.engine = database.create_engine(
        f"sqlite:///{tmp_db_path}", connect_args={"check_same_thread": False}
    )
    test_files_dir = cfg.BASE_DIR / "data" / "files_test"
    cfg.FILES_DIR = test_files_dir
    dist_svc.FILES_DIR = test_files_dir
    files_router.FILES_DIR = test_files_dir
    download_router.FILES_DIR = test_files_dir
    database.create_db_and_tables()

    client = TestClient(app_module.app)

    # 预置一个测试用户 (走 API 创建，便于拿到 token)
    r = client.post("/api/users", headers=ADMIN, json={
        "name": "测试用户甲", "uuid": "11111111-2222-3333-4444-555555555555",
        "password": "test-pass-1", "node_ids": []
    })
    assert r.status_code == 200, r.text
    user = r.json()
    token = user["token"]

    # 第二个用户：其 token 将被硬编码进模板，验证自动替换
    r = client.post("/api/users", headers=ADMIN, json={
        "name": "测试用户乙", "uuid": "99999999-8888-7777-6666-555555555555",
        "password": "test-pass-2", "node_ids": []
    })
    assert r.status_code == 200, r.text
    user_b = r.json()
    token_b = user_b["token"]

    print("== 1. APK 静态分发 ==")
    apk_bytes = b"META-INF/MANIFEST.MF fake apk content" * 100
    r = client.post("/api/files", headers=ADMIN, files={
        "file": ("myapp.apk", io.BytesIO(apk_bytes), "application/vnd.android.package-archive")
    }, data={"file_type": "auto", "name": "客户端安装包", "remark": "全员公用"})
    check("上传 APK 成功", r.status_code == 200, r.text)
    apk_file = r.json()
    check("类型识别为 apk", apk_file["file_type"] == "apk")
    check("大小记录正确", apk_file["size"] == len(apk_bytes))

    r = client.get(f"/dl/{apk_file['id']}", params={"token": token})
    check("APK 下载 200", r.status_code == 200)
    check("APK 内容原样", r.content == apk_bytes)
    check("APK Content-Disposition 附件", "attachment" in r.headers.get("content-disposition", ""))

    print("== 2. ZIP 模板个性化渲染 ==")
    template = """# 用户专属配置
name: {{name}}
uuid: {{uuid}}
password: {{password}}
token: {{token}}
node_list_yaml: |
{{node_list_yaml}}
outbounds:
{{outbounds_yaml}}
"""
    zip_bytes = make_test_zip(template)
    r = client.post("/api/files", headers=ADMIN, files={
        "file": ("client-config.zip", io.BytesIO(zip_bytes), "application/zip")
    }, data={"file_type": "zip"})
    check("上传 ZIP 成功", r.status_code == 200, r.text)
    zip_file = r.json()
    check("自动识别模板文件", zip_file["template_name"] == "config/template.yaml", zip_file["template_name"])

    r = client.get(f"/dl/{zip_file['id']}", params={"token": token})
    check("ZIP 下载 200", r.status_code == 200, r.text[:200])
    with zipfile.ZipFile(io.BytesIO(r.content)) as zf:
        names = zf.namelist()
        rendered = zf.read("config/template.yaml").decode("utf-8")
        logo = zf.read("assets/logo.png")
        readme = zf.read("README.txt").decode("utf-8")

    check("ZIP 内文件齐全", set(names) == {"config/template.yaml", "assets/logo.png", "README.txt"})
    check("模板渲染 name", "name: 测试用户甲" in rendered, rendered)
    check("模板渲染 uuid", "uuid: 11111111-2222-3333-4444-555555555555" in rendered)
    check("模板渲染 password", "password: test-pass-1" in rendered)
    check("模板渲染 token", f"token: {token}" in rendered)
    check("公共文件原样", logo == b"\x89PNG-fake-binary-content")
    check("文本公共文件原样", readme == "这是公共说明文件，所有用户一致。")

    print("== 2.1 硬编码 token 自动替换 ==")
    hardcoded_template = f"""# launcher config
upstream_url: "https://hk.ryugo.org/sub?token={token_b}"
token: "{token_b}"
config_file: "config.json"
sing_box_bin: "./sing-box.exe"
"""
    zip2 = make_test_zip(hardcoded_template)
    r = client.post("/api/files", headers=ADMIN, files={
        "file": ("launcher.zip", io.BytesIO(zip2), "application/zip")
    }, data={"file_type": "zip"})
    check("上传含硬编码 token 的 ZIP 成功", r.status_code == 200, r.text)
    zip2_file = r.json()

    r = client.get(f"/dl/{zip2_file['id']}", params={"token": token})
    check("硬编码模板下载 200", r.status_code == 200, r.text[:200])
    with zipfile.ZipFile(io.BytesIO(r.content)) as zf:
        rendered2 = zf.read("config/template.yaml").decode("utf-8")
    check("token 键值替换为用户甲 token", f'token: "{token}"' in rendered2, rendered2)
    check("URL 内 token 同步替换", f"sub?token={token}" in rendered2, rendered2)
    check("用户乙 token 不再出现", token_b not in rendered2)
    check("公共字段未受影响", 'sing_box_bin: "./sing-box.exe"' in rendered2)

    print("== 3. 远程链接拉取与缓存 ==")
    # 用本地起的 HTTP 服务模拟远程源
    from http.server import HTTPServer, SimpleHTTPRequestHandler
    import threading
    import tempfile as _tf

    remote_dir = _tf.mkdtemp()
    remote_apk = b"REMOTE-APK-V2-BYTES" * 50
    with open(os.path.join(remote_dir, "app-remote.apk"), "wb") as f:
        f.write(remote_apk)

    class _H(SimpleHTTPRequestHandler):
        def __init__(self, *a, **kw):
            super().__init__(*a, directory=remote_dir, **kw)

        def log_message(self, *a):
            pass

    srv = HTTPServer(("127.0.0.1", 0), _H)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    remote_url = f"http://127.0.0.1:{srv.server_port}/app-remote.apk"

    r = client.post("/api/files", headers=ADMIN, data={
        "file_type": "auto", "name": "远程APK", "source_url": remote_url
    })
    check("远程链接创建成功", r.status_code == 200, r.text)
    remote_file = r.json()
    check("类型按扩展名识别 apk", remote_file["file_type"] == "apk")
    check("source_url 已记录", remote_file["source_url"] == remote_url)
    check("cached_at 已写入", remote_file["cached_at"] is not None)

    r = client.get(f"/dl/{remote_file['id']}", params={"token": token})
    check("远程文件下载 200", r.status_code == 200)
    check("远程文件内容正确", r.content == remote_apk)

    # 更新远程源内容，强制刷新后应拿到新内容
    with open(os.path.join(remote_dir, "app-remote.apk"), "wb") as f:
        f.write(b"REMOTE-APK-V3-BYTES" * 50)
    r = client.post(f"/api/files/{remote_file['id']}/refresh", headers=ADMIN)
    check("强制刷新 200", r.status_code == 200, r.text)
    r = client.get(f"/dl/{remote_file['id']}", params={"token": token})
    check("刷新后内容为最新", r.content == b"REMOTE-APK-V3-BYTES" * 50)

    # 无效 scheme 拒绝
    r = client.post("/api/files", headers=ADMIN, data={"source_url": "file:///etc/passwd"})
    check("非 http(s) 链接拒绝 400", r.status_code == 400)

    srv.shutdown()

    print("== 4. 文本内容组件 (字符串 -> 下载链接) ==")
    r = client.post("/api/files", headers=ADMIN, data={
        "name": "launcher.conf", "content_text": "upstream: https://hk.ryugo.org/sub?token={{token}}\ntoken: \"{{token}}\"\nuser: {{name}}\nsing_box_bin: ./sing-box.exe\n"
    })
    check("文本内容创建成功", r.status_code == 200, r.text)
    text_file = r.json()
    check("类型识别为 text", text_file["file_type"] == "text")
    check("文件名保留指定后缀", text_file["original_name"] == "launcher.conf", text_file["original_name"])
    check("文本模式无 source_url", text_file["source_url"] is None)

    r = client.get(f"/dl/{text_file['id']}", params={"token": token})
    check("文本下载 200", r.status_code == 200)
    body = r.content.decode("utf-8")
    check("占位符原样保留不渲染", 'token: "{{token}}"', body)
    check("URL 内占位符原样", "sub?token={{token}}" in body)
    check("user 占位符原样", "user: {{name}}" in body)
    check("公共内容原样", "sing_box_bin: ./sing-box.exe" in body)
    check("响应 Content-Disposition 附件", "attachment" in r.headers.get("content-disposition", ""))

    # 硬编码 token 智能替换同样作用于文本模式
    r = client.post("/api/files", headers=ADMIN, data={
        "name": "conf2.txt", "content_text": f"token: \"{token_b}\"\n"
    })
    assert r.status_code == 200, r.text
    text_file2 = r.json()
    r = client.get(f"/dl/{text_file2['id']}", params={"token": token})
    body2 = r.content.decode("utf-8")
    check("文本模式硬编码 token 原样保留", f"token: \"{token_b}\"" in body2, body2)
    check("未替换为下载者 token", f"token: \"{token}\"" not in body2)

    print("== 5. 权限与状态控制 ==")


    r = client.get(f"/dl/{apk_file['id']}", params={"token": "wrong-token"})
    check("无效 token 拒绝 401", r.status_code == 401)

    r = client.put(f"/api/files/{apk_file['id']}", headers=ADMIN, json={"is_active": False})
    check("停用文件成功", r.status_code == 200)
    r = client.get(f"/dl/{apk_file['id']}", params={"token": token})
    check("停用后下载 404", r.status_code == 404)

    r = client.get("/api/files", headers=ADMIN)
    check("文件列表 6 条", len(r.json()) == 6)
    check("管理 API 无 token 拒绝", client.get("/api/files").status_code == 401)

    r = client.delete(f"/api/files/{apk_file['id']}", headers=ADMIN)
    check("删除文件成功", r.status_code == 200)
    check("磁盘文件已清理", not (test_files_dir / apk_file["stored_name"]).exists())

    # 清理测试目录
    if test_files_dir.exists():
        import shutil
        shutil.rmtree(test_files_dir, ignore_errors=True)
    try:
        os.remove(tmp_db)
    except OSError:
        pass

    print(f"\n结果: {PASS} 通过, {FAIL} 失败")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
