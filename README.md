# qtcloud-pay

量潮支付云 — 量潮知识管理体系中的支付云服务平台。

## 概述

qtcloud-pay 是量潮支付领域的云服务平台，提供支付网关、账务处理、对账结算等核心支付能力的云上部署与托管服务。

## 测试

```sh
# provider 服务端：Go 单元 + 集成测试
cd src/provider && make test

# Python 端到端测试（编译真实二进制 + HTTP API，对齐 data/roadmap/tests.md）
uv run pytest tests/
```

## 许可

Apache 2.0
