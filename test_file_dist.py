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
    app_module.DB_PATH = app_module.BASE_DIR / tmp_db
    app_module.engine = app_module.create_engine(
        f"sqlite:///{app_module.DB_PATH}", connect_args={"check_same_thread": False}
    )
    app_module.FILES_DIR = app_module.BASE_DIR / "data" / "files_test"
    app_module.create_db_and_tables()

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

    print("== 3. 权限与状态控制 ==")
    r = client.get(f"/dl/{apk_file['id']}", params={"token": "wrong-token"})
    check("无效 token 拒绝 401", r.status_code == 401)

    r = client.put(f"/api/files/{apk_file['id']}", headers=ADMIN, json={"is_active": False})
    check("停用文件成功", r.status_code == 200)
    r = client.get(f"/dl/{apk_file['id']}", params={"token": token})
    check("停用后下载 404", r.status_code == 404)

    r = client.get("/api/files", headers=ADMIN)
    check("文件列表 3 条", len(r.json()) == 3)
    check("管理 API 无 token 拒绝", client.get("/api/files").status_code == 401)

    r = client.delete(f"/api/files/{apk_file['id']}", headers=ADMIN)
    check("删除文件成功", r.status_code == 200)
    check("磁盘文件已清理", not (app_module.FILES_DIR / apk_file["stored_name"]).exists())

    # 清理测试目录
    if app_module.FILES_DIR.exists():
        import shutil
        shutil.rmtree(app_module.FILES_DIR, ignore_errors=True)
    try:
        os.remove(tmp_db)
    except OSError:
        pass

    print(f"\n结果: {PASS} 通过, {FAIL} 失败")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
