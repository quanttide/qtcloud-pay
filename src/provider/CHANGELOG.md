# CHANGELOG

## [0.1.0-alpha.3] - 2026-08-03

### Fixed
- RDS 服务关联角色修正：正确名称 `AliyunServiceRoleForRdsPgsqlOnEcs`（rds 产品 API），已创建并改为文档化一次性前置（不再由 terraform 管理，见 docs/dev-guide/iac.md）

### Changed
- 移除 workflow_dispatch 手动触发：部署仅由 `provider/*` tag 驱动（生产保护）

## [0.1.0-alpha.2] - 2026-08-03

### Changed
- 部署选型落地：环境默认 `prod`，数据库账号 `qtcloud_pay`；VPC/RDS 归入系统级 `quanttide` 命名与资源组（命名规则见 docs/dev-guide/iac.md）
- Docker 镜像改名 `qtcloud-pay-provider`（本地 Makefile 标签同步，为 cli/studio 镜像预留命名空间）
- 生产保护：RDS/VPC `prevent_destroy` + RDS 删除保护、OSS 状态桶版本控制、deploy 挂 `production` 环境（可配 Required reviewers）

### Fixed
- CI 部署链路：阿里云凭证 secret 改用 `ALIYUN_ACCESS_KEY_ID` / `ALIYUN_ACCESS_KEY_SECRET`；镜像名改经 `GITHUB_ENV` 传递（job output 含 secret 会被 GitHub 脱敏置空）
- 创建 RDS PostgreSQL 服务关联角色（`AliyunServiceRoleForRdsPgsqlOnEcs`，rds 产品 API），解决首次开通 `ServiceLinkedRole.NotExist`

## [0.1.0-alpha.1] - 2026-08-03

### Added
- 新增 `cmd/server` 入口：从环境变量加载配置、组装依赖、启动服务，支持优雅关闭
- 新增 `internal/middleware` 请求日志中间件（方法、路径、状态码、耗时）
- 新增 `Makefile`（build/run/test/vet/lint/clean）
- **实现账本核心（服务端 v0.1.0，M1–M4）**：
  - `internal/account`：账户与余额，充值登记（打款凭证号幂等）
  - `internal/transaction`：交易账本，账本写入唯一入口（幂等键 + 唯一约束），流水查询
  - `internal/coupon` / `internal/voucher`：优惠券（折扣/满减）与代金券批量发放（批次号幂等）、过期惰性流转、结算核销/抵现
  - `internal/billing`：计费规则，默认抵扣顺序「满减 → 折扣 → 代金券 → 余额」（纯计算）
  - `internal/order`：订单与结算，单事务编排（锁账户串行化、消费/核销交易、结算明细快照）
  - `internal/reconciliation`：对账与可查（余额-交易一致性校验、银行流水 CSV 核对、账单导出）
- 存储：GORM 统一调度，开发 SQLite / 生产 PostgreSQL 方言切换；AutoMigrate 建表
- 账本核心单元测试覆盖各包 95% 以上（多数 100%）

### Changed
- 按标准 Go 项目布局重构结构：`internal/` 私有代码分层，`channel` 渠道模块（transport/service/adapters/model），`wechat`/`alipay` 移入 `internal/channel/`（不再可外部导入）
- 配置由构造时传参改为环境变量注入（`WECHAT_*` / `ALIPAY_*`，存储用 `DB_DRIVER` / `DATABASE_URL` / `DB_SQLITE_DSN`）
- 更新 README：使用方式由库引用改为服务运行（环境变量 + HTTP API），补充账本核心 API
- 抽取 `internal/app` 组装包（`Open` / `OpenDB` / `BuildMux` / `NewProvider`），生产入口与测试共用
- Python 端到端测试：编译真实二进制 + 临时 SQLite 库启动服务，`tests/` 下覆盖 tests.md 全部 25 个 TC（含并发幂等、过期状态机、三业务总对账）

### Fixed
- 修复核销交易幂等键冲突：优惠券与代金券自增 ID 各自从 1 开始，幂等键加入抵扣类型区分（`settle:{order}:redeem:{kind}:{id}`）
- 修复发券交易幂等键跨类型冲突：优惠券与代金券可能使用相同批次号，幂等键按类型命名空间（`issue:coupon:{batch}` / `issue:voucher:{batch}`）
- 修复折扣券语义：rate 为折扣力度（9 折 = rate 90 = 省 10%），抵扣 = 应付 × (100 − rate) / 100；多张折扣券选力度最大（rate 最低）
- 修复 SQLite 并发写锁：`app.Open` 对非 PostgreSQL 驱动限制连接池为单连接（`SetMaxOpenConns(1)`），消除文件库并发结算 `database is locked` → 500（Go `:memory:` 测试因自设单连接未暴露，Python 端到端并发用例才复现）

### Changed
- **金额传输改为元（两位小数数字）**：新增 `pkg/money.Cents`（JSON 元 ↔ 内部分，严格校验拒绝三位及以上小数）；transport 请求/响应金额字段统一经 `Cents` 转换，内部账本仍为整数分（service/repository 零改动）；`POST /reconcile/bank` CSV 保持分（`amount_cents`）；文档与三层测试（单测/itest/端到端）同步更新
- 新增 Docker 部署：多阶段 `Dockerfile`（golang:1.26-alpine 构建含 SQLite CGO 工具链 → alpine:3.20 非 root 运行）+ `.dockerignore` + Makefile `docker-build`/`docker-run`

## [0.0.1] - 2026-07-11

### Added
- 初始化项目配置和支付 API HTTP 服务  
- 实现微信 JSAPI 和支付宝页面支付提供者  
- 新增 Provider 接口及 Go 模块  
- 添加 API 参考、开发者指南、用户指南等文档  
- 新增 WrongMethod/InvalidBody API 测试，提升 wechat/alipay 包测试覆盖率至 95% 以上  

### Changed
- 更新 README，补充实现细节并简化范围，添加 ROADMAP 文档  

### Fixed
- 修复 SetTransport 未委托给 gopay http 客户端的问题，同时修正 provider_test 中的 mock 代码
