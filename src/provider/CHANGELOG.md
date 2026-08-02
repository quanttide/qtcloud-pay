# CHANGELOG

## [Unreleased]

### Added
- 新增 `cmd/server` 入口：从环境变量加载配置、组装 Provider、启动服务，支持优雅关闭
- 新增 `internal/middleware` 请求日志中间件（方法、路径、状态码、耗时）
- 新增 `Makefile`（build/run/test/vet/lint/clean）

### Changed
- 按标准 Go 项目布局重构结构：`internal/` 私有代码分层，`channel` 渠道模块（transport/service/adapters/model），`wechat`/`alipay` 移入 `internal/channel/`（不再可外部导入）
- 配置由构造时传参改为环境变量注入（`WECHAT_*` / `ALIPAY_*`）
- 更新 README：使用方式由库引用改为服务运行（环境变量 + HTTP API）

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
