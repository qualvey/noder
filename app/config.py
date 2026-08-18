# -*- coding: utf-8 -*-
"""应用配置：路径、常量、环境变量。"""
import os
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent.parent
DB_PATH = BASE_DIR / "data.db"
TEMPLATE_PATH = BASE_DIR / "template.json"
STATIC_DIR = BASE_DIR / "static"

# 管理员秘钥配置 (可以通过环境变量 ADMIN_SECRET_TOKEN 覆盖)
ADMIN_SECRET_TOKEN = os.getenv("ADMIN_SECRET_TOKEN", "admin-secret")

ALLOWED_PROTOCOLS = {"tuic", "vless", "anytls"}

# 允许用户通过 config_override 覆盖的顶层配置键
# 目前限制 route / dns，后期扩展只需往此集合添加键名
ALLOWED_OVERRIDE_KEYS = {"route", "dns"}

# 分发文件存储目录与限制
FILES_DIR = BASE_DIR / "data" / "files"
ALLOWED_FILE_TYPES = {"apk", "zip", "text"}
MAX_FILE_SIZE = 100 * 1024 * 1024  # 100MB
REMOTE_CACHE_TTL = 86400  # 远程拉取缓存有效期: 1 天 (秒)

# 模板渲染支持的占位符说明 (见 doc/file_distribution.md)
TEMPLATE_VAR_DOC = (
    "{{uuid}} {{password}} {{token}} {{name}} {{user_name}} "
    "{{node_list_yaml}} {{node_list_json}} {{outbounds_yaml}} {{outbounds_json}}"
)
