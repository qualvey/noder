# -*- coding: utf-8 -*-
"""用户侧下载 API：ZIP 按用户渲染模板；普通文件/文本走共享 token 原样分发。"""
import secrets

from fastapi import APIRouter, Depends, HTTPException, Query, status
from fastapi.responses import FileResponse, Response
from sqlmodel import Session, select

from app.config import FILES_DIR
from app.database import get_session
from app.models import DistFile, User
from app.routers.settings import get_or_create_shared_token
from app.services.dist import is_remote_cache_expired, refresh_remote_file
from app.services.template_render import render_zip_for_user

router = APIRouter(tags=["download"])


@router.get("/dl/{file_id}", summary="下载分发文件 (ZIP 按用户渲染 / 普通文件与文本走共享 token)")
def download_dist_file(file_id: int, token: str = Query(..., description="鉴权 Token：ZIP 用用户 Token，普通文件/文本用共享 Token"), session: Session = Depends(get_session)):
    dist = session.get(DistFile, file_id)
    if not dist or not dist.is_active:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="File not found or disabled")

    # 鉴权分流：ZIP 个性化渲染走用户 token；普通文件/文本走共享 token
    user = None
    if dist.file_type == "zip":
        user = session.exec(select(User).where(User.token == token)).first()
        if not user or not user.is_active:
            raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid or inactive user token")
    else:
        shared = get_or_create_shared_token(session)
        if not token or not secrets.compare_digest(token, shared):
            raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid shared download token")

    stored_path = FILES_DIR / dist.stored_name
    if not stored_path.exists():
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="File data missing on disk")

    # 远程模式：缓存过期则自动刷新，失败时保留旧缓存继续服务
    if dist.source_url and is_remote_cache_expired(dist):
        try:
            refresh_remote_file(dist, session)
            stored_path = FILES_DIR / dist.stored_name
        except HTTPException as e:
            print(f"[file-dist] remote refresh failed, serving stale cache: {e.detail}")

    if dist.file_type == "text":
        # 文本文件：死字符，原样分发，不做任何渲染
        return Response(
            content=stored_path.read_bytes(),
            media_type="text/plain; charset=utf-8",
            headers={"Content-Disposition": f'attachment; filename="{dist.original_name}"'},
        )

    if dist.file_type == "zip":
        known_tokens = {u.token for u in session.exec(select(User)).all()}
        data = render_zip_for_user(stored_path, dist.template_name, user, known_tokens)
        return Response(
            content=data,
            media_type="application/zip",
            headers={"Content-Disposition": f'attachment; filename="{dist.original_name}"'},
        )

    # APK 静态分发
    return FileResponse(
        stored_path,
        media_type="application/vnd.android.package-archive",
        filename=dist.original_name,
    )
