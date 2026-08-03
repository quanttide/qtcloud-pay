# 贡献指南 · qtcloud-pay Studio

`src/studio`（Flutter）——量潮支付工作台客户端：管理人员图形化工作台（Windows/macOS 桌面）。上级纪律见仓库根 [CONTRIBUTING.md](../../CONTRIBUTING.md)。

## 定位

- 对接服务端账本 API（`src/provider`），把充值登记、发券、订单结算、对账等操作包装成页面
- **客户端只做展示与表单，不承载账务逻辑**：金额换算、状态语义、幂等键构造一律由服务端处理
- 页面与组件需求来源：主仓库 `data/roadmap/studio.md`

## 模块划分

见 [doc/index.md](doc/index.md)（screens / widgets / models / services 分层）。

## 提交规范

Conventional Commits，中文描述，作用域 `studio`：

```
feat(studio): 新增充值登记页面
```

## 开发命令

```bash
flutter pub get
flutter run -d windows   # 或 -d macos
flutter test
```

## 关键纪律

1. **账务逻辑不落客户端**：金额用整数分或服务端返回的展示字段，不做浮点运算推导
2. **契约来自服务端**：状态、抵扣明细等展示数据以 provider API 契约为准，客户端不自行定义语义
3. **密钥与配置**：不写入客户端代码（服务端地址等走配置注入）
4. 新增页面先更新 [doc/index.md](doc/index.md) 模块划分，再实现

## 测试

```bash
flutter test
```

覆盖：widget 渲染、表单校验、service 层 HTTP 调用与错误展示。
