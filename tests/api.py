"""provider 服务端 HTTP 客户端：传输层 + 领域操作 + 账本断言。

纯标准库实现，无第三方依赖。测试通过 conftest 的 `api` fixture 获取实例。
"""

from __future__ import annotations

import json
import time
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone
from typing import Any


def future_expiry(seconds: int = 24 * 3600) -> str:
    """N 秒后的过期时间（RFC3339，带时区）。"""
    return (datetime.now(timezone.utc) + timedelta(seconds=seconds)).isoformat()


def unique(prefix: str) -> str:
    """生成全局唯一标识（订单号/凭证号/批次号共用，避免测试间幂等键冲突）。"""
    return f"{prefix}-{time.time_ns()}"


def _money(cents: int) -> dict[str, Any]:
    """分 → 结构化金额对象（API 传输格式：整数分 + CNY）。"""
    return {"amount": cents, "currency": "CNY"}


def _cents(v: Any) -> int:
    """解析 API 响应中的结构化金额对象为分。"""
    return int(v["amount"])


class ApiClient:
    """基于标准库的 JSON HTTP 客户端 + 账本核心领域辅助。"""

    def __init__(self, base_url: str) -> None:
        self.base_url = base_url.rstrip("/")

    # --- 传输层 ---

    def request(self, method: str, path: str, body: Any = None) -> tuple[int, Any]:
        data = None
        headers = {}
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(
            self.base_url + path, data=data, headers=headers, method=method
        )
        try:
            with urllib.request.urlopen(req, timeout=10) as resp:
                payload = resp.read()
                return resp.status, json.loads(payload) if payload else None
        except urllib.error.HTTPError as exc:
            payload = exc.read()
            try:
                parsed = json.loads(payload) if payload else None
            except json.JSONDecodeError:
                parsed = payload.decode("utf-8", errors="replace")
            return exc.code, parsed

    def get(self, path: str) -> tuple[int, Any]:
        return self.request("GET", path)

    def post(self, path: str, body: Any = None) -> tuple[int, Any]:
        return self.request("POST", path, body)

    # --- 领域操作（走真实 HTTP API，失败即抛错） ---

    def create_account(self, customer_id: str) -> str:
        status, body = self.post("/accounts", {"customer_id": customer_id})
        assert status == 201, f"create_account: {status} {body}"
        return body["id"]

    def recharge(self, account_id: str, amount: int, voucher_no: str) -> None:
        status, body = self.post(
            f"/accounts/{account_id}/recharges",
            {"amount": _money(amount), "voucher_no": voucher_no},
        )
        assert status == 200, f"recharge: {status} {body}"

    def refund(self, account_id: str, amount: int, voucher_no: str) -> None:
        status, body = self.post(
            f"/accounts/{account_id}/refunds",
            {"amount": _money(amount), "voucher_no": voucher_no},
        )
        assert status == 200, f"refund: {status} {body}"

    def issue_coupon(
        self,
        account_id: str,
        *,
        coupon_type: str = "discount",
        rate: int | None = None,
        threshold: int | None = None,
        amount: int | None = None,
        scope: str = "all",
        product_id: str | None = None,
        expires_at: str | None = None,
        count: int = 1,
        batch_no: str | None = None,
    ) -> None:
        body: dict[str, Any] = {
            "type": coupon_type,
            "scope": scope,
            "expires_at": expires_at or future_expiry(),
            "count": count,
            "batch_no": batch_no or unique("B"),
        }
        if rate is not None:
            body["rate"] = rate
        if threshold is not None:
            body["threshold"] = _money(threshold)
        if amount is not None:
            body["amount"] = _money(amount)
        if product_id is not None:
            body["product_id"] = product_id
        status, resp = self.post(f"/accounts/{account_id}/coupons", body)
        assert status == 200, f"issue_coupon: {status} {resp}"

    def issue_voucher(
        self,
        account_id: str,
        *,
        amount: int,
        scope: str = "all",
        product_id: str | None = None,
        expires_at: str | None = None,
        count: int = 1,
        batch_no: str | None = None,
    ) -> None:
        body: dict[str, Any] = {
            "amount": _money(amount),
            "scope": scope,
            "expires_at": expires_at or future_expiry(),
            "count": count,
            "batch_no": batch_no or unique("V"),
        }
        if product_id is not None:
            body["product_id"] = product_id
        status, resp = self.post(f"/accounts/{account_id}/vouchers", body)
        assert status == 200, f"issue_voucher: {status} {resp}"

    def settle(
        self,
        *,
        order_id: str,
        account_id: str,
        scope: str,
        amount: int,
        product_id: str | None = None,
        customer_id: str | None = None,
    ) -> dict[str, Any]:
        body: dict[str, Any] = {
            "order_id": order_id,
            "account_id": account_id,
            "scope": scope,
            "amount": _money(amount),
        }
        if product_id is not None:
            body["product_id"] = product_id
        if customer_id is not None:
            body["customer_id"] = customer_id
        status, resp = self.post("/orders", body)
        assert status == 201, f"settle: {status} {resp}"
        return resp

    # --- 查询 ---

    def get_account(self, account_id: str) -> dict[str, Any]:
        status, body = self.get(f"/accounts/{account_id}")
        assert status == 200, f"get_account: {status} {body}"
        return body

    def balance(self, account_id: str) -> int:
        return _cents(self.get_account(account_id)["balance"])

    def get_coupons(self, account_id: str) -> list[dict[str, Any]]:
        status, body = self.get(f"/accounts/{account_id}/coupons")
        assert status == 200, f"get_coupons: {status} {body}"
        return body["coupons"]

    def get_vouchers(self, account_id: str) -> list[dict[str, Any]]:
        status, body = self.get(f"/accounts/{account_id}/vouchers")
        assert status == 200, f"get_vouchers: {status} {body}"
        return body["vouchers"]

    def get_transactions(self, account_id: str) -> list[dict[str, Any]]:
        status, body = self.get(f"/accounts/{account_id}/transactions?limit=100")
        assert status == 200, f"get_transactions: {status} {body}"
        # 金额统一归一为分（API 传输为元）
        return [
            {
                **tx,
                "amount": _cents(tx["amount"]),
                "balance_after": _cents(tx["balance_after"]),
            }
            for tx in body["transactions"]
        ]

    def statement(self, account_id: str) -> dict[str, Any]:
        status, body = self.get(f"/accounts/{account_id}/statement")
        assert status == 200, f"statement: {status} {body}"
        # 金额统一归一为分（API 传输为元）
        body["opening_balance"] = _cents(body["opening_balance"])
        body["closing_balance"] = _cents(body["closing_balance"])
        for e in body["entries"]:
            e["amount"] = _cents(e["amount"])
            e["running_balance"] = _cents(e["running_balance"])
        return body

    def consistency(self) -> list[dict[str, Any]]:
        status, body = self.get("/reconcile/consistency")
        assert status == 200, f"consistency: {status} {body}"
        return body["discrepancies"]

    # --- 断言辅助 ---

    def assert_ledger(self, account_id: str, want_balance: int) -> None:
        """核对余额正确且与交易一致（不错）。"""
        balance = self.balance(account_id)
        assert balance == want_balance, f"balance = {balance}, want {want_balance}"
        for d in self.consistency():
            assert d["account_id"] != account_id, f"一致性差异: {d}"

    def assert_detail(
        self, order: dict[str, Any], want: list[tuple[str, int]]
    ) -> None:
        """断言结算明细的 (类型, 金额) 序列（金额已归一为分）。"""
        got = [(d["kind"], _cents(d["amount"])) for d in order["settle_detail"]]
        assert got == want, f"detail = {got}, want {want}"

    def count_type(self, account_id: str, typ: str) -> int:
        return sum(1 for tx in self.get_transactions(account_id) if tx["type"] == typ)

    def net_flow(self, account_id: str) -> int:
        """账户净变动：Σ(充值) − Σ(余额支付) − Σ(退款)。"""
        flow = 0
        for tx in self.get_transactions(account_id):
            if tx["type"] == "recharge":
                flow += tx["amount"]
            elif tx["type"] in ("consume", "refund"):
                flow -= tx["amount"]
        return flow
