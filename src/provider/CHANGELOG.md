# Changelog — provider

## provider/v0.0.1 (2026-07-11)

- feat: 实现支付 API HTTP 服务（3 端点：POST /pay, GET /query/{id}, POST /refund）
- feat: 实现微信 JSAPI 支付（V3 API）
- feat: 实现支付宝 PagePay / WapPay / Query / Refund
- feat: Provider 接口抽象，支持可插拔支付提供商
- test: 13 个 API 集成测试覆盖正常、客户端错误、服务端错误、错误方法
- test: 子包单元测试覆盖 alipay/wechat 各 API 方法与传输错误
- test: 适配器测试覆盖数据格式转换与金额单位换算
