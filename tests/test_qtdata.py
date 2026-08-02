"""场景 B：量潮数据（高校老师购买数据服务，几千元）——TC-B01..B07。

对齐 tests.md 业务场景 B：大额对公打款 → 充值入账 → 领满减券/代金券 → 大额订单结算（含力度选择）→ 账本核对。
"""

from __future__ import annotations

from tests.api import ApiClient, unique


def test_b01_recharge_large(api: ApiClient) -> None:
    """TC-B01 开户与大额充值：余额 = 交易求和；重复提交不重复入账。"""
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


def test_b03_combined_settlement(api: ApiClient) -> None:
    """TC-B03 满减 + 代金券 + 余额组合：代金券先于余额。"""
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

    order = api.settle(
        order_id=unique("O-SJ-1"), account_id=acc, scope="data", amount=800000
    )
    # 满减 100000 → 代金券 50000 → 余额 650000；余额剩 150000（tests.md「余额 50000」为笔误）
    api.assert_detail(
        order, [("coupon", 100000), ("voucher", 50000), ("balance", 650000)]
    )
    api.assert_ledger(acc, 150000)


def test_b04_pick_best_full_reduction(api: ApiClient) -> None:
    """TC-B04 多满减券选力度最大（减额最大）。"""
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


def test_b05_scope_isolation(api: ApiClient) -> None:
    """TC-B05 课程券不能用于数据订单（跨业务隔离）。"""
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


def test_b06_pick_best_discount(api: ApiClient) -> None:
    """TC-B06 多折扣券选力度最大（rate 最低，省得最多）。"""
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


def test_b07_voucher_no_change(api: ApiClient) -> None:
    """TC-B07 代金券面值大于剩余应付：只抵应付，不找零（力度约定）。"""
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
