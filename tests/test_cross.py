"""横切场景 X：跨业务通用规则——TC-X01..X06。

对齐 tests.md 横切场景：过期状态机、并发幂等、指定商品券、总对账与三业务总闭环。
"""

from __future__ import annotations

import threading
import time

from tests.api import ApiClient, _yuan, future_expiry, unique


def test_x01_used_coupon_not_reusable(api: ApiClient) -> None:
    """TC-X01 已使用券不可复用：明细无券项，余额补足，券不重复核销。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 20000, unique("GT-001"))
    api.issue_coupon(
        acc, coupon_type="discount", rate=90, scope="course", batch_no=unique("GT-B-001")
    )

    o1 = api.settle(order_id=unique("O-1"), account_id=acc, scope="course", amount=10000)
    api.assert_detail(o1, [("coupon", 1000), ("balance", 9000)])

    # 第二次结算：券已 used，不再参与
    o2 = api.settle(order_id=unique("O-2"), account_id=acc, scope="course", amount=10000)
    api.assert_detail(o2, [("balance", 10000)])
    api.assert_ledger(acc, 1000)
    assert api.count_type(acc, "redeem") == 1  # 券只核销一次


def test_x02_product_scope_coupon(api: ApiClient) -> None:
    """TC-X02 指定商品券仅限该商品。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 20000, unique("GT-001"))
    api.issue_coupon(
        acc,
        coupon_type="full_reduction",
        threshold=10000,
        amount=5000,
        scope="product",
        product_id="course-1",
        batch_no=unique("GT-B-001"),
    )

    # 其他商品：不参与
    o1 = api.settle(
        order_id=unique("O-1"),
        account_id=acc,
        product_id="course-2",
        scope="course",
        amount=10000,
    )
    api.assert_detail(o1, [("balance", 10000)])
    assert api.get_coupons(acc)[0]["status"] == "issued"

    # 指定商品：正常核销
    o2 = api.settle(
        order_id=unique("O-2"),
        account_id=acc,
        product_id="course-1",
        scope="course",
        amount=10000,
    )
    api.assert_detail(o2, [("coupon", 5000), ("balance", 5000)])


def test_x03_expired_status_visible(api: ApiClient) -> None:
    """TC-X03 过期状态查询可见：过期券 expired、可用券 issued。"""
    acc = api.create_account(unique("stu"))
    api.issue_coupon(
        acc,
        coupon_type="discount",
        rate=90,
        scope="course",
        expires_at=future_expiry(seconds=2),
        batch_no=unique("GT-B-001"),
    )
    api.issue_coupon(
        acc, coupon_type="discount", rate=95, scope="course", batch_no=unique("GT-B-002")
    )

    # 等待第一张过期（惰性流转在查询时触发）
    time.sleep(3)
    statuses = [c["status"] for c in api.get_coupons(acc)]
    assert sorted(statuses) == ["expired", "issued"], f"statuses = {statuses}"


def test_x04_concurrent_same_order(api: ApiClient) -> None:
    """TC-X04 并发同订单号结算：仅一笔生效（不重）。"""
    acc = api.create_account(unique("stu"))
    api.recharge(acc, 20000, unique("GT-001"))

    body = {
        "order_id": unique("O-C-1"),
        "account_id": acc,
        "scope": "course",
        "amount": _yuan(10000),
    }
    results: list[int] = []
    lock = threading.Lock()

    def settle() -> None:
        status, _ = api.post("/orders", body)
        with lock:
            results.append(status)

    threads = [threading.Thread(target=settle) for _ in range(2)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    assert results == [201, 201], f"statuses = {results}"
    # 仅一笔生效：余额只扣一次、一条消费交易
    api.assert_ledger(acc, 10000)
    assert api.count_type(acc, "consume") == 1


def test_x05_global_reconciliation(api: ApiClient) -> None:
    """TC-X05 三业务总对账：混合流水后逐账户核对、交易可追溯、账单导出。"""
    # 课堂：充值 20000 + 折扣券消费 10000（券 1000 / 余额 9000）
    stu = api.create_account(unique("stu"))
    api.recharge(stu, 20000, unique("GT-001"))
    api.issue_coupon(
        stu, coupon_type="discount", rate=90, scope="course", batch_no=unique("GT-B-001")
    )
    o1 = api.settle(order_id=unique("O-GT-1"), account_id=stu, scope="course", amount=10000)

    # 数据：充值 800000 + 满减券消费 800000
    teacher = api.create_account(unique("teacher"))
    api.recharge(teacher, 800000, unique("SJ-001"))
    api.issue_coupon(
        teacher,
        coupon_type="full_reduction",
        threshold=500000,
        amount=100000,
        scope="data",
        batch_no=unique("SJ-B-001"),
    )
    o2 = api.settle(
        order_id=unique("O-SJ-1"), account_id=teacher, scope="data", amount=800000
    )

    # 云：充值 10000 + 按量消费 3000
    cloud = api.create_account(unique("cloud"))
    api.recharge(cloud, 10000, unique("Y-001"))
    o3 = api.settle(order_id=unique("O-Y-1"), account_id=cloud, scope="cloud", amount=3000)

    # 逐账户：余额 = 交易求和，无一致性差异
    api.assert_ledger(stu, 11000)      # 20000 − (10000−1000)
    api.assert_ledger(teacher, 100000)  # 800000 − (800000−100000)
    api.assert_ledger(cloud, 7000)     # 10000 − 3000

    # 交易可追溯：消费带 order_id；券核销关联订单；订单结算明细完整
    for order in (o1, o2, o3):
        assert order["settle_detail"], f"订单 {order['id']} 结算明细为空"
    for acc in (stu, teacher, cloud):
        for tx in api.get_transactions(acc):
            if tx["type"] == "consume":
                assert tx["order_id"], f"consume 缺 order_id: {tx}"
    assert api.get_coupons(stu)[0]["order_id"] == o1["id"]

    # 账单导出：期初 + 净变动 = 期末
    for acc in (stu, teacher, cloud):
        stmt = api.statement(acc)
        assert (
            stmt["opening_balance"] + api.net_flow(acc) == stmt["closing_balance"]
        ), f"账户 {acc} 账单不平衡"


def test_x06_full_closed_loop(api: ApiClient) -> None:
    """TC-X06 三业务总闭环（模拟账户先行）：全程不依赖支付通道。"""
    # 未挂载支付渠道：/pay 不存在（反直觉点：不接入支付完成闭环）
    status, _ = api.get("/pay")
    assert status == 404, f"/pay status = {status}, want 404"

    # 课堂旅程：打款 → 记额度 → 发券 → 学习扣费
    stu = api.create_account(unique("stu"))
    api.recharge(stu, 20000, unique("GT-001"))
    api.issue_coupon(
        stu,
        coupon_type="full_reduction",
        threshold=10000,
        amount=2000,
        scope="course",
        batch_no=unique("GT-B-001"),
    )
    api.settle(order_id=unique("O-GT-1"), account_id=stu, scope="course", amount=10000)

    # 数据旅程
    teacher = api.create_account(unique("teacher"))
    api.recharge(teacher, 800000, unique("SJ-001"))
    api.issue_voucher(teacher, amount=50000, scope="all", batch_no=unique("SJ-V-001"))
    api.settle(
        order_id=unique("O-SJ-1"), account_id=teacher, scope="data", amount=800000
    )

    # 云旅程
    cloud = api.create_account(unique("cloud"))
    api.recharge(cloud, 10000, unique("Y-001"))
    api.settle(order_id=unique("O-Y-1"), account_id=cloud, scope="cloud", amount=3000)

    # 总验收：三个账户账本逐笔对得上
    api.assert_ledger(stu, 12000)      # 20000 − (10000−2000)
    api.assert_ledger(teacher, 50000)  # 800000 − (800000−50000)
    api.assert_ledger(cloud, 7000)     # 10000 − 3000
