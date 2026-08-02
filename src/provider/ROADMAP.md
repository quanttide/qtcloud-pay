# provider 开发路线图（待落实到代码的事项）

只列**直接需要落实到代码**的任务；背景与洞察见[域级路线图](../../../../data/roadmap/provider.md)与[计费同构洞察](../../../../data/insight/billing-isomorphism.md)。优先级与理由以域级路线图为准。

## 任务清单

| # | 优先级 | 任务 | 落点 | 状态 |
|---|--------|------|------|------|
| T1 | P1 | 方案下发：商品目录 | 新模型 `Product`（id、scope、定价）与下发/管理 API | 未开始 |
| T2 | P1 | 方案下发：计费规则管理 API | `BillingRule` 表已有（priority/condition），补 CRUD 端点 | 未开始 |
| T3 | P1 | 方案下发：券策略参数化 | `billing.Calculate` 硬编码的力度选择/抵扣顺序改为规则驱动 | 未开始，依赖 T1/T2 与 P0 契约确认 |
| T4 | P1 | 账本上报：结算报表 | `reconciliation` 扩展：按账期/账户的结算报表端点（回给商务云） | 未开始 |
| T5 | P2 | 支付回调自动入账（v0.2.0） | `channel` 回调（`ParseNotify`/`VerifyNotify`）→ `transaction.Append`（type=recharge，幂等键=渠道交易号），替代手动登记 | 未开始 |

## 已覆盖（无需新增代码）

- **按量/按次扣费**：`consume` 已实现（课堂按学习扣费、云按量、数据按交付进度）
- **多退少补**：退款登记 `POST /accounts/{id}/refunds`（`refund:{voucher_no}` 幂等）+ 再充值

## 影响实现方式的待定决策（等 P0 确认）

- **层边界仲裁**：满减力度选择、代金券不找零归商务云策略还是支付云执行语义——决定 T3 是「配置化」还是「保持硬编码、接口透传」
- **方案契约范围**：商品目录与适用范围的关系（scope 归商品目录还是独立维度）——决定 T1 的模型设计
