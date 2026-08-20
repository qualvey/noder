# -*- coding: utf-8 -*-
"""
Mihomo 导出器测试 (test_mihomo.py)
1. VLESS + REALITY + vision 字段映射
2. TUIC v5 字段映射
3. 导出守卫: mihomo 不支持 anytls 明确报错
4. YAML 输出可回读
5. 模板注入: data/mihomo.yml 示例占位替换 / 特殊型保留 / 策略组接线
运行: uv run python test_mihomo.py
"""
import pathlib
import sys
import tempfile

# Windows 控制台 UTF-8 输出
if sys.stdout.encoding and sys.stdout.encoding.lower() not in ("utf-8", "utf8"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

import yaml
from fastapi import HTTPException

import app.config as cfg
from app.exporters.mihomo import (
    build_mihomo_config_yaml,
    build_mihomo_proxy,
    build_mihomo_proxies,
    build_mihomo_proxies_yaml,
    generate_mihomo_config,
)
from app.models import Node, User
from app.services.template_render import build_template_context

PASS = 0
FAIL = 0


def check(name, cond, extra=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  ✅ {name}")
    else:
        FAIL += 1
        print(f"  ❌ {name} {extra}")


def expect_400(name, fn, keyword=""):
    try:
        fn()
        check(name, False, "expected HTTPException 400 but none raised")
    except HTTPException as e:
        ok = e.status_code == 400 and (not keyword or keyword in str(e.detail))
        check(name, ok, f"status={e.status_code} detail={e.detail}")


def main():
    user = User(
        name="u", token="tok-123456",
        uuid="11111111-2222-3333-4444-555555555555", password="pwd-abc",
        is_active=True,
    )

    print("== 1. VLESS + REALITY + vision 映射 ==")
    vless = Node(
        tag="hk-01", node_name="HK", protocol="vless",
        server_address="1.2.3.4", server_port=8443,
        security="reality", sni="aws.amazon.com",
        public_key="99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo", short_id="1a91",
        fingerprint="chrome", flow="xtls-rprx-vision", is_active=True,
    )
    p = build_mihomo_proxy(vless, user)
    check("name=tag", p.get("name") == "hk-01")
    check("type=vless", p.get("type") == "vless")
    check("server/port", p.get("server") == "1.2.3.4" and p.get("port") == 8443)
    check("uuid=用户凭证", p.get("uuid") == user.uuid)
    check("network=tcp", p.get("network") == "tcp")
    check("udp=true", p.get("udp") is True)
    check("tls=true", p.get("tls") is True)
    check("servername=SNI", p.get("servername") == "aws.amazon.com")
    check("flow=vision", p.get("flow") == "xtls-rprx-vision")
    ro = p.get("reality-opts", {})
    check("reality-opts.public-key", ro.get("public-key") == "99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo")
    check("reality-opts.short-id", ro.get("short-id") == "1a91")
    check("client-fingerprint", p.get("client-fingerprint") == "chrome")

    print("== 2. TUIC v5 映射 ==")
    tuic = Node(
        tag="tuic-01", node_name="TUIC", protocol="tuic",
        server_address="t.example.com", server_port=8443,
        security="tls", congestion_control="cubic", is_active=True,
    )
    p2 = build_mihomo_proxy(tuic, user)
    check("type=tuic", p2.get("type") == "tuic")
    check("uuid=用户凭证", p2.get("uuid") == user.uuid)
    check("password=用户凭证", p2.get("password") == user.password)
    check("alpn=[h3]", p2.get("alpn") == ["h3"])
    check("congestion-controller=透传", p2.get("congestion-controller") == "cubic")
    check("udp-relay-mode=native", p2.get("udp-relay-mode") == "native")
    check("reduce-rtt=false(与 sing-box 一致)", p2.get("reduce-rtt") is False)
    check("tuic-version=5", p2.get("tuic-version") == 5)

    print("== 3. 导出守卫 ==")
    anytls = Node(
        tag="a1", node_name="A", protocol="anytls",
        server_address="a.com", server_port=443,
        security="tls", is_active=True,
    )
    expect_400("mihomo 不支持 anytls 明确报错", lambda: build_mihomo_proxy(anytls, user), "Mihomo")

    print("== 4. YAML 输出 ==")
    y = build_mihomo_proxies_yaml([vless, tuic], user)
    check("YAML 含 vless 节点", "- name: hk-01" in y, y)
    check("YAML 含 reality-opts", "reality-opts:" in y)
    check("YAML 含 tuic 节点", "- name: tuic-01" in y)
    data = yaml.safe_load(y)
    check("YAML 可回读且 2 条", isinstance(data, list) and len(data) == 2, str(data))
    check("回读 type 顺序", [d["type"] for d in data] == ["vless", "tuic"])
    # 守卫语义: 列表里混入不支持的协议 -> 整体拒绝 (不静默跳过)
    expect_400("YAML 列表含 anytls 整体拒绝", lambda: build_mihomo_proxies_yaml([vless, tuic, anytls], user), "Mihomo")

    print("== 5. 模板注入 (data/mihomo.yml) ==")
    # 临时模板文件：示例占位 proxies + 特殊型 dns-out + 策略组
    tmp_tpl = pathlib.Path(tempfile.mktemp(suffix=".yml"))
    tmp_tpl.write_text("""\
mixed-port: 10801
proxies:
  - name: "vless-reality-vision"
    type: vless
    server: server
    port: 443
    uuid: uuid
  - name: tuic
    type: tuic
    server: server
    port: 10443
    uuid: 00000000-0000-0000-0000-000000000001
    password: PASSWORD_1
  - name: "dns-out"
    type: dns
proxy-groups:
  - name: "auto"
    type: url-test
    proxies:
    url: "https://cp.cloudflare.com/generate_204"
  - name: Proxy
    type: select
    proxies:
      - ss1
      - auto
rules:
  - MATCH,Proxy
""", encoding="utf-8")
    cfg.MIHOMO_TEMPLATE_PATH = tmp_tpl

    config = generate_mihomo_config([vless, tuic], user)
    check("模板非节点字段保留", config.get("mixed-port") == 10801, str(config.get("mixed-port")))
    pnames = [p["name"] for p in config["proxies"]]
    check("示例占位 vless 被替换", "vless-reality-vision" not in pnames, str(pnames))
    check("示例占位 tuic 被替换", "tuic" not in pnames, str(pnames))
    check("真实节点注入", set(pnames) == {"hk-01", "tuic-01", "dns-out"}, str(pnames))
    check("特殊型 dns-out 保留", any(p.get("type") == "dns" for p in config["proxies"]))
    check("真实节点为生成字段", next(p for p in config["proxies"] if p["name"] == "hk-01").get("reality-opts") is not None)

    groups = {g["name"]: g for g in config["proxy-groups"]}
    check("Proxy 组含组引用 auto", "auto" in groups["Proxy"]["proxies"])
    check("Proxy 组剔除示例引用 ss1", "ss1" not in groups["Proxy"]["proxies"])
    check("Proxy 组接入真实节点", set(groups["Proxy"]["proxies"]) == {"auto", "hk-01", "tuic-01"}, str(groups["Proxy"]["proxies"]))
    check("url-test 组替换为节点列表", set(groups["auto"]["proxies"]) == {"hk-01", "tuic-01"}, str(groups["auto"]["proxies"]))

    y2 = build_mihomo_config_yaml([vless, tuic], user)
    back = yaml.safe_load(y2)
    check("完整配置 YAML 可回读", back["mixed-port"] == 10801 and len(back["proxies"]) == 3)

    try:
        tmp_tpl.unlink()
    except OSError:
        pass

    print("== 6. 模板占位符懒加载 (mihomo 守卫不阻断普通 zip) ==")
    anytls_user = User(
        name="anytls-user", token="u2",
        uuid="22222222-2222-3333-4444-555555555555", password="p2",
        nodes=[anytls], is_active=True,
    )
    ctx_without = build_template_context(anytls_user, include_mihomo=False)
    check("不含 mihomo 占位符的上下文不触发守卫", "mihomo_proxies_yaml" not in ctx_without)
    expect_400(
        "含 mihomo 占位符时守卫仍然拦截 (anytls)",
        lambda: build_template_context(anytls_user, include_mihomo=True),
        "Mihomo",
    )

    print(f"\n结果: {PASS} 通过, {FAIL} 失败")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
