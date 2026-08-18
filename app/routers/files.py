# -*- coding: utf-8 -*-
"""管理员 CRUD API - 分发文件管理 (/api/files)。"""
from datetime import datetime
import io
from pathlib import Path
from typing import List, Optional
import urllib.parse
import uuid as uuid_lib
import zipfile

from fastapi import APIRouter, Depends, File, Form, HTTPException, UploadFile, status
from sqlmodel import Session, select

from app.config import ALLOWED_FILE_TYPES, FILES_DIR, MAX_FILE_SIZE
from app.database import get_session
from app.deps import verify_admin_token
from app.models import DistFile, DistFileUpdate
from app.services.dist import fetch_remote_file, refresh_remote_file, validate_remote_url

router = APIRouter(
    prefix="/api/files",
    tags=["admin-files"],
    dependencies=[Depends(verify_admin_token)],
)


@router.post("", response_model=DistFile, summary="上传分发文件 (APK / ZIP / 文本，支持本地文件、远程链接或文本内容)")
async def create_dist_file(
    file: Optional[UploadFile] = File(None),
    file_type: str = Form("auto"),
    template_name: Optional[str] = Form(None),
    name: Optional[str] = Form(None),
    download_name: Optional[str] = Form(None),
    remark: Optional[str] = Form(None),
    source_url: Optional[str] = Form(None),
    content_text: Optional[str] = Form(None),
    session: Session = Depends(get_session),
):
    # 数据来源：本地文件 / 远程链接 / 文本内容 (三选一)
    if file is not None and file.filename:
        content = await file.read()
        if not content:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="上传文件为空")
        if len(content) > MAX_FILE_SIZE:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=f"文件过大，最大支持 {MAX_FILE_SIZE // (1024 * 1024)}MB")
        original_name = Path(file.filename).name
    elif source_url and source_url.strip():
        source_url = validate_remote_url(source_url.strip())
        content = fetch_remote_file(source_url)
        original_name = Path(urllib.parse.urlparse(source_url).path).name or "remote.bin"
    elif content_text is not None:
        content = content_text.encode("utf-8")
        if len(content) > MAX_FILE_SIZE:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=f"文本过大，最大支持 {MAX_FILE_SIZE // (1024 * 1024)}MB")
        original_name = (name or "file").strip() or "file.txt"
        if not Path(original_name).suffix:
            original_name += ".txt"
        file_type = "text"
    else:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="请上传文件、填写远程链接或输入文本内容 (三选一)")

    # 类型判定：auto 时按扩展名推断
    if file_type in ("auto", "", None):
        ext = Path(original_name).suffix.lower().lstrip(".")
        file_type = "zip" if ext == "zip" else "apk"
    if file_type not in ALLOWED_FILE_TYPES:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=f"不支持的文件类型 '{file_type}'，仅支持 apk / zip / text")

    # ZIP 校验 + 模板文件确定
    if file_type == "zip":
        try:
            with zipfile.ZipFile(io.BytesIO(content)) as zf:
                names = zf.namelist()
                bad = zf.testzip()
        except zipfile.BadZipFile:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="无效的 ZIP 文件")
        if bad:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=f"ZIP 文件损坏: {bad}")
        if not template_name:
            yaml_names = [n for n in names if n.lower().endswith((".yaml", ".yml"))]
            if not yaml_names:
                raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="ZIP 内未找到 .yaml/.yml 模板，请指定模板文件名")
            template_name = yaml_names[0]
        elif not any(Path(n).name == Path(template_name).name for n in names):
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=f"ZIP 中不存在模板文件: {template_name}")

    # 落盘存储
    FILES_DIR.mkdir(parents=True, exist_ok=True)
    stored_name = f"{uuid_lib.uuid4().hex}_{original_name}"
    (FILES_DIR / stored_name).write_bytes(content)

    dist = DistFile(
        name=name or original_name,
        file_type=file_type,
        template_name=template_name,
        original_name=original_name,
        download_name=(download_name or "").strip() or None,
        stored_name=stored_name,
        size=len(content),
        is_active=True,
        remark=remark or None,
        source_url=source_url,
        cached_at=datetime.now().isoformat(timespec="seconds") if source_url else None,
    )
    session.add(dist)
    session.commit()
    session.refresh(dist)
    return dist


@router.post("/{file_id}/refresh", response_model=DistFile, summary="强制刷新远程文件缓存")
def refresh_dist_file(file_id: int, session: Session = Depends(get_session)):
    dist = session.get(DistFile, file_id)
    if not dist:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="File not found")
    if not dist.source_url:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="该文件不是远程链接模式，无需刷新")
    return refresh_remote_file(dist, session)


@router.get("", response_model=List[DistFile], summary="获取分发文件列表")
def list_dist_files(session: Session = Depends(get_session)):
    return session.exec(select(DistFile).order_by(DistFile.id.desc())).all()


@router.put("/{file_id}", response_model=DistFile, summary="更新分发文件元数据")
def update_dist_file(file_id: int, file_data: DistFileUpdate, session: Session = Depends(get_session)):
    dist = session.get(DistFile, file_id)
    if not dist:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="File not found")
    update_dict = file_data.model_dump(exclude_unset=True)
    for key, value in update_dict.items():
        setattr(dist, key, value)
    session.add(dist)
    session.commit()
    session.refresh(dist)
    return dist


@router.delete("/{file_id}", summary="删除分发文件")
def delete_dist_file(file_id: int, session: Session = Depends(get_session)):
    dist = session.get(DistFile, file_id)
    if not dist:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="File not found")
    stored_path = FILES_DIR / dist.stored_name
    if stored_path.exists():
        stored_path.unlink()
    session.delete(dist)
    session.commit()
    return {"message": f"File {file_id} deleted successfully"}
