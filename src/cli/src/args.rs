use clap::{Args, Parser, Subcommand};
use clap_complete::Shell;

/// 量潮支付账本核心命令行工作台（qtcloud-pay CLI）。
///
/// 设计文档：data/roadmap/cli.md。CLI 只做展示与参数组装，账务逻辑（结算、幂等、状态机）全在服务端。
#[derive(Debug, Parser)]
#[command(name = "qtcloud-pay", version)]
pub struct Cli {
    #[command(flatten)]
    pub global: GlobalArgs,

    #[command(subcommand)]
    pub command: Command,
}

#[derive(Debug, Clone, Args)]
pub struct GlobalArgs {
    /// 账本核心 API 地址（优先级：参数 > QTPAY_SERVER_URL > ~/.config/qtcloud-pay/config.toml）
    #[arg(long, global = true)]
    pub server: Option<String>,

    /// JSON 输出（金额保持整数分，供脚本消费）
    #[arg(long, global = true)]
    pub json: bool,

    /// 只输出结果与错误
    #[arg(long, global = true)]
    pub quiet: bool,

    /// 禁用 ANSI 着色（非 TTY 自动禁用）
    #[arg(long, global = true)]
    pub no_color: bool,
}

#[derive(Debug, Subcommand)]
pub enum Command {
    /// 总览：服务端地址、里程碑状态与今日待办
    Status,

    /// 账户与余额
    Accounts(AccountsArgs),

    /// 充值登记（对公打款入账）
    Recharges(RechargesArgs),

    /// 优惠券发放与查询
    Coupons(CouponsArgs),

    /// 代金券发放与查询
    Vouchers(VouchersArgs),

    /// 订单结算与查询
    Orders(OrdersArgs),

    /// 对账：余额校验与银行流水比对
    Reconcile(ReconcileArgs),

    /// 参数配置（计费抵扣顺序）
    Config(ConfigArgs),

    /// 里程碑验收
    Milestone(MilestoneArgs),

    /// 生成 shell 补全
    Completions(CompletionsArgs),
}

#[derive(Debug, Args)]
pub struct AccountsArgs {
    #[command(subcommand)]
    pub action: AccountsAction,
}

#[derive(Debug, Subcommand)]
pub enum AccountsAction {
    /// 创建账户
    Create {
        /// 客户名
        #[arg(long)]
        name: String,
    },
    /// 查询账户与余额
    Get { account_id: String },
    /// 查询交易流水
    Transactions {
        account_id: String,
        /// 交易类型过滤：recharge/consume/issue/redeem（或 充值/消费/发券/核销）
        #[arg(long = "type")]
        type_: Option<String>,
    },
    /// 导出账单（CSV）
    Statement {
        account_id: String,
        /// 输出文件路径（缺省输出到 stdout）
        #[arg(short, long)]
        output: Option<String>,
    },
}

#[derive(Debug, Args)]
pub struct RechargesArgs {
    #[command(subcommand)]
    pub action: RechargesAction,
}

#[derive(Debug, Subcommand)]
pub enum RechargesAction {
    /// 充值登记（对公打款入账）
    Create {
        account_id: String,
        /// 金额（元，如 100.00，转整数分提交）
        #[arg(long)]
        amount: String,
        /// 打款凭证号（幂等键，必填）
        #[arg(long)]
        receipt_no: String,
    },
}

#[derive(Debug, Args)]
pub struct CouponsArgs {
    #[command(subcommand)]
    pub action: CouponsAction,
}

#[derive(Debug, Subcommand)]
pub enum CouponsAction {
    /// 发放优惠券（幂等键 = 发放批次号）
    Issue {
        account_id: String,
        /// 类型：rate 折扣券 / threshold 满减券
        #[arg(long)]
        kind: String,
        /// 折扣率（rate 时必填，如 0.9 = 9 折）
        #[arg(long)]
        rate: Option<f64>,
        /// 满减门槛（元，threshold 时必填）
        #[arg(long)]
        threshold: Option<String>,
        /// 满减金额（元，threshold 时必填）
        #[arg(long)]
        amount: Option<String>,
        /// 适用范围：全场 / 指定业务 / 指定商品
        #[arg(long)]
        scope: Option<String>,
        /// 有效期（YYYY-MM-DD）
        #[arg(long)]
        expires_at: Option<String>,
        /// 发放批次号（幂等键，必填）
        #[arg(long)]
        batch_no: String,
    },
    /// 查询优惠券
    List { account_id: String },
}

#[derive(Debug, Args)]
pub struct VouchersArgs {
    #[command(subcommand)]
    pub action: VouchersAction,
}

#[derive(Debug, Subcommand)]
pub enum VouchersAction {
    /// 发放代金券（幂等键 = 发放批次号）
    Issue {
        account_id: String,
        /// 面值（元）
        #[arg(long)]
        amount: String,
        /// 适用范围：全场 / 指定业务 / 指定商品
        #[arg(long)]
        scope: Option<String>,
        /// 有效期（YYYY-MM-DD）
        #[arg(long)]
        expires_at: Option<String>,
        /// 发放批次号（幂等键，必填）
        #[arg(long)]
        batch_no: String,
    },
    /// 查询代金券
    List { account_id: String },
}

#[derive(Debug, Args)]
pub struct OrdersArgs {
    #[command(subcommand)]
    pub action: OrdersAction,
}

#[derive(Debug, Subcommand)]
pub enum OrdersAction {
    /// 下单并结算（幂等键 = 订单号）
    Create {
        /// 账户 id
        #[arg(long)]
        account: String,
        /// 订单金额（元）
        #[arg(long)]
        amount: String,
        /// 订单号（幂等键，必填）
        #[arg(long)]
        order_no: String,
        /// 商品/业务说明
        #[arg(long)]
        subject: Option<String>,
    },
    /// 查询订单与结算明细
    Get { order_id: String },
}

#[derive(Debug, Args)]
pub struct ReconcileArgs {
    /// 待校验账户 id（可多个）
    #[arg(required = true)]
    pub accounts: Vec<String>,

    /// 银行流水 CSV（对公打款核对，默认列 date/amount/remark）
    #[arg(long)]
    pub bank: Option<String>,

    /// 银行流水列名映射，如 date=交易日期,amount=金额,remark=备注
    #[arg(long)]
    pub bank_cols: Option<String>,
}

#[derive(Debug, Args)]
pub struct ConfigArgs {
    #[command(subcommand)]
    pub action: ConfigAction,
}

#[derive(Debug, Subcommand)]
pub enum ConfigAction {
    /// 计费抵扣顺序（BillingRule.priority）
    #[command(subcommand)]
    BillingRule(BillingRuleAction),
}

#[derive(Debug, Subcommand)]
pub enum BillingRuleAction {
    /// 展示默认抵扣顺序
    Show,
    /// 设置抵扣顺序（预留：服务端参数 API 就绪后启用）
    Set {
        /// 优先级，如 coupon:voucher:balance
        #[arg(long)]
        priority: String,
    },
}

#[derive(Debug, Args)]
pub struct MilestoneArgs {
    #[command(subcommand)]
    pub action: MilestoneAction,
}

#[derive(Debug, Subcommand)]
pub enum MilestoneAction {
    /// 里程碑列表（本地验收登记）
    List,
    /// 验收打勾并记录日期（写入 ~/.config/qtcloud-pay/state.toml）
    Check {
        milestone: String,
        /// 验收结论备注
        #[arg(long)]
        note: Option<String>,
    },
    /// 自动执行验收（当前支持 M1）
    Verify { milestone: String },
}

#[derive(Debug, Args)]
pub struct CompletionsArgs {
    /// shell 类型
    #[arg(value_enum)]
    pub shell: Shell,
}
