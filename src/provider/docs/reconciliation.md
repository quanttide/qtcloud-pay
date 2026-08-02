# reconciliation 对账与可查（M4）

包：`internal/reconciliation`（transport / service / model）

## 职责

账本可靠性的最后一环：一致性校验（不错）、对公打款核对、账单导出（可查）。

## 依赖

`account`、`transaction`

## 核心流程

- **一致性校验**：逐账户比对「余额字段」与「Σ(充值) − Σ(余额支付)」，输出差异清单（先 SQL 聚合、再逐账户明细核对）
- **对公打款核对**：上传银行流水 CSV → 解析（纯函数）→ 与充值交易按（金额 + 日期 / 凭证号）比对 → 差异报告
- **账单导出**：期初余额 + 流水 + 期末余额（复用 transaction 查询），CSV/JSON

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/accounts/{id}/statement` | 账单导出 |

## 测试

CSV 解析表驱动；构造不一致数据验证校验输出。
