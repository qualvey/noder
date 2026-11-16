# Noder 测试工程与质量保证文档中心

欢迎查阅 **Noder (Sing-Box / Mihomo / Xray 订阅中间件与代理控制面服务)** 测试工程中心。本目录包含系统的全景测试策略、详细测试用例规约、端到端集成验收规范及自动化测试执行指南。

---

## 📚 测试文档导航

| 规范文档 | 内容说明 | 适用对象 |
| :--- | :--- | :--- |
| [**全景测试策略规范 (`test_strategy.md`)**](./test_strategy.md) | 测试金字塔分层模型、各层覆盖率指标、测试数据治理、双数据库驱动兼容性与质量门禁 (DoD) | QA 负责人、架构师、后端研发 |
| [**业务域测试用例规约 (`test_cases.md`)**](./test_cases.md) | 涵盖 7 大核心域（协议契约、用户生命周期、订阅引擎、文件分发、Mall 交付、Agent 控制面、安全健壮性）的详细测试用例表 | 测试工程师、功能测试、开发自测 |
| [**Xray Agent E2E 集成验收规范 (`../e2e/xray-agent-integration.md`)**](../e2e/xray-agent-integration.md) | 真机与多节点联动、网络握手、边缘断网自愈、版本冲突防御、流量计费与断连阻断验收 | 交付工程师、运维、E2E 测试 |
| [**系统行为锁定测试准则 (`../micro-service/refactor/07-behavior-test.md`)**](../micro-service/refactor/07-behavior-test.md) | 重构前后外部业务契约与状态流转一致性保护准则 | 核心开发、回归测试 |

---

## 🛠️ 快速执行命令表

### 1. 运行全量测试
```bash
# 执行所有后端单元测试与集成测试
go test ./...

# 显示详细执行过程
go test -v ./...
```

### 2. 查看代码覆盖率
```bash
# 控制台查看各模块覆盖率
go test -cover ./...

# 生成可视化 HTML 覆盖率报告并在浏览器查看
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 3. 按模块单跑测试
```bash
# 核心订阅生成模块
go test -v -cover ./internal/service/singbox/...
go test -v -cover ./internal/service/mihomo/...
go test -v -cover ./internal/service/xray/...

# 文件与分发存储模块
go test -v -cover ./internal/service/file/...

# 商业交付与 Agent 控制面模块
go test -v ./internal/handler/... -run "TestService|TestAgent|TestSpec"
```

### 4. 前端类型检查与构建测试
```bash
cd web
pnpm install
pnpm run build
```

---

## 🎯 核心质量指标现状 (Quality Metrics Dashboard)

- **全量测试通过率**：100% (All packages PASS)
- **协议契约层 (`internal/contract`)**：覆盖率 ~52% (严格校验 VLESS/TUIC/XHTTP 契约)
- **数据持久层 (`internal/db`)**：覆盖率 ~74% (生命周期隔离、级联清理、审计)
- **Mihomo 订阅生成 (`internal/service/mihomo`)**：覆盖率 **84.4%**
- **Sing-Box 订阅生成 (`internal/service/singbox`)**：覆盖率 **61.7%**
- **Xray 订阅生成 (`internal/service/xray`)**：覆盖率 **76.2%**
- **分发文件服务 (`internal/service/file`)**：覆盖率 **89.9%**
- **模板渲染管道 (`internal/service/template`)**：覆盖率 **77.9%**
- **接口控制层 (`internal/handler`)**：覆盖率包含完整的用户、节点、鉴权、设置、模板历史回滚、Mall 自动化交付及 Agent 同步测试
