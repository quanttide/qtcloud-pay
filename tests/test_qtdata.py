"""场景 B：量潮数据（高校老师购买数据服务，几千元）——TC-B01..B08。

对齐 tests.md 业务场景 B：付费（预收对公打款）→ 记入额度 → 按交付进度/数据量
弹性扣费 → 多退少补（退款登记 / 补款充值）→ 账本核对。
"""

from __future__ import annotations

from tests.api import ApiClient, _money, unique


def test_b01_recharge_large(api: ApiClient) -> None:
    """TC-B01 开户与付费记额度：预收入账，余额 = 交易求和；重复提交不重复入账。"""
    acc = api.create_account(unique("teacher"))
    voucher_no = unique("SJ-001")
    api.recharge(acc, 800000, voucher_no)
    api.recharge(acc, 800000, voucher_no)  # 重复：同凭证号

    api.assert_ledger(acc, 800000)
    assert api.count_type(acc, "recharge") == 1


def test_b02_issue_data_incentives(api: ApiClient) -> None:
    """TC-B02 发放数据服务激励券：满减券 + 代金券，批次幂等。"""
    acc = api.create_account(unique("teacher"))
    coupon_batch = unique("SJ-B-001")
    api.issue_coupon(
        acc,
        coupon_type="full_reduction",
        threshold=500000,
        amount=100000,
        scope="data",
        batch_no=coupon_batch,
    )
    api.issue_coupon(
        acc,
        coupon_type="full_reduction",
        threshold=500000,
        amount=100000,
        scope="data",
        batch_no=coupon_batch,
    )  # 重复
    voucher_batch = unique("SJ-V-001")
    api.issue_voucher(acc, amount=50000, scope="all", batch_no=voucher_batch)
    api.issue_voucher(acc, amount=50000, scope="all", batch_no=voucher_batch)  # 重复

    assert api.get_coupons(acc)[0]["status"] == "issued"
    assert api.get_vouchers(acc)[0]["status"] == "issued"
    assert api.count_type(acc, "issue") == 2


def test_b03_progress_based_billing(api: ApiClient) -> None:
    """TC-B03 按交付进度扣费：分期结算 + 数据量弹性计费，券 → 代金券 → 余额逐笔对得上。"""
    acc = api.create_account(unique("teacher"))
    api.recharge(acc, 800000, unique("SJ-001"))
    api.issue_coupon(
        acc,
        coupon_type="full_reduction",
        threshold=500000,
        amount=100000,
        scope="data",
        batch_no=unique("SJ-B-001"),
    )
    api.issue_voucher(acc, amount=50000, scope="all", batch_no=unique("SJ-V-001"))

    # 第一期：按交付进度 500000（满减 100000 + 代金券 50000 + 余额 350000）
    o1 = api.settle(
        order_id=unique("O-SJ-1"), account_id=acc, scope="data", amount=500000
    )
    api.assert_detail(
        o1, [("coupon", 100000), ("voucher", 50000), ("balance", 350000)]
    )
    api.assert_ledger(acc, 450000)

    # 第二期：按实际数据量 300000（纯余额，弹性计费）
    o2 = api.settle(
        order_id=unique("O-SJ-2"), account_id=acc, scope="data", amount=300000
    )
    api.assert_detail(o2, [("balance", 300000)])
    api.assert_ledger(acc, 150000)

    # 账本：2 条消费 + 2 条核销，逐笔对得上
    assert api.count_type(acc, "consume") == 2
    assert api.count_type(acc, "redeem") == 2


def test_b04_refund_and_recharge(api: ApiClient) -> None:
    """TC-B04 多退少补：按实际用量结算后，多退（退款登记，幂等）少补（补款充值）。"""
    acc = api.create_account(unique("teacher"))
    api.recharge(acc, 800000, unique("SJ-001"))  # 按预估合同额预收

    # 实际用量 700000 → 多退 100000 回原路
    api.settle(order_id=unique("O-SJ-1"), account_id=acc, scope="data", amount=700000)
    api.assert_ledger(acc, 100000)

    refund_no = unique("SJ-R-001")
    api.refund(acc, 100000, refund_no)
    api.refund(acc, 100000, refund_no)  # 重复：同凭证号不重复退
    api.assert_ledger(acc, 0)
    assert api.count_type(acc, "refund") == 1

    # 少补：实际用量超出预存 → 余额不足整体回滚 → 补款后再结算
    order_id = unique("O-SJ-2")
    status, _ = api.post(
        "/orders",
        {"order_id": order_id, "account_id": acc, "scope": "data", "amount": _money(300000)},
    )
    assert status == 422, f"status = {status}, want 422"
    api.assert_ledger(acc, 0)  # 回滚干净

    api.recharge(acc, 300000, unique("SJ-002"))  # 少补
    api.settle(order_id=order_id, account_id=acc, scope="data", amount=300000)
    api.assert_ledger(acc, 0)


def test_b05_pick_best_full_reduction(api: ApiClient) -> None:
    """TC-B05 多满减券选力度最大（减额最大）。"""
    acc = api.create_account(unique("teacher"))
    api.recharge(acc, 800000, unique("SJ-001"))
    api.issue_coupon(
        acc,
        coupon_type="full_reduction",
        threshold=500000,
        amount=100000,
        scope="data",
        batch_no=unique("SJ-B-001"),
    )
    api.issue_coupon(
        acc,
        coupon_type="full_reduction",
        threshold=800000,
        amount=200000,
        scope="data",
        batch_no=unique("SJ-B-002"),
    )

    order = api.settle(
        order_id=unique("O-SJ-1"), account_id=acc, scope="data", amount=800000
    )
    # 核销满 800000 减 200000 的券（力度最大）
    api.assert_detail(order, [("coupon", 200000), ("balance", 600000)])
    statuses = [c["status"] for c in api.get_coupons(acc)]
    assert sorted(statuses) == ["issued", "used"], f"statuses = {statuses}"


def test_b06_scope_isolation(api: ApiClient) -> None:
    """TC-B06 课程券不能用于数据订单（跨业务隔离）。"""
    acc = api.create_account(unique("teacher"))
    api.recharge(acc, 200000, unique("SJ-001"))
    api.issue_coupon(
        acc,
        coupon_type="discount",
        rate=90,
        scope="course",
        batch_no=unique("SJ-B-001"),
    )

    order = api.settle(
        order_id=unique("O-SJ-1"), account_id=acc, scope="data", amount=100000
    )
    # 课程券不参与数据订单
    api.assert_detail(order, [("balance", 100000)])
    assert api.get_coupons(acc)[0]["status"] == "issued"


def test_b07_pick_best_discount(api: ApiClient) -> None:
    """TC-B07 多折扣券选力度最大（rate 最低，省得最多）。"""
    acc = api.create_account(unique("teacher"))
    api.recharge(acc, 100000, unique("SJ-001"))
    api.issue_coupon(
        acc, coupon_type="discount", rate=90, scope="data", batch_no=unique("SJ-B-001")
    )
    api.issue_coupon(
        acc, coupon_type="discount", rate=80, scope="data", batch_no=unique("SJ-B-002")
    )

    order = api.settle(
        order_id=unique("O-SJ-1"), account_id=acc, scope="data", amount=100000
    )
    # 8 折：省 20000，力度最大
    api.assert_detail(order, [("coupon", 20000), ("balance", 80000)])
    statuses = [c["status"] for c in api.get_coupons(acc)]
    assert sorted(statuses) == ["issued", "used"], f"statuses = {statuses}"


def test_b08_voucher_no_change(api: ApiClient) -> None:
    """TC-B08 代金券面值大于剩余应付：只抵应付，不找零（力度约定）。"""
    acc = api.create_account(unique("teacher"))
    api.recharge(acc, 30000, unique("SJ-001"))
    api.issue_voucher(acc, amount=50000, scope="all", batch_no=unique("SJ-V-001"))

    order = api.settle(
        order_id=unique("O-SJ-1"), account_id=acc, scope="data", amount=30000
    )
    api.assert_detail(order, [("voucher", 30000)])
    # 余额分文未动
    api.assert_ledger(acc, 30000)
    assert api.get_vouchers(acc)[0]["status"] == "used"
