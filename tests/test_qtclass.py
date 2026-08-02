"""场景 A：量潮课堂（高校学生付费学习，几十~几百元）——TC-A01..A09。

对齐 tests.md 业务场景 A：付费（对公打款）→ 记入额度 → 按学习扣费 → 按交付发代金券 → 账本核对。
"""

from __future__ import annotations

import time

from tests.api import ApiClient, future_expiry, unique


def test_a01_recharge_idempotent(api: ApiClient) -> None:
    """TC-A01 开户与付费记额度：重复提交同凭证号不重复入账（不重）。"""
    acc = api.create_account(unique("stu"))
    voucher_no = unique("GT-001")
    api.recharge(acc, 20000, voucher_no)
    api.recharge(acc, 20000, voucher_no)  # 重复提交同凭证号

    api.assert_ledger(acc, 20000)
    txs = api.get_transactions(acc)
    assert len(txs) == 1, f"txs = {len(txs)}, want 1"
    tx = txs[0]
    assert tx["type"] == "recharge"
    assert tx["amount"] == 20000
    assert tx["balance_after"] == 20000


def test_a02_issue_incentives(api: ApiClient) -> None:
    """TC-A02 按交付发放激励：批量 + 幂等 + 发券交易入账（不丢、不重）。"""
    acc = api.create_account(unique("stu"))
    batch_no = unique("GT-B-001")
    api.issue_coupon(
        acc, coupon_type="discount", rate=90, scope="course", count=10, batch_no=batch_no
    )
    api.issue_coupon(
        acc, coupon_type="discount", rate=90, scope="course", count=10, batch_no=batch_no
    )  # 重复提交同批次
    voucher_batch = unique("GT-V-001")
    api.issue_voucher(acc, amount=2000, scope="all", batch_no=voucher_batch)
    api.issue_voucher(acc, amount=2000, scope="all", batch_no=voucher_batch)  # 重复

    coupons = api.get_coupons(acc)
    assert len(coupons) == 10, f"coupons = {len(coupons)}, want 10"
    assert all(c["status"] == "issued" for c in coupons)
    assert len(api.get_vouchers(acc)) == 1
    # 两个批次各 1 条发券交易，重复提交不新增
    assert api.count_type(acc, "issue") == 2


def test_a03_learning_deduction(api: ApiClient) -> None:
    """TC-A03 按学习扣费：付费记额度后，费用随学习进度逐次扣除。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 20000, unique("GT-001"))  # 付费 20000 记入额度

    # 学第一节课扣 10000
    o1 = api.settle(
        order_id=unique("O-GT-1"), account_id=acc, scope="course", amount=10000
    )
    api.assert_detail(o1, [("balance", 10000)])
    api.assert_ledger(acc, 10000)

    # 学第二节课再扣 10000
    o2 = api.settle(
        order_id=unique("O-GT-2"), account_id=acc, scope="course", amount=10000
    )
    api.assert_detail(o2, [("balance", 10000)])
    api.assert_ledger(acc, 0)

    # 两笔消费交易 running balance 连续（10000 → 0），余额 = 交易求和
    # 接口按 id 倒序返回（最新在前），断言前按 id 翻转为时间正序
    txs = [tx for tx in api.get_transactions(acc) if tx["type"] == "consume"]
    txs.sort(key=lambda tx: tx["id"])
    assert [tx["balance_after"] for tx in txs] == [10000, 0], f"txs = {txs}"


def test_a04_full_reduction_threshold(api: ApiClient) -> None:
    """TC-A04 满减券门槛（时机）：不达标不抵扣、达标抵扣并核销。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 30000, unique("GT-001"))
    api.issue_coupon(
        acc,
        coupon_type="full_reduction",
        threshold=10000,
        amount=2000,
        scope="course",
        batch_no=unique("GT-B-001"),
    )

    # 不达标：8000 < 10000
    o1 = api.settle(order_id=unique("O-GT-1"), account_id=acc, scope="course", amount=8000)
    api.assert_detail(o1, [("balance", 8000)])
    assert api.get_coupons(acc)[0]["status"] == "issued"

    # 达标：10000
    o2 = api.settle(order_id=unique("O-GT-2"), account_id=acc, scope="course", amount=10000)
    api.assert_detail(o2, [("coupon", 2000), ("balance", 8000)])
    assert api.get_coupons(acc)[0]["status"] == "used"
    api.assert_ledger(acc, 30000 - 8000 - 8000)


def test_a05_voucher_before_balance(api: ApiClient) -> None:
    """TC-A05 代金券优先于余额。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 10000, unique("GT-001"))
    api.issue_voucher(acc, amount=2000, scope="all", batch_no=unique("GT-V-001"))

    order = api.settle(order_id=unique("O-GT-1"), account_id=acc, scope="course", amount=10000)
    api.assert_detail(order, [("voucher", 2000), ("balance", 8000)])
    # 余额：10000 − 8000 = 2000（tests.md 中「余额 8000」为笔误）
    api.assert_ledger(acc, 2000)


def test_a06_mixed_settlement(api: ApiClient) -> None:
    """TC-A06 混合结算顺序：券 → 代金券 → 余额；账本逐笔对得上。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 10000, unique("GT-001"))
    api.issue_coupon(
        acc, coupon_type="discount", rate=90, scope="course", batch_no=unique("GT-B-001")
    )
    api.issue_voucher(acc, amount=2000, scope="all", batch_no=unique("GT-V-001"))

    order = api.settle(order_id=unique("O-GT-1"), account_id=acc, scope="course", amount=10000)
    # 9 折省 1000 → 代金券 2000 → 余额 7000
    api.assert_detail(order, [("coupon", 1000), ("voucher", 2000), ("balance", 7000)])
    api.assert_ledger(acc, 3000)
    assert api.count_type(acc, "redeem") == 2
    assert api.count_type(acc, "consume") == 1


def test_a07_insufficient_balance_rollback(api: ApiClient) -> None:
    """TC-A07 余额不足整体回滚：无订单、券未核销、余额不变、无交易写入。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 5000, unique("GT-001"))
    api.issue_coupon(
        acc, coupon_type="discount", rate=90, scope="course", batch_no=unique("GT-B-001")
    )
    api.issue_voucher(acc, amount=2000, scope="all", batch_no=unique("GT-V-001"))

    order_id = unique("O-GT-1")
    status, _ = api.post(
        "/orders",
        {
            "order_id": order_id,
            "account_id": acc,
            "scope": "course",
            "amount": 10000,
        },
    )
    assert status == 422, f"status = {status}, want 422"

    # 订单不存在；券均未核销；余额不变；无消费/核销交易（回滚干净）
    status, _ = api.get(f"/orders/{order_id}")
    assert status == 404
    assert all(c["status"] == "issued" for c in api.get_coupons(acc))
    assert all(v["status"] == "issued" for v in api.get_vouchers(acc))
    api.assert_ledger(acc, 5000)
    assert api.count_type(acc, "consume") + api.count_type(acc, "redeem") == 0


def test_a08_expired_coupon(api: ApiClient) -> None:
    """TC-A08 过期课程券不可用：扣费明细无券项，惰性流转为 expired。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 10000, unique("GT-001"))
    # 短有效期券，等待其过期（Python 侧无数据库句柄，无法回拨时间）
    api.issue_coupon(
        acc,
        coupon_type="discount",
        rate=90,
        scope="course",
        expires_at=future_expiry(seconds=2),
        batch_no=unique("GT-B-001"),
    )
    time.sleep(3)

    order = api.settle(order_id=unique("O-GT-1"), account_id=acc, scope="course", amount=10000)
    api.assert_detail(order, [("balance", 10000)])
    assert api.get_coupons(acc)[0]["status"] == "expired"


def test_a09_order_idempotent(api: ApiClient) -> None:
    """TC-A09 扣费幂等：重复提交返回同一订单，余额只扣一次、券只核销一次（不重）。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 20000, unique("GT-001"))
    api.issue_coupon(
        acc, coupon_type="discount", rate=90, scope="course", batch_no=unique("GT-B-001")
    )

    body = {
        "order_id": unique("O-GT-1"),
        "account_id": acc,
        "scope": "course",
        "amount": 10000,
    }
    o1 = api.settle(**body)
    o2 = api.settle(**body)  # 重复提交

    assert o1["id"] == o2["id"]
    # 9 折省 1000，余额支付 9000；只扣一次 → 20000 − 9000 = 11000
    api.assert_ledger(acc, 11000)
    assert api.count_type(acc, "consume") == 1
    assert api.get_coupons(acc)[0]["status"] == "used"
