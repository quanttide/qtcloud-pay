use std::process::ExitCode;

use clap::Parser;

use qtcloud_pay_cli::args::Cli;
use qtcloud_pay_cli::commands;

#[tokio::main]
async fn main() -> ExitCode {
    let cli = Cli::parse();
    match commands::run(cli).await {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            eprintln!("✘ {e}");
            ExitCode::from(e.exit_code())
        }
    }
}
