from contextlib import asynccontextmanager
import json
import os
from pathlib import Path
import secrets
from typing import List, Optional
import uuid as uuid_lib

from fastapi import FastAPI, Depends, HTTPException, Header, Query, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from sqlmodel import Field, Relationship, Session, SQLModel, create_engine, select
import uvicorn

# ------------------------------------------------------------------
# 1. 数据库与模板文件路径配置
# ------------------------------------------------------------------
BASE_DIR = Path(__file__).parent
DB_PATH = BASE_DIR / "data.db"
TEMPLATE_PATH = BASE_DIR / "template.json"
sqlite_url = f"sqlite:///{DB_PATH}"

engine = create_engine(sqlite_url, connect_args={"check_same_thread": False})

# 管理员秘钥配置 (可以通过环境变量 ADMIN_SECRET_TOKEN 覆盖)
ADMIN_SECRET_TOKEN = os.getenv("ADMIN_SECRET_TOKEN", "admin-secret")

ALLOWED_PROTOCOLS = {"tuic", "vless", "anytls"}

# ------------------------------------------------------------------
# 2. SQLModel 数据库模型与 Pydantic Schema
# 方案 3：节点单独分表 (不存凭证)，用户表存储凭证，支持用户关联多个节点
# ------------------------------------------------------------------
class UserNodeLink(SQLModel, table=True):
    __table_args__ = {"extend_existing": True}
    user_id: Optional[int] = Field(default=None, foreign_key="user.id", primary_key=True)
    node_id: Optional[int] = Field(default=None, foreign_key="node.id", primary_key=True)

class NodeBase(SQLModel):
    node_name: str                              # 节点名称
    protocol: str = Field(default="vless")       # 仅限: tuic, vless, anytls
    server_address: str                         # 上游 IP / 域名
    server_port: int                            # 端口
    method: Optional[str] = None                 # 加密方式
    security: Optional[str] = "tls"              # auto, tls, reality, none
    sni: Optional[str] = None                    # TLS SNI / ServerName
    transport_type: Optional[str] = "direct"     # direct, ws, grpc, http
    path: Optional[str] = None                   # 传输路径
    is_active: bool = True                       # 是否启用
    
    # VLESS REALITY 专属与扩展属性
    public_key: Optional[str] = None             # REALITY 公钥
    short_id: Optional[str] = None               # REALITY Short ID
    fingerprint: Optional[str] = "chrome"        # uTLS 指纹 (默认 chrome)
    flow: Optional[str] = "xtls-rprx-vision"     # 流控 (默认 xtls-rprx-vision)
    remark: Optional[str] = Field(default=None)  # 管理员备注 (仅管理员可见)

class Node(NodeBase, table=True):
    __table_args__ = {"extend_existing": True}
    id: Optional[int] = Field(default=None, primary_key=True)
    users: List["User"] = Relationship(back_populates="nodes", link_model=UserNodeLink)

class NodeCreate(NodeBase):
    pass

class NodeRead(NodeBase):
    id: int

class NodeUpdate(SQLModel):
    node_name: Optional[str] = None
    protocol: Optional[str] = None
    server_address: Optional[str] = None
    server_port: Optional[int] = None
    method: Optional[str] = None
    security: Optional[str] = None
    sni: Optional[str] = None
    transport_type: Optional[str] = None
    path: Optional[str] = None
    is_active: Optional[bool] = None
    public_key: Optional[str] = None
    short_id: Optional[str] = None
    fingerprint: Optional[str] = None
    flow: Optional[str] = None
    remark: Optional[str] = None

class UserBase(SQLModel):
    name: str
    is_active: bool = True
    uuid: Optional[str] = Field(default=None)      # 用户专属 UUID (用于 VLESS / TUIC / AnyTLS)
    password: Optional[str] = Field(default=None)  # 用户专属密码 (用于 TUIC / AnyTLS 等)
    remark: Optional[str] = Field(default=None)    # 管理员备注 (仅管理员可见)

class User(UserBase, table=True):
    __table_args__ = {"extend_existing": True}
    id: Optional[int] = Field(default=None, primary_key=True)
    token: str = Field(index=True, unique=True)
    nodes: List[Node] = Relationship(back_populates="users", link_model=UserNodeLink)

class UserCreate(UserBase):
    token: Optional[str] = None     # 未传则自动生成 UUID
    uuid: Optional[str] = None      # 未传则自动生成 UUID
    password: Optional[str] = None  # 未传则自动生成随机密码
    node_ids: List[int] = Field(default_factory=list)

class UserRead(UserBase):
    id: int
    token: str
    uuid: Optional[str] = None
    password: Optional[str] = None
    node_ids: List[int] = Field(default_factory=list)

class UserUpdate(SQLModel):
    name: Optional[str] = None
    token: Optional[str] = None
    uuid: Optional[str] = None
    password: Optional[str] = None
    is_active: Optional[bool] = None
    remark: Optional[str] = None
    node_ids: Optional[List[int]] = None

# 辅助校验函数
def validate_node_protocol_and_security(protocol: str, security: Optional[str], public_key: Optional[str] = None, short_id: Optional[str] = None):
    proto = protocol.lower() if protocol else "vless"
    sec = (security or "").lower()
    
    if proto not in ALLOWED_PROTOCOLS:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Protocol '{protocol}' is invalid. Allowed protocols: {', '.join(ALLOWED_PROTOCOLS)}"
        )
    if proto == "tuic" and sec != "tls":
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="TUIC protocol strictly requires security mode to be 'tls'."
        )
    if proto == "vless":
        if sec != "reality":
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="VLESS protocol strictly requires security mode to be 'reality'."
            )
        if not public_key or not short_id:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="VLESS REALITY protocol strictly requires both 'public_key' and 'short_id'."
            )

# ------------------------------------------------------------------
# 3. 数据库初始化与生命周期
# ------------------------------------------------------------------
def create_db_and_tables():
    from sqlalchemy import text
    SQLModel.metadata.create_all(engine)
    with engine.connect() as conn:
        for col in ["public_key", "short_id", "fingerprint", "flow", "remark"]:
            try:
                conn.execute(text(f"ALTER TABLE node ADD COLUMN {col} VARCHAR"))
                conn.commit()
            except Exception:
                pass
        try:
            conn.execute(text("ALTER TABLE user ADD COLUMN remark VARCHAR"))
            conn.commit()
        except Exception:
            pass

def get_session():
    with Session(engine) as session:
        yield session

def verify_admin_token(x_admin_token: Optional[str] = Header(default=None, alias="X-Admin-Token")):
    """管理员 API 鉴权依赖"""
    if x_admin_token != ADMIN_SECRET_TOKEN:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or missing X-Admin-Token header"
        )

@asynccontextmanager
async def lifespan(app: FastAPI):
    create_db_and_tables()
    with Session(engine) as session:
        if not session.exec(select(Node)).first():
            # 预置 1 个 TUIC 节点
            tuic_node = Node(
                node_name="TUIC 高速专线 01",
                protocol="tuic",
                server_address="tuic.example.com",
                server_port=8443,
                security="tls",
                sni="tuic.example.com",
                is_active=True
            )
            session.add(tuic_node)

            # 预置 1 个 VLESS REALITY 节点 (按 doc/vless.md 定义)
            vless_node = Node(
                node_name="VLESS REALITY 专线 02",
                protocol="vless",
                server_address="example.com",
                server_port=8443,
                security="reality",
                sni="aws.amazon.com",
                public_key="99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo",
                short_id="1a91",
                fingerprint="chrome",
                flow="xtls-rprx-vision",
                is_active=True
            )
            session.add(vless_node)

            # 预置 1 个 AnyTLS 节点
            anytls_node = Node(
                node_name="AnyTLS 极速专线 03",
                protocol="anytls",
                server_address="anytls.example.com",
                server_port=8443,
                security="tls",
                sni="anytls.example.com",
                is_active=True
            )
            session.add(anytls_node)

            session.commit()
            session.refresh(tuic_node)
            session.refresh(vless_node)
            session.refresh(anytls_node)

            test_user = User(
                name="张三",
                token="my-secret-token",
                uuid=str(uuid_lib.uuid4()),
                password=secrets.token_hex(8),
                is_active=True,
                nodes=[tuic_node, vless_node, anytls_node]
            )
            session.add(test_user)
            session.commit()
    yield

# ------------------------------------------------------------------
# 4. FastAPI 应用初始化与静态文件挂载
# ------------------------------------------------------------------
app = FastAPI(
    title="Sing-Box Subscription Middleman",
    description="支持多协议节点与动态 Sing-Box 多节点订阅导出的中间件管理服务",
    version="0.5.0",
    lifespan=lifespan
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 挂载静态文件目录
static_dir = BASE_DIR / "static"
if static_dir.exists():
    app.mount("/static", StaticFiles(directory=static_dir), name="static")

@app.get("/", include_in_schema=False)
def serve_index():
    index_file = BASE_DIR / "static" / "index.html"
    if index_file.exists():
        return FileResponse(index_file)
    return {"message": "Sing-Box Subscription Middleman API Server is running."}

# ------------------------------------------------------------------
# 5. Sing-Box 配置生成核心逻辑 (对齐 doc/vless.md 规格)
# ------------------------------------------------------------------
def build_singbox_outbound(node: Node, user: User) -> dict:
    protocol = (node.protocol or "vless").lower()
    tag = node.node_name

    outbound = {
        "type": protocol,
        "tag": tag,
        "server": node.server_address,
        "server_port": node.server_port,
    }

    user_uuid = user.uuid or user.token
    user_password = user.password or user.token

    if protocol == "tuic":
        outbound["uuid"] = user_uuid
        outbound["password"] = user_password
        outbound["congestion_control"] = "bbr"
        outbound["zero_rtt_handshake"] = False
        tls_config = {"enabled": True}
        if node.sni:
            tls_config["server_name"] = node.sni
        tls_config["alpn"] = ["h3"]
        outbound["tls"] = tls_config
    elif protocol == "vless":
        outbound["uuid"] = user_uuid
        outbound["flow"] = node.flow or "xtls-rprx-vision"
        
        if node.security in ["tls", "reality"]:
            tls_config = {"enabled": True}
            if node.sni:
                tls_config["server_name"] = node.sni

            # utls 开启
            tls_config["utls"] = {
                "enabled": True,
                "fingerprint": node.fingerprint or "chrome"
            }

            if node.security == "reality":
                tls_config["reality"] = {
                    "enabled": True,
                    "public_key": node.public_key or "",
                    "short_id": node.short_id or ""
                }
            outbound["tls"] = tls_config
    elif protocol == "anytls":
        outbound["uuid"] = user_uuid
        outbound["password"] = user_password
        if node.security in ["tls", "reality"]:
            tls_config = {"enabled": True}
            if node.sni:
                tls_config["server_name"] = node.sni
            if node.security == "reality":
                tls_config["reality"] = {
                    "enabled": True,
                    "public_key": node.public_key or "",
                    "short_id": node.short_id or ""
                }
            outbound["tls"] = tls_config

    if node.transport_type and node.transport_type != "direct":
        transport = {"type": node.transport_type}
        if node.path:
            transport["path"] = node.path
        outbound["transport"] = transport

    return outbound

def load_singbox_template() -> dict:
    """读取外置 template.json 模板文件"""
    if TEMPLATE_PATH.exists():
        try:
            with open(TEMPLATE_PATH, "r", encoding="utf-8") as f:
                return json.load(f)
        except Exception as e:
            print(f"Error loading template.json ({e}), falling back to basic template.")
    
    # 基本降级模板
    return {
        "log": {"level": "info", "timestamp": True},
        "outbounds": [
            {"type": "direct", "tag": "direct", "routing_mark": 1024},
            {"tag": "Proxy", "type": "selector", "outbounds": ["urltest"]},
            {"tag": "urltest", "type": "urltest", "outbounds": []}
        ]
    }

def generate_singbox_config(nodes: List[Node], user: User) -> dict:
    active_nodes = [n for n in nodes if n.is_active]
    node_outbounds = [build_singbox_outbound(n, user) for n in active_nodes]
    node_tags = [n.node_name for n in active_nodes]

    # 读取外置 template.json
    config = load_singbox_template()
    outbounds = config.get("outbounds", [])

    # 1. 遍历模板 outbounds，将节点 tag 注入到 Proxy (selector) 与 urltest
    for ob in outbounds:
        if isinstance(ob, dict):
            tag = ob.get("tag", "")
            ob_type = ob.get("type", "")

            # selector 分组 (如 tag == "Proxy" 或 type == "selector")
            if tag == "Proxy" or ob_type == "selector":
                existing = ob.get("outbounds", [])
                for n_tag in node_tags:
                    if n_tag not in existing:
                        existing.append(n_tag)
                ob["outbounds"] = existing

            # urltest 分组 (如 tag == "urltest" 或 type == "urltest")
            elif tag == "urltest" or ob_type == "urltest":
                ob["outbounds"] = node_tags

    # 2. 将该用户的节点对象直接补充到 outbounds 数组末尾
    outbounds.extend(node_outbounds)

    config["outbounds"] = outbounds
    return config

# ------------------------------------------------------------------
# 6. 用户侧核心 API (Token 验证与多节点导出)
# ------------------------------------------------------------------
@app.get("/sub", summary="获取 Sing-Box 订阅配置")
def get_singbox_config(token: str = Query(..., description="用户鉴权 Token"), session: Session = Depends(get_session)):
    user = session.exec(select(User).where(User.token == token)).first()
    if not user or not user.is_active:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid or inactive user token")

    if not user.nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="No nodes associated with this user")

    active_nodes = [n for n in user.nodes if n.is_active]
    if not active_nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="All bound nodes are inactive")

    return generate_singbox_config(active_nodes, user)

@app.get("/node", summary="获取用户绑定的动态拼接节点")
def get_user_nodes(token: str = Query(..., description="用户鉴权 Token"), session: Session = Depends(get_session)):
    user = session.exec(select(User).where(User.token == token)).first()
    if not user or not user.is_active:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid or inactive user token")

    if not user.nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="No nodes associated with this user")

    active_nodes = [n for n in user.nodes if n.is_active]
    if not active_nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="All bound nodes are inactive")

    return [
        {
            "id": node.id,
            "node_name": node.node_name,
            "protocol": node.protocol,
            "server_address": node.server_address,
            "server_port": node.server_port,
            "outbound": build_singbox_outbound(node, user)
        }
        for node in active_nodes
    ]

@app.get("/api/user/verify", summary="用户 Token 验证与节点数据查询")
def verify_user_token(token: str = Query(..., description="用户 Token"), session: Session = Depends(get_session)):
    """用户侧 API：验证 Token、查库验证有效性，并返回所有绑定的节点字段与配置"""
    user = session.exec(select(User).where(User.token == token)).first()
    if not user or not user.is_active:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="用户 Token 无效或用户已被禁用")

    if not user.nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="该用户未绑定可用节点")

    active_nodes = [n for n in user.nodes if n.is_active]
    if not active_nodes:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="用户绑定的节点不存在或已被禁用")

    node_config = generate_singbox_config(active_nodes, user)
    return {
        "valid": True,
        "user_name": user.name,
        "token": user.token,
        "node_ids": [n.id for n in active_nodes],
        "nodes": [{"id": n.id, "node_name": n.node_name, "protocol": n.protocol} for n in active_nodes],
        "singbox_config": node_config
    }

# ------------------------------------------------------------------
# 7. 管理员 CRUD API - 节点管理 (/api/nodes)
# ------------------------------------------------------------------
@app.post("/api/nodes", response_model=NodeRead, dependencies=[Depends(verify_admin_token)], summary="创建节点")
def create_node(node_data: NodeCreate, session: Session = Depends(get_session)):
    validate_node_protocol_and_security(node_data.protocol, node_data.security, node_data.public_key, node_data.short_id)
    node = Node.model_validate(node_data)
    session.add(node)
    session.commit()
    session.refresh(node)
    return node

@app.get("/api/nodes", response_model=List[NodeRead], dependencies=[Depends(verify_admin_token)], summary="获取节点列表")
def list_nodes(session: Session = Depends(get_session)):
    return session.exec(select(Node)).all()

@app.get("/api/nodes/{node_id}", response_model=NodeRead, dependencies=[Depends(verify_admin_token)], summary="获取单个节点")
def get_node(node_id: int, session: Session = Depends(get_session)):
    node = session.get(Node, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    return node

@app.put("/api/nodes/{node_id}", response_model=NodeRead, dependencies=[Depends(verify_admin_token)], summary="更新节点")
def update_node(node_id: int, node_data: NodeUpdate, session: Session = Depends(get_session)):
    node = session.get(Node, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    
    update_dict = node_data.model_dump(exclude_unset=True)
    target_proto = update_dict.get("protocol", node.protocol)
    target_sec = update_dict.get("security", node.security)
    target_pbk = update_dict.get("public_key", node.public_key)
    target_sid = update_dict.get("short_id", node.short_id)
    validate_node_protocol_and_security(target_proto, target_sec, target_pbk, target_sid)

    for key, value in update_dict.items():
        setattr(node, key, value)
    
    session.add(node)
    session.commit()
    session.refresh(node)
    return node

@app.delete("/api/nodes/{node_id}", dependencies=[Depends(verify_admin_token)], summary="删除节点")
def delete_node(node_id: int, session: Session = Depends(get_session)):
    node = session.get(Node, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    
    session.delete(node)
    session.commit()
    return {"message": f"Node {node_id} deleted successfully"}

# ------------------------------------------------------------------
# 8. 管理员 CRUD API - 用户管理 (/api/users)
# ------------------------------------------------------------------
@app.post("/api/users", response_model=UserRead, dependencies=[Depends(verify_admin_token)], summary="创建用户")
def create_user(user_data: UserCreate, session: Session = Depends(get_session)):
    token = user_data.token or str(uuid_lib.uuid4())
    user_uuid = user_data.uuid or str(uuid_lib.uuid4())
    user_pwd = user_data.password or secrets.token_hex(8)
    
    existing = session.exec(select(User).where(User.token == token)).first()
    if existing:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="Token already exists")

    bound_nodes = []
    if user_data.node_ids:
        bound_nodes = session.exec(select(Node).where(Node.id.in_(user_data.node_ids))).all()

    user = User(
        name=user_data.name,
        token=token,
        uuid=user_uuid,
        password=user_pwd,
        is_active=user_data.is_active,
        nodes=bound_nodes
    )
    session.add(user)
    session.commit()
    session.refresh(user)

    res = UserRead(
        id=user.id,
        name=user.name,
        is_active=user.is_active,
        token=user.token,
        uuid=user.uuid,
        password=user.password,
        node_ids=[n.id for n in user.nodes]
    )
    return res

@app.get("/api/users", response_model=List[UserRead], dependencies=[Depends(verify_admin_token)], summary="获取用户列表")
def list_users(session: Session = Depends(get_session)):
    users = session.exec(select(User)).all()
    res = []
    for u in users:
        res.append(UserRead(
            id=u.id,
            name=u.name,
            is_active=u.is_active,
            token=u.token,
            uuid=u.uuid,
            password=u.password,
            node_ids=[n.id for n in u.nodes]
        ))
    return res

@app.get("/api/users/{user_id}", response_model=UserRead, dependencies=[Depends(verify_admin_token)], summary="获取单个用户")
def get_user(user_id: int, session: Session = Depends(get_session)):
    user = session.get(User, user_id)
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")
    return UserRead(
        id=user.id,
        name=user.name,
        is_active=user.is_active,
        token=user.token,
        uuid=user.uuid,
        password=user.password,
        node_ids=[n.id for n in user.nodes]
    )

@app.put("/api/users/{user_id}", response_model=UserRead, dependencies=[Depends(verify_admin_token)], summary="更新用户")
def update_user(user_id: int, user_data: UserUpdate, session: Session = Depends(get_session)):
    user = session.get(User, user_id)
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")
    
    update_dict = user_data.model_dump(exclude_unset=True)

    if "token" in update_dict and update_dict["token"] != user.token:
        existing = session.exec(select(User).where(User.token == update_dict["token"])).first()
        if existing:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="Token already exists")

    if "node_ids" in update_dict:
        node_ids = update_dict.pop("node_ids")
        if node_ids is not None:
            bound_nodes = session.exec(select(Node).where(Node.id.in_(node_ids))).all()
            user.nodes = bound_nodes

    for key, value in update_dict.items():
        setattr(user, key, value)
    
    session.add(user)
    session.commit()
    session.refresh(user)
    return UserRead(
        id=user.id,
        name=user.name,
        is_active=user.is_active,
        token=user.token,
        uuid=user.uuid,
        password=user.password,
        node_ids=[n.id for n in user.nodes]
    )

@app.delete("/api/users/{user_id}", dependencies=[Depends(verify_admin_token)], summary="删除用户")
def delete_user(user_id: int, session: Session = Depends(get_session)):
    user = session.get(User, user_id)
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")
    
    session.delete(user)
    session.commit()
    return {"message": f"User {user_id} deleted successfully"}

# ------------------------------------------------------------------
# 9. 直接运行服务入口
# ------------------------------------------------------------------
if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
