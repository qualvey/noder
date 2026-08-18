# -*- coding: utf-8 -*-
"""FastAPI 公共依赖。"""
from typing import Optional

from fastapi import Header, HTTPException, status

from app.config import ADMIN_SECRET_TOKEN


def verify_admin_token(x_admin_token: Optional[str] = Header(default=None, alias="X-Admin-Token")):
    """管理员 API 鉴权依赖。"""
    if x_admin_token != ADMIN_SECRET_TOKEN:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or missing X-Admin-Token header",
        )
