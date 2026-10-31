# 规则设计（离线草稿）

客户端位置：`lib/screens/rule_design_screen.dart`、`lib/models/rule_draft.dart`、`lib/services/local_draft_store.dart`

## 职责

**完全离线**设计代金券发放规则：草稿存本地 JSON（`~/.qtcloud-pay-studio/rule_drafts.json`），不接触服务端。设计产物是服务端计价规则集 payload 的 `issuance` 段（发放渠道 + 面额规则），校验通过后导出 PUT 请求体，联调时由管理员提交到 `PUT /admin/voucher-pricing-rules/{id}`。

与「发券页·发放规则页签」的分工：那边只读展示服务端已生效规则；这边离线设计未来规则。

## 页面与组件

| 页面/组件 | 说明 |
|-----------|------|
| `RuleDesignScreen` | 左栏草稿列表（新建/删除），右栏编辑区；可注入 `LocalDraftStore`（测试） |
| `_DraftEditor` | 元信息（source/version）、渠道列表、面额规则、校验结果、导出按钮 |
| `_ChannelDialog` | 渠道编辑：普通渠道（固定面额）/ 追加奖励渠道（面额规则 + bonus_type）二选一 |
| `LocalDraftStore` | 本地草稿仓：单 JSON 文件读写；文件损坏返回空列表不抛错 |

## 校验（对齐服务端 `validatePricingRuleSet` 发放段）

- 渠道必填：名称、触发条件、有效期规则、每次事件张数（正整数或非空描述）
- 普通渠道：`amount_cents` > 0 且不带 `amount_rule`
- 追加奖励渠道：`bonus_type` ∈ first_completion / output_assessment / group_interaction，`amount_rule` 必填，不带固定面额；含追加奖励时 `bonus_denomination_rule` 必填
- scope ∈ all / cloud / course / data / product

核销场景（`redemption.scenarios`）不在本页设计范围，联调时在服务端补齐——导出前校验面板会提示这一点。

## 导出

校验通过后「复制 PUT 请求体」：`{"source", "version", "payload": {meta, issuance, billing_semantics.voucher_is_money=true}}`，可直接作为 `PUT /admin/voucher-pricing-rules/{id}` 的 JSON body。

## 测试

单元测试：校验各分支（缺字段/面额为 0/追加奖励规则）、payload 形状对齐、JSON 往返、本地仓读写与损坏容错。
widget 测试：新建草稿 → 填合法渠道 → 校验通过 → 本地文件落盘。
