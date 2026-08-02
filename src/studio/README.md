# qtcloud_pay_studio

量潮支付工作台客户端 — 管理人员图形化工作台（Flutter 桌面应用，Windows/macOS）。

对接服务端账本 API（`src/provider`），把充值登记、发券、订单结算、对账等操作包装成页面；客户端只做展示与表单，不承载账务逻辑。

## 文档

- [doc/index.md](doc/index.md) — 模块划分设计（screens / widgets / models / services）
- [服务端设计文档](../provider/docs/index.md) — 账本 API 模块划分
- [工作台设计](../../../../data/roadmap/studio.md) — 页面与组件需求来源

## 开发

```bash
flutter pub get
flutter run -d windows   # 或 -d macos
flutter test
```
