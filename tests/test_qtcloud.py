"""场景 C：量潮云（PaaS 按量计费，预存余额多次小额消费）——TC-C01..C03。

对齐 tests.md 业务场景 C：充值预存 → 按量多次消费 → 余额连续扣减至耗尽 → 账本核对。
"""

from __future__ import annotations

from tests.api import ApiClient, unique


def test_c01_recharge_prepaid(api: ApiClient) -> None:
    """TC-C01 开户与预存充值。"""
    acc = api.create_account(unique("cloud"))
    api.recharge(acc, 10000, unique("Y-001"))

    api.assert_ledger(acc, 10000)
    assert api.count_type(acc, "recharge") == 1


def test_c02_issue_all_scope_voucher(api: ApiClient) -> None:
    """TC-C02 发放全场代金券：批次幂等。"""
    acc = api.create_account(unique("cloud"))
    batch_no = unique("Y-V-001")
    api.issue_voucher(acc, amount=2000, scope="all", batch_no=batch_no)
    api.issue_voucher(acc, amount=2000, scope="all", batch_no=batch_no)  # 重复

    assert api.get_vouchers(acc)[0]["status"] == "issued"
    assert api.count_type(acc, "issue") == 1


def test_c03_metered_consumption(api: ApiClient) -> None:
    """TC-C03 按量多次消费：余额连续扣减；代金券全额抵现时余额不扣。"""
    acc = api.create_account(unique("cloud"))
    api.recharge(acc, 10000, unique("Y-001"))

    # 第一次：3000，余额 7000（此时尚无代金券，纯余额）
    api.settle(order_id=unique("O-Y-1"), account_id=acc, scope="cloud", amount=3000)
    api.assert_ledger(acc, 7000)

    # 第二次前发放代金券：2000 由代金券全额抵现，余额不扣
    api.issue_voucher(acc, amount=2000, scope="all", batch_no=unique("Y-V-001"))
    o2 = api.settle(order_id=unique("O-Y-2"), account_id=acc, scope="cloud", amount=2000)
    api.assert_detail(o2, [("voucher", 2000)])
    api.assert_ledger(acc, 7000)

    # 第三次：5000，余额 2000
    api.settle(order_id=unique("O-Y-3"), account_id=acc, scope="cloud", amount=5000)
    api.assert_ledger(acc, 2000)

    # 账单运行余额连续正确：充值 10000 → 消费 3000 → 发券（不变）→ 核销（不变）→ 消费 5000
    stmt = api.statement(acc)
    entries = stmt["entries"]
    assert len(entries) == 5, f"entries = {len(entries)}, want 5"
    assert [e["running_balance"] for e in entries] == [10000, 7000, 7000, 7000, 2000]
    # 代金券抵现应有一条核销交易
    assert api.count_type(acc, "redeem") == 1
