"""API 集成测试：验证支付 API HTTP 服务功能。

通过运行 Go provider 包中 API 相关的测试用例，
覆盖 Pay / Query / Refund 三个端点的正常及异常场景。
"""

import subprocess
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]


def test_api_http_endpoints_pass():
    """运行 Go provider 包内的 API 测试用例。

    测试覆盖：
      - POST /pay           创建支付（正常、请求体错误、Provider 错误、错误方法）
      - GET  /query/{id}    查询订单（正常、空 id、未找到、错误方法）
      - POST /refund        申请退款（正常、Provider 错误、请求体错误、invalid JSON、错误方法）
    """
    provider_dir = PROJECT_ROOT / "src" / "provider"
    # 只跑 API 相关的测试，避免混入其他 provider 测试
    result = subprocess.run(
        ["go", "test", "-run", "TestAPI_", "./...", "-v"],
        cwd=str(provider_dir),
        capture_output=True,
        text=True,
        timeout=60,
    )
    print(result.stdout)
    if result.stderr:
        print(result.stderr, file=sys.stderr)

    assert result.returncode == 0, (
        f"API tests failed (exit code {result.returncode})\n"
        f"stdout:\n{result.stdout}\n"
        f"stderr:\n{result.stderr}"
    )

    passed_count = result.stdout.count("--- PASS:")
    failed_count = result.stdout.count("--- FAIL:")
    assert failed_count == 0, f"{failed_count} API test(s) failed"
    assert passed_count >= 13, f"expected >=13 passed API tests, got {passed_count}"
