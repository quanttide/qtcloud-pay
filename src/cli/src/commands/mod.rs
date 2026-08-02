pub mod accounts;
pub mod config;
pub mod coupons;
pub mod milestone;
pub mod orders;
pub mod recharges;
pub mod reconcile;
pub mod status;
pub mod vouchers;

use std::io::IsTerminal;

use clap::CommandFactory;
use colored::control;

use crate::api::Client;
use crate::args::{Cli, Command, CompletionsArgs};
use crate::config::Config;
use crate::error::CliError;

/// 命令上下文：HTTP 客户端 + 全局输出选项
pub struct Ctx {
    pub(crate) client: Client,
    pub(crate) server_url: String,
    pub(crate) json: bool,
    pub(crate) quiet: bool,
    pub(crate) color: bool,
}

pub async fn run(cli: Cli) -> Result<(), CliError> {
    let Cli { global, command } = cli;
    match command {
        Command::Completions(args) => completions(args),
        other => {
            let config = Config::load(global.server.as_deref())?;
            let client = Client::new(config.server_url.clone())
                .map_err(|e| CliError::Fatal(format!("初始化 HTTP 客户端失败：{e}")))?;
            let color = !global.no_color
                && std::io::stdout().is_terminal()
                && std::env::var("NO_COLOR").is_err();
            control::set_override(color);
            let ctx = Ctx {
                client,
                server_url: config.server_url,
                json: global.json,
                quiet: global.quiet,
                color,
            };
            dispatch(ctx, other).await
        }
    }
}

async fn dispatch(ctx: Ctx, command: Command) -> Result<(), CliError> {
    match command {
        Command::Status => status::run(&ctx).await,
        Command::Accounts(args) => accounts::run(&ctx, args).await,
        Command::Recharges(args) => recharges::run(&ctx, args).await,
        Command::Coupons(args) => coupons::run(&ctx, args).await,
        Command::Vouchers(args) => vouchers::run(&ctx, args).await,
        Command::Orders(args) => orders::run(&ctx, args).await,
        Command::Reconcile(args) => reconcile::run(&ctx, args).await,
        Command::Config(args) => config::run(&ctx, args).await,
        Command::Milestone(args) => milestone::run(&ctx, args).await,
        Command::Completions(_) => unreachable!("completions 已提前处理"),
    }
}

/// 生成 shell 补全：qtcloud-pay completions bash|zsh|fish|powershell|elvish
fn completions(args: CompletionsArgs) -> Result<(), CliError> {
    let mut cmd = Cli::command();
    clap_complete::generate(args.shell, &mut cmd, "qtcloud-pay", &mut std::io::stdout());
    Ok(())
}
