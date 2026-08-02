"""provider 服务端集成测试冒烟用例：验证二进制可启动且账本 API 可用。"""


def test_server_health(api):
    # 服务已就绪：一致性校验端点可用
    status, body = api.get("/reconcile/consistency")
    assert status == 200
    assert body["discrepancies"] == []


def test_create_account_and_recharge(api):
    # 开户 → 充值 → 余额与流水正确
    status, account = api.post("/accounts", {"customer_id": "stu-1"})
    assert status == 201
    account_id = account["id"]

    status, _ = api.post(
        f"/accounts/{account_id}/recharges",
        {"amount": 20000, "voucher_no": "GT-001"},
    )
    assert status == 200

    status, account = api.get(f"/accounts/{account_id}")
    assert status == 200
    assert account["balance"] == 20000

    status, txs = api.get(f"/accounts/{account_id}/transactions")
    assert status == 200
    assert len(txs["transactions"]) == 1
    assert txs["transactions"][0]["type"] == "recharge"


def test_settle_order(api):
    # 完整小闭环：开户 → 充值 → 下单结算
    _, account = api.post("/accounts", {"customer_id": "stu-2"})
    account_id = account["id"]
    api.post(
        f"/accounts/{account_id}/recharges",
        {"amount": 10000, "voucher_no": "GT-002"},
    )

    status, order = api.post(
        "/orders",
        {
            "order_id": "O-GT-1",
            "account_id": account_id,
            "scope": "course",
            "amount": 10000,
        },
    )
    assert status == 201
    assert order["status"] == "settled"

    _, account = api.get(f"/accounts/{account_id}")
    assert account["balance"] == 0
