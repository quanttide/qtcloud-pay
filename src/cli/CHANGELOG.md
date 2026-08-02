# Changelog

## 0.1.0（未发布）

- 初始化 Rust 工程：命令树 accounts / recharges / coupons / vouchers / orders / reconcile / config / milestone / completions
- 核心模块：money（元↔分，整数分无浮点误差）、config（参数 > 环境变量 > 配置文件）、退出码约定（0/1/2/3）
- 里程碑验收：`milestone verify M1` 自动执行 M1 验收（创建账户 → 充值 → 幂等拦截 → 余额校验）
- 测试：金额转换、余额一致性、银行流水 CSV 比对
