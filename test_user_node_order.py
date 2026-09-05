# -*- coding: utf-8 -*-
"""测试针对用户维度的节点自定义排序与置顶首选导出。"""
import json
import uuid
from sqlmodel import Session, select

from app.database import engine, create_db_and_tables
from app.models import Node, User, get_sorted_node_ids
from app.routers.users import _to_read
from app.routers.subscription import _get_user_ordered_active_nodes
from app.services.singbox import generate_singbox_config
from app.services.mihomo import generate_mihomo_config


def test_user_node_ordering():
    create_db_and_tables()

    with Session(engine) as session:
        # 创建 3 个测试节点
        node_a = Node(
            node_name="Node A",
            tag="tag-a",
            protocol="vless",
            server_address="a.example.com",
            server_port=443,
            security="reality",
            public_key="public_key_a",
            short_id="short_id_a",
            sni="sni.a.com",
            is_active=True,
        )
        node_b = Node(
            node_name="Node B",
            tag="tag-b",
            protocol="tuic",
            server_address="b.example.com",
            server_port=8443,
            security="tls",
            sni="sni.b.com",
            is_active=True,
        )
        node_c = Node(
            node_name="Node C",
            tag="tag-c",
            protocol="vless",
            server_address="c.example.com",
            server_port=443,
            security="reality",
            public_key="public_key_c",
            short_id="short_id_c",
            sni="sni.c.com",
            is_active=True,
        )
        session.add_all([node_a, node_b, node_c])
        session.commit()
        session.refresh(node_a)
        session.refresh(node_b)
        session.refresh(node_c)

        # 1. 创建用户，故意将 Node B 放在首位: [B, C, A]
        custom_order = [node_b.id, node_c.id, node_a.id]
        test_user = User(
            name="OrderTestUser",
            token=f"order-token-{uuid.uuid4().hex[:8]}",
            uuid=str(uuid.uuid4()),
            password="secret-password",
            is_active=True,
            nodes=[node_a, node_b, node_c],
            node_order=json.dumps(custom_order),
        )
        session.add(test_user)
        session.commit()
        session.refresh(test_user)

        # 2. 验证 get_sorted_node_ids
        sorted_ids = get_sorted_node_ids(test_user)
        assert sorted_ids == custom_order, f"Expected {custom_order}, got {sorted_ids}"

        # 3. 验证 _to_read
        user_read = _to_read(test_user)
        assert user_read.node_ids == custom_order

        # 4. 验证 _get_user_ordered_active_nodes
        ordered_nodes = _get_user_ordered_active_nodes(test_user)
        assert len(ordered_nodes) == 3
        assert ordered_nodes[0].id == node_b.id
        assert ordered_nodes[1].id == node_c.id
        assert ordered_nodes[2].id == node_a.id

        # 5. 验证 Singbox 配置首个节点
        sb_config = generate_singbox_config(ordered_nodes, test_user)
        proxy_ob = next(ob for ob in sb_config["outbounds"] if ob.get("tag") == "Proxy")
        # 验证 Proxy 候选列表包含 node_b 的 tag
        assert "tag-b" in proxy_ob["outbounds"]
        # 验证 urltest 列表首位为 tag-b
        urltest_ob = next(ob for ob in sb_config["outbounds"] if ob.get("tag") == "urltest")
        assert urltest_ob["outbounds"][0] == "tag-b"

        # 6. 验证 Mihomo 配置中真实节点顺序首位为 tag-b，且 url-test 策略组首位为 tag-b
        mh_config = generate_mihomo_config(ordered_nodes, test_user)
        real_proxy_names = [p["name"] for p in mh_config["proxies"] if p.get("name") != "dns-out"]
        assert real_proxy_names == ["tag-b", "tag-c", "tag-a"]
        auto_group = next(g for g in mh_config["proxy-groups"] if g.get("type") in ("url-test", "fallback"))
        assert auto_group["proxies"][0] == "tag-b"

        # 7. 更新用户顺序：将 Node A 置顶: [A, B, C]
        new_order = [node_a.id, node_b.id, node_c.id]
        test_user.node_order = json.dumps(new_order)
        session.add(test_user)
        session.commit()
        session.refresh(test_user)

        updated_nodes = _get_user_ordered_active_nodes(test_user)
        assert updated_nodes[0].id == node_a.id
        assert updated_nodes[1].id == node_b.id
        assert updated_nodes[2].id == node_c.id

        print("ALL USER NODE ORDERING TESTS PASSED SUCCESSFULLY!")


if __name__ == "__main__":
    test_user_node_ordering()
