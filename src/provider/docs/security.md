# 安全与权限

## 分层

支付服务安全分三层：

1. API 网关 JWT 鉴权：入口层校验 qtcloud-auth 签发的 JWT。
2. 服务端应用层 SECRET_KEY：支付服务自行签发和校验服务端凭据，账号系统异常时仍 fail-closed。
3. 支付域权限表：按用户、角色、权限校验每个业务端点。

`GET /health` 是公开探活端点；其他账本、渠道、对账、规则和权限管理端点均受权限中间件保护。匿名请求返回 401，已认证但无权限返回 403。`X-Admin-Token` 保留为系统级超管通道，用于紧急运维和权限初始化；生产必须通过 secret 注入，缺失时不会打开对应能力。

## 配置

| 变量 | 必填 | 用途 |
|------|------|------|
| `SECRET_KEY` | 是 | 应用级密钥；缺失时启动失败 |
| `ADMIN_TOKEN` | 否 | 系统级超管通道；为空时不能旁路权限 |
| `AUTH_JWT_PUBLIC_JWK` | 否 | qtcloud-auth 的 RS256 Public JWK/JWKS |
| `AUTH_JWT_PUBLIC_PEM` | 否 | qtcloud-auth 的 PEM 公钥，本地兼容入口 |
| `AUTH_JWT_ISSUER` | 否 | 校验 `iss` |
| `AUTH_JWT_AUDIENCE` | 否 | 校验 `aud` |

`SECRET_KEY` 必须使用强随机值并放在 GitHub 仓库级 secret；密码本单独表格记录：系统 `qtcloud-pay`、用途 `provider SECRET_KEY`、存储位置 `GitHub secret SECRET_KEY`。如执行人无密码本权限，汇报标注“待黎想登记”。

## 角色矩阵

| 角色 | 权限 |
|------|------|
| `admin` | `*:*` |
| `operator` | 账户读写、发券、结算、对账读写、规则读写、渠道支付/查询/退款；无删除、无权限管理 |
| `viewer` | 账户、订单、对账、规则只读，以及渠道查询 |

权限表随 `app.Open` 自动迁移并初始化默认角色。用户标识使用 qtcloud-auth JWT 的 `sub`，也兼容 `user_id` / `customer_id`；服务端凭据使用 `SECRET_KEY` 签发，`sub` 同样作为权限主体。

## 管理 API

以下端点需要 `security:admin` 或 `X-Admin-Token`：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/security/permissions` | 查看角色与权限清单 |
| GET | `/admin/security/users/{user_id}/roles` | 查询用户角色 |
| PUT | `/admin/security/users/{user_id}/roles` | 分配用户角色，body: `{"roles":["operator"]}` |
| POST | `/admin/security/service-tokens` | 签发短期服务端凭据，body: `{"subject":"svc_x","ttl_seconds":3600}` |

## 网关 JWT

传统版 API Gateway JWT 认证插件配置要点：

- `parameter: Authorization`、`parameterLocation: header`，支持 `Authorization: Bearer {token}`。
- 配置 qtcloud-auth 的 Public JWK/JWKS；插件用公钥校验签名，私钥只留在签发方。
- `exp` 必填且有效期小于 7 天； claims 不放敏感数据。
- 插件绑定充值、退款、发券、查账、账户、结算、规则等支付 API。
- 插件只能认证；细粒度授权由本服务 `internal/security` 完成。

官方文档列出的网关错误语义包含 JWT 缺失 400、无匹配 JWK/过期/无效 403。对外验收若要求“匿名 401”，以支付服务 FC 直连结果为准；网关侧可通过响应映射统一成 401。

## 推广边界

`internal/security` 当前保持在支付服务内，后续稳定后抽象到 `quanttide.authentication`。其他服务端套用时只需要：

1. 启动强制读取 `SECRET_KEY`。
2. 将业务 mux 包在权限中间件后。
3. 为本域定义资源-动作权限矩阵和默认角色。
4. 复用 qtcloud-auth JWT 公钥，同时保留 SECRET_KEY 服务端凭据作为兜底身份源。

资产云不再推进独立账号系统路线；按同一 SECRET_KEY + 网关鉴权 + 本域权限模型接入。
