# -*- coding: utf-8 -*-
"""文件分发：远程拉取、缓存刷新。"""
from datetime import datetime
import urllib.parse
import urllib.request
from typing import Optional

from fastapi import HTTPException, status
from sqlmodel import Session

from app.config import FILES_DIR, MAX_FILE_SIZE, REMOTE_CACHE_TTL
from app.models import DistFile


def validate_remote_url(url: str) -> str:
    """校验远程链接仅允许 http/https，防止 SSRF。"""
    parsed = urllib.parse.urlparse(url)
    if parsed.scheme not in ("http", "https") or not parsed.netloc:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="远程链接仅支持 http/https 协议",
        )
    return url


def fetch_remote_file(url: str, timeout: int = 60) -> bytes:
    """从远程 URL 拉取文件内容，超限/失败抛 HTTPException。"""
    validate_remote_url(url)
    req = urllib.request.Request(url, headers={"User-Agent": "noder-sub-server"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            content = resp.read()
    except Exception as e:
        raise HTTPException(
            status_code=status.HTTP_502_BAD_GATEWAY,
            detail=f"远程拉取失败: {e}",
        )
    if len(content) > MAX_FILE_SIZE:
        raise HTTPException(
            status_code=status.HTTP_413_REQUEST_ENTITY_TOO_LARGE,
            detail=f"远程文件过大，最大支持 {MAX_FILE_SIZE // (1024 * 1024)}MB",
        )
    return content


def is_remote_cache_expired(dist: DistFile) -> bool:
    """远程文件缓存是否过期 (默认 1 天)。"""
    if not dist.cached_at:
        return True
    try:
        cached = datetime.fromisoformat(dist.cached_at)
    except ValueError:
        return True
    return (datetime.now() - cached).total_seconds() >= REMOTE_CACHE_TTL


def refresh_remote_file(dist: DistFile, session: Session) -> DistFile:
    """拉取远程文件并更新缓存。"""
    content = fetch_remote_file(dist.source_url)
    stored_path = FILES_DIR / dist.stored_name
    stored_path.write_bytes(content)
    dist.size = len(content)
    dist.cached_at = datetime.now().isoformat(timespec="seconds")
    session.add(dist)
    session.commit()
    session.refresh(dist)
    return dist
