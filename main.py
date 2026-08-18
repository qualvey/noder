# -*- coding: utf-8 -*-
"""Sing-Box Subscription Middleman - 应用入口。

模块化结构：
- app/config.py        配置 (路径/常量/环境变量)
- app/database.py      数据库 engine/session/建表/种子数据/lifespan
- app/models.py        SQLModel 模型与 Schema
- app/deps.py          公共依赖 (管理员鉴权)
- app/services/        业务逻辑 (singbox 拼接 / 模板渲染 / 远程文件)
- app/routers/         API 路由 (subscription/download/nodes/users/files)
"""
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles

from app.config import STATIC_DIR
from app.database import lifespan
from app.routers import download, files, nodes, subscription, users

app = FastAPI(
    title="Sing-Box Subscription Middleman",
    description="支持多协议节点与动态 Sing-Box 多节点订阅导出的中间件管理服务",
    version="0.11.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# ------------------------------------------------------------------
# API 路由 (优先于静态资源匹配)
# ------------------------------------------------------------------
app.include_router(subscription.router)
app.include_router(download.router)
app.include_router(nodes.router)
app.include_router(users.router)
app.include_router(files.router)

# ------------------------------------------------------------------
# 静态资源 (前端构建产物输出到 static/)
# 挂载兜底 "/" 支持 SPA 与子路径反代 (相对路径资源)
# ------------------------------------------------------------------
if STATIC_DIR.exists():
    app.mount("/static", StaticFiles(directory=STATIC_DIR), name="static")
    app.mount("/", StaticFiles(directory=STATIC_DIR, html=True), name="spa")


@app.get("/", include_in_schema=False)
def serve_index():
    index_file = STATIC_DIR / "index.html"
    if index_file.exists():
        return FileResponse(index_file)
    return {"message": "Sing-Box Subscription Middleman API Server is running."}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="127.0.0.1", port=8000)
