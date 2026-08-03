# AGENTS（qtcloud-pay · src/studio）

面向在 studio（Flutter）内工作的编码 agent 的指令。上级纪律见仓库根 [AGENTS.md](../../../AGENTS.md)。

## 本 scope 是什么

量潮支付工作台客户端（Flutter 桌面应用，Windows/macOS）：管理人员图形化工作台，对接服务端账本 API（`src/provider`）。**只做展示与表单，不承载账务逻辑**。

## 关键文件

| 文件 | 作用 |
|------|------|
| `README.md` | 定位、文档导航、开发命令 |
| `CONTRIBUTING.md` | 本 scope 贡献规范 |
| `doc/index.md` | 模块划分设计（screens / widgets / models / services） |
| `pubspec.yaml` | 依赖声明 |
| 服务端设计 `../provider/docs/index.md` | 账本 API 契约 |
| 域级设计 `../../../data/roadmap/studio.md`（主仓库） | 页面与组件需求来源 |

## 纪律

1. **账务逻辑不落客户端**：金额、状态、抵扣明细展示以服务端 API 契约为准，不做浮点推导
2. 新增页面先更新 `doc/index.md` 模块划分，再实现
3. 服务端地址与敏感配置不写入代码（走配置注入）

## 验证

```bash
flutter test
```

提交前核对：测试通过、`doc/index.md` 已同步、CHANGELOG 是否需更新。
