# qtcloud-pay lite · 量潮钱包（lite）

用户端自服务客户端（Flutter）——**用户管理自己的钱包**：余额、交易流水（只读）。

## 定位

- 面向普通用户：看自己的余额与流水
- 登录复用统一账号入口（qtcloud-auth `/oauth/token`），customerId 取 userinfo 的 sub
- 管理人员图形化工作台走同仓 `src/studio`；账本核心与渠道走 `src/provider`
- 前身是 `domains/quanttide-auth/apps/qtaccount`（账号+钱包双 tab 一体），按域拆分后钱包侧落此

## 开发

```bash
flutter pub get
flutter run -d linux --dart-define=QTCLOUD_AUTH_BASE_URL=http://localhost:8080 --dart-define=QTCLOUD_PAY_BASE_URL=http://localhost:8080
flutter test
```
