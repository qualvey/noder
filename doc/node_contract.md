# 节点结构契约设计文档 (Node Contract)

## 1. 背景与目标

系统未来要支持多代理核心导出（Sing-Box / Mihomo / 其他）。核心洞察：

> **节点的参数不变，只是导出的语法不一样。**

因此引入**节点结构契约 (Node Contract)**：在核心读取阶段（节点创建/更新）按协议严格校验字段结构，
确保入库的节点数据合法；导出阶段由各核心的导出器按自身语法转换。

## 2. 契约模型 (app/contracts.py)

```python
@dataclass
class NodeContract:
    protocol: str                          # 协议名
    required: set                          # 必须存在的字段（值不能为空）
    optional: set                          # 可选字段（文档性约束）
    deps: Dict[str, Dict[str, List[str]]]  # 条件依赖: {字段: {取值: [必须存在的字段]}}
    fixed: Dict[str, str]                  # 固定取值: {字段: 必须等于的值}
    conflicts: List[Tuple]                 # 互斥: [(字段A, 取值A, 字段B, 取值B)]
```

### 当前协议契约

| 协议 | required | optional | fixed | deps | enum |
| :--- | :--- | :--- | :--- | :--- | :--- |
| tuic | tag, server_address, server_port, uuid, password | congestion_control | security=tls | - | congestion_control: bbr/cubic/new_reno |
| vless | tag, server_address, server_port, uuid | node_name, flow | security=reality | security=reality → public_key, short_id, sni, fingerprint | flow: "" / xtls-rprx-vision；fingerprint: chrome/firefox/edge/safari/360/qq/ios/android/random/randomized |
| anytls | server_address, server_port | node_name, method, sni, transport_type, path, remark | - | security=reality → public_key, short_id | - |

> **字段归属说明**：`tag`/`node_name`/`server_address`/`server_port`/`security`/`public_key`/`short_id`/`fingerprint`/`flow`/`congestion_control` 等为**节点自有字段**（存 Node 表）；`uuid`/`password` 为**用户级凭证**（存 User 表，每个用户独立，见 models.py 方案 3）。

### 校验规则（两阶段）

`validate_node_contract(values, protocol, node_level=False)` 按顺序检查（protocol 为必传参数）：

1. **必填字段**：`required` 中缺失或为空的字段 → 400
2. **固定取值**：`fixed` 中字段不等于约定值 → 400（如 tuic 必须 tls）
3. **条件依赖**：字段取特定值时必须同时提供依赖字段 → 400（如 reality 必须有 public_key/short_id/sni/fingerprint）
4. **枚举取值**：`enum` 中字段不在白名单 → 400（如 flow 只能是空或 xtls-rprx-vision）
5. **互斥**：字段取值组合冲突 → 400

**节点创建/更新**（`node_level=True`）：只校验节点自有字段（`required ∩ NODE_OWNED_FIELDS`），
uuid/password 等用户级凭证不在节点 payload 中，此时不校验。

**导出阶段**（`build_singbox_outbound`，`node_level=False`）：节点字段与用户凭证合并成完整视图后全量校验——
tuic 缺 uuid/password、vless 缺 uuid 等直接 400，**不静默导出坏配置**（与不支持协议明确报错同一原则）。

## 3. 代理核心注册表 (CORE_REGISTRY)

每个核心声明支持的协议列表与输出格式：

```python
CORE_REGISTRY = {
    "singbox": CoreInfo(key="singbox", name="Sing-Box",
                        supported_protocols={"tuic", "vless", "anytls"},
                        content_type="application/json"),
    "mihomo": CoreInfo(key="mihomo", name="Mihomo",
                        supported_protocols={"vless", "tuic"},  # anytls 不支持
                        content_type="text/yaml"),
}
```

**导出守卫**：`assert_protocol_supported(core, protocol)` —— 导出时若节点协议不在核心支持列表内，
**明确报错**（HTTP 400，带核心名与支持列表），不静默跳过、不降级。

已在 `app/services/singbox.py`（全部协议）与 `app/exporters/mihomo.py`（vless/tuic）入口接入；
anytls 节点导出到 mihomo 时被守卫明确拒绝（保留未来扩展，mihomo 支持后加入集合即可）。

**Mihomo 导出**：`app/exporters/mihomo.py` 生成 mihomo 风格 proxies 条目（YAML），
覆盖主要使用组合 vless+reality+vision 与 tuic v5：
- 用户侧端点 `GET /mihomo?token=...` 返回 proxies 列表 YAML（text/yaml）
- 模板占位符 `{{mihomo_proxies_yaml}}`（见 doc/file_distribution.md）

## 4. 接入新核心的流程（未来）

1. `CORE_REGISTRY` 注册新核心（key/name/supported_protocols/content_type）
2. 新增 `app/exporters/{core}.py`：`build_{core}_proxy(node, user)` + 模板注入（mihomo 已落地，作为样板）
3. 数据层 Node 表按需加字段（超集，不破坏现有）
4. 契约层按需补充新协议的 required/deps/fixed/enum

## 5. 测试

`uv run python test_node_contract.py`：
- 契约单元校验：tuic/vless 固定取值、reality 依赖（含 sni/fingerprint）、枚举取值（flow/fingerprint/congestion_control）、必填缺失、未知协议、全量/节点级两阶段校验
- API 层：POST /api/nodes 非法数据 400 / 合法数据 200
- 核心注册表：singbox 全协议 / mihomo 仅 vless+tuic、不支持协议明确报错

`uv run python test_mihomo.py`：mihomo 导出字段映射（vless+reality+vision / tuic v5）、anytls 守卫拒绝、YAML 回读。

回归：test_verification.py / test_config_override.py / test_file_dist.py / test_e2e_http.py 全部通过。

## 6. 版本

- pyproject: 0.8.0（待 bump 至 0.9.0 随前端一起发版）
- Node 表新增 `tag`（outbound 标识，契约必填）与 `congestion_control`（tuic 可选）；
  老库自动 ALTER 迁移 + `tag=node_name` 回填；导出阶段合并用户凭证全量校验
- `validate_node_contract(values, protocol, node_level=False)`：protocol 为**必传参数**（不依赖 values 兜底）
- 契约机制含 enum 取值白名单（flow/fingerprint/congestion_control，对齐 sing-box 合法值）
- 新增模块：`app/contracts.py`、`app/exporters/mihomo.py`；端点 `GET /mihomo`（YAML）
- 重构：`models.py` 的协议校验迁移至契约驱动；`routers/nodes.py` 改用契约校验；`singbox.py`/`mihomo.py` 加导出守卫
