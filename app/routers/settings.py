# -*- coding: utf-8 -*-
"""系统设置 API：共享下载 token 的查看与重置。

共享 token 用于普通文件 (apk) / 文本文件 (text) 的下载鉴权，
与用户 token 相互独立；管理员可随时重置，重置后旧 token 立即失效。
ZIP 个性化下载仍走用户 token（见 /dl 端点鉴权分流）。
"""
import secrets

from fastapi import APIRouter, Depends
from sqlmodel import Session

from app.database import get_session
from app.deps import verify_admin_token
from app.models import AppSetting

router = APIRouter(
    prefix="/api/settings",
    tags=["admin-settings"],
    dependencies=[Depends(verify_admin_token)],
)

SHARED_TOKEN_KEY = "shared_download_token"


def get_or_create_shared_token(session: Session) -> str:
    """读取共享下载 token；不存在则生成（幂等）。"""
    setting = session.get(AppSetting, SHARED_TOKEN_KEY)
    if not setting:
        setting = AppSetting(key=SHARED_TOKEN_KEY, value=secrets.token_hex(16))
        session.add(setting)
        session.commit()
        return setting.value
    return setting.value


@router.get("/shared-token", summary="获取共享下载 Token (普通文件/文本文件)")
def read_shared_token(session: Session = Depends(get_session)):
    return {"token": get_or_create_shared_token(session)}


@router.post("/shared-token/reset", summary="重置共享下载 Token (旧值立即失效)")
def reset_shared_token(session: Session = Depends(get_session)):
    setting = session.get(AppSetting, SHARED_TOKEN_KEY)
    new_token = secrets.token_hex(16)
    if setting:
        setting.value = new_token
        session.add(setting)
    else:
        session.add(AppSetting(key=SHARED_TOKEN_KEY, value=new_token))
    session.commit()
    return {"token": new_token}
