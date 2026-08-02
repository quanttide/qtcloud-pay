"""pytest 集成测试夹具：构建并启动 provider 服务端二进制，供测试通过 HTTP API 访问。

用法：

    def test_something(api):
        account_id = api.create_account("stu-1")
        ...
"""

from __future__ import annotations

import os
import socket
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path

import pytest

from .api import ApiClient

REPO_ROOT = Path(__file__).resolve().parents[1]  # apps/qtcloud-pay
PROVIDER_DIR = REPO_ROOT / "src" / "provider"
SERVER_ADDR = "127.0.0.1"
START_TIMEOUT = 30  # 服务启动超时（秒）


def _free_port() -> int:
    """获取一个空闲端口（存在微小竞态，测试场景可接受）。"""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind((SERVER_ADDR, 0))
        return s.getsockname()[1]


def _build_binary(dst: Path) -> None:
    """编译 provider 服务端二进制到指定位置。"""
    result = subprocess.run(
        ["go", "build", "-o", str(dst), "./cmd/server"],
        cwd=PROVIDER_DIR,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(f"go build 失败:\n{result.stdout}\n{result.stderr}")


def _wait_ready(base_url: str, timeout: float = START_TIMEOUT) -> None:
    """轮询健康端点（GET /reconcile/consistency）直至服务就绪。"""
    deadline = time.monotonic() + timeout
    last_error: Exception | None = None
    while time.monotonic() < deadline:
        try:
            with urllib.request.urlopen(base_url + "/reconcile/consistency", timeout=1) as resp:
                if resp.status == 200:
                    return
        except (urllib.error.URLError, OSError) as exc:
            last_error = exc
        time.sleep(0.1)
    raise TimeoutError(f"provider 服务在 {timeout}s 内未就绪: {base_url}（最后错误: {last_error}）")


@pytest.fixture(scope="session")
def server_url(tmp_path_factory: pytest.TempPathFactory) -> str:
    """构建并启动 provider 服务端，返回基础 URL；会话结束自动关闭。

    - 二进制编译到临时目录，不污染仓库
    - 使用独立临时 SQLite 库（DB_SQLITE_DSN）
    - 不挂载支付渠道（仅账本核心 API）
    """
    tmp = tmp_path_factory.mktemp("provider")
    binary = tmp / "provider-server"
    _build_binary(binary)

    port = _free_port()
    base_url = f"http://{SERVER_ADDR}:{port}"

    env = os.environ.copy()
    env["DB_DRIVER"] = "sqlite"
    env["DB_SQLITE_DSN"] = str(tmp / "test.db")

    proc = subprocess.Popen(
        [str(binary), "-addr", f"{SERVER_ADDR}:{port}"],
        cwd=PROVIDER_DIR,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        start_new_session=True,
    )
    try:
        _wait_ready(base_url)
    except Exception:
        proc.terminate()
        try:
            output, _ = proc.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
            output = ""
        raise RuntimeError(f"provider 启动失败，输出:\n{output}") from None
    yield base_url

    proc.terminate()
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=5)


@pytest.fixture(scope="session")
def api(server_url: str) -> ApiClient:
    """账本核心领域客户端（真实 HTTP API，见 tests/api.py）。"""
    return ApiClient(server_url)
