# -*- coding: utf-8 -*-
import json
import pytest
from fastapi.testclient import TestClient
from sqlmodel import Session, SQLModel, create_engine
from sqlmodel.pool import StaticPool

from app.database import get_session
from app.deps import verify_admin_token
from main import app
from app.models import Template, TemplateHistory


# ================== 测试夹具 (Fixtures) ==================

@pytest.fixture(name="session")
def session_fixture():
    """创建独立的内存 SQLite 数据库，避免污染真实数据。"""
    engine = create_engine(
        "sqlite:///:memory:",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )
    SQLModel.metadata.create_all(engine)
    with Session(engine) as session:
        yield session


@pytest.fixture(name="client")
def client_fixture(session: Session):
    """配置 TestClient，重写依赖项（数据库会话与鉴权）。"""
    def get_session_override():
        return session

    # 绕过管理员 Token 检查，直接通过
    def verify_admin_token_override():
        return True

    app.dependency_overrides[get_session] = get_session_override
    app.dependency_overrides[verify_admin_token] = verify_admin_token_override

    client = TestClient(app)
    yield client
    app.dependency_overrides.clear()


@pytest.fixture(name="seed_templates")
def seed_templates_fixture(session: Session):
    """预先植入 sing-box 和 mihomo 基础数据。"""
    singbox_tpl = Template(
        target="sing-box",
        name="sing-box 默认模板",
        content_format="json",
        content=json.dumps({"log": {"level": "info"}, "outbounds": [{"type": "direct", "tag": "direct"}]}),
        version=1,
    )
    mihomo_tpl = Template(
        target="mihomo",
        name="mihomo 默认模板",
        content_format="yaml",
        content="port: 7890\nproxy-groups:\n  - name: Proxy\n    type: select\n",
        version=1,
    )
    session.add_all([singbox_tpl, mihomo_tpl])
    session.commit()
    session.refresh(singbox_tpl)
    session.refresh(mihomo_tpl)

    # 写入初始版本历史
    assert singbox_tpl.id is not None
    assert mihomo_tpl.id is not None
    session.add_all([
        TemplateHistory(template_id=singbox_tpl.id, target="sing-box", version=1, content=singbox_tpl.content, remark="初始化"),
        TemplateHistory(template_id=mihomo_tpl.id, target="mihomo", version=1, content=mihomo_tpl.content, remark="初始化"),
    ])
    session.commit()
    return {"sing-box": singbox_tpl, "mihomo": mihomo_tpl}


# ================== 接口测试用例 ==================

def test_list_templates(client: TestClient, seed_templates):
    """测试获取模板列表。"""
    response = client.get("/api/templates")
    assert response.status_code == 200
    data = response.json()
    assert len(data) == 2
    targets = [item["target"] for item in data]
    assert "sing-box" in targets
    assert "mihomo" in targets


def test_get_template_by_target(client: TestClient, seed_templates):
    """测试查询指定内核模板。"""
    response = client.get("/api/templates/sing-box")
    assert response.status_code == 200
    data = response.json()
    assert data["target"] == "sing-box"
    assert data["version"] == 1
    assert "outbounds" in data["content"]

    # 测试不存在的目标
    response_404 = client.get("/api/templates/unknown-core")
    assert response_404.status_code == 404


def test_save_template_success(client: TestClient, seed_templates):
    """测试正常更新模板，验证主表版本递增和内容变更。"""
    new_json_content = json.dumps({"outbounds": [{"type": "block", "tag": "block"}]})
    payload = {
        "target": "sing-box",
        "content": new_json_content,
        "remark": "更新出站规则为 block",
    }
    response = client.put("/api/templates", json=payload)
    assert response.status_code == 200
    data = response.json()
    print("\n[DEBUG PUT Response]:", data)  # 👈 打印实际返回内容
    assert data["version"] == 2
    assert data["content"] == new_json_content


def test_save_template_invalid_json(client: TestClient, seed_templates):
    """测试提交格式损坏的 JSON。"""
    payload = {
        "target": "sing-box",
        "content": "{ broken_json: ",
        "remark": "非法格式",
    }
    response = client.put("/api/templates", json=payload)
    assert response.status_code == 400
    assert "校验失败" in response.json()["detail"]


def test_save_template_missing_required_field(client: TestClient, seed_templates):
    """测试业务规则校验（sing-box 缺少 outbounds）。"""
    payload = {
        "target": "sing-box",
        "content": json.dumps({"log": {"level": "debug"}}),
        "remark": "缺少 outbounds",
    }
    response = client.put("/api/templates", json=payload)
    assert response.status_code == 400
    assert "outbounds" in response.json()["detail"]


def test_save_template_mihomo_yaml(client: TestClient, seed_templates):
    """测试 mihomo YAML 的正常更新与缺少 proxy-groups 的校验。"""
    valid_yaml = "proxy-groups:\n  - name: Auto\n    type: url-test\n"
    res_ok = client.put("/api/templates", json={"target": "mihomo", "content": valid_yaml})
    assert res_ok.status_code == 200
    assert res_ok.json()["version"] == 2

    invalid_yaml = "port: 1080\nmode: global\n"
    res_err = client.put("/api/templates", json={"target": "mihomo", "content": invalid_yaml})
    assert res_err.status_code == 400
    assert "proxy-groups" in res_err.json()["detail"]


def test_get_template_histories(client: TestClient, seed_templates):
    """测试获取历史版本列表。"""
    # 再次保存一次，产生第 2 个版本
    new_content = json.dumps({"outbounds": []})
    client.put("/api/templates", json={"target": "sing-box", "content": new_content, "remark": "第二次修改"})

    response = client.get("/api/templates/sing-box/histories")
    assert response.status_code == 200
    histories = response.json()
    assert len(histories) == 2
    # 倒序排列，最新版本在前
    assert histories[0]["version"] == 2
    assert histories[1]["version"] == 1


def test_rollback_template(client: TestClient, seed_templates):
    """测试一键回滚逻辑。"""
    # 1. 取得 v1 的历史快照 ID
    histories = client.get("/api/templates/sing-box/histories").json()
    v1_history_id = histories[0]["id"]
    v1_content = histories[0]["content"]

    # 2. 提交新内容使其升至 v2
    v2_content = json.dumps({"outbounds": [{"tag": "v2-node"}]})
    client.put("/api/templates", json={"target": "sing-box", "content": v2_content, "remark": "升级至 v2"})

    # 3. 执行回滚到 v1
    rollback_res = client.post(f"/api/templates/sing-box/rollback/{v1_history_id}")
    assert rollback_res.status_code == 200
    current_data = rollback_res.json()

    # 回滚应产生新版本号 (v3)，但内容还原为 v1 的内容
    assert current_data["version"] == 3
    assert current_data["content"] == v1_content

    # 4. 再次检查历史表，应有 3 条记录（v1 初始, v2 修改, v3 回滚产生的记录）
    histories_after = client.get("/api/templates/sing-box/histories").json()
    assert len(histories_after) == 3
    assert "回滚" in histories_after[0]["remark"]