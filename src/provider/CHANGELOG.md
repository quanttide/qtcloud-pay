# CHANGELOG

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
