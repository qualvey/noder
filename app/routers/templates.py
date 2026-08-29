# -*- coding: utf-8 -*-
"""管理员 CRUD API - 模板管理与历史回滚 (/api/templates)。"""
from datetime import datetime
import json
from typing import List, Optional
from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from sqlmodel import Session, select
import yaml

from app.database import get_session
from app.deps import verify_admin_token
from app.models import Template, TemplateHistory, TemplateRead

router = APIRouter(
    prefix="/api/templates",
    tags=["admin-templates"],
    dependencies=[Depends(verify_admin_token)],
)

# ================== 1. 结构模型定义 ==================

class SaveTemplateRequest(BaseModel):
    target: str                  # 如 "sing-box" 或 "mihomo"
    content: str                 # 模板内容字符串
    remark: Optional[str] = "Web 前端更新"


# ================== 2. 内部校验逻辑 ==================

def validate_template_content(target: str, content: str, content_format: str):
    """根据目标内核与格式校验语法与必填字段"""
    try:
        if content_format == "json":
            data = json.loads(content)
            if not isinstance(data, dict):
                raise ValueError("JSON 根节点必须为对象 (dict)")
            if target == "sing-box" and "outbounds" not in data:
                raise ValueError("sing-box 配置缺少必要的 'outbounds' 节点")
        elif content_format == "yaml":
            data = yaml.safe_load(content)
            if not isinstance(data, dict):
                raise ValueError("YAML 根节点必须为映射对象 (dict)")
            if target == "mihomo" and "proxy-groups" not in data:
                raise ValueError("mihomo 配置缺少必要的 'proxy-groups' 节点")
    except Exception as e:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"模板语法或字段校验失败: {str(e)}"
        )

# ================== 3. API 路由实现 ==================

@router.get("", response_model=List[TemplateRead], summary="获取所有内核模板列表")
def list_templates(session: Session = Depends(get_session)):
    """供前端下拉切换 sing-box / mihomo"""
    return session.exec(select(Template)).all()


@router.get("/{target}", response_model=TemplateRead, summary="获取指定内核的当前模板")
def get_template(target: str, session: Session = Depends(get_session)):
    tpl = session.exec(select(Template).where(Template.target == target)).first()
    if not tpl:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND, 
            detail=f"未找到内核 {target} 的模板"
        )
    return tpl


@router.put("", response_model=TemplateRead, summary="提交并保存模板内容（自动产生历史快照）")
def save_template(
    payload: SaveTemplateRequest,
    session: Session = Depends(get_session)
):
    # 1. 查出现有模板
    tpl = session.exec(select(Template).where(Template.target == payload.target)).first()
    if not tpl:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND, 
            detail=f"未找到目标内核 {payload.target} 的模板定义"
        )

    # 2. 格式与业务规则校验
    validate_template_content(payload.target, payload.content, tpl.content_format)

    # 3. 递增主表版本并更新
    tpl.version += 1
    tpl.content = payload.content
    tpl.updated_at = datetime.utcnow()
    session.add(tpl)
    session.commit()
    session.refresh(tpl)

    # 4. 插入历史版本快照
    assert tpl.id is not None
    history = TemplateHistory(
        template_id=tpl.id,
        target=tpl.target,
        version=tpl.version,
        content=payload.content,
        remark=payload.remark or f"更新至 v{tpl.version}",
    )
    session.add(history)
    session.commit()

    return tpl


@router.get("/{target}/histories", response_model=List[TemplateHistory], summary="获取指定内核的历史版本列表")
def get_template_histories(
    target: str,
    session: Session = Depends(get_session)
):
    stmt = select(TemplateHistory).where(TemplateHistory.target == target).order_by(TemplateHistory.id.desc())
    return session.exec(stmt).all()


@router.post("/{target}/rollback/{history_id}", response_model=TemplateRead, summary="回滚到指定的历史版本")
def rollback_template(
    target: str,
    history_id: int,
    session: Session = Depends(get_session)
):
    tpl = session.exec(select(Template).where(Template.target == target)).first()
    history = session.get(TemplateHistory, history_id)

    if not tpl or not history or history.template_id != tpl.id:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND, 
            detail="指定的历史版本或目标模板不存在"
        )

    # 产生新版本覆盖主表（保留连续递增轨迹）
    tpl.version += 1
    tpl.content = history.content
    tpl.updated_at = datetime.utcnow()
    session.add(tpl)
    session.commit()
    session.refresh(tpl)

    # 记录一条回滚动作作为新历史
    assert tpl.id is not None
    new_history = TemplateHistory(
        template_id=tpl.id,
        target=tpl.target,
        version=tpl.version,
        content=history.content,
        remark=f"从版本 v{history.version} 执行回滚 (快照 ID: {history.id})",
    )
    session.add(new_history)
    session.commit()

    return tpl