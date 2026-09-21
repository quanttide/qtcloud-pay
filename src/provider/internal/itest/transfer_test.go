package itest

import (
	"net/http"
	"strconv"
	"testing"
)

func TestVoucherTransferJourney(t *testing.T) {
	e := newEnv(t)
	from := e.createAccount("transfer_from")
	to := e.createAccount("transfer_to")
	e.recharge(from, 10000, "transfer-recharge-1")

	var created map[string]any
	e.post("/transfers", map[string]any{
		"from_account_id": from,
		"to_account_id":   to,
		"amount":          amountOf(2500),
		"note":            "实训基地积分转赠",
		"idempotency_key": "itest-transfer-1",
	}).mustStatus(e, http.StatusCreated).json(e, &created)
	if created["status"] != "pending" || centsOf(created["amount"]) != 2500 {
		t.Fatalf("created = %+v", created)
	}
	e.assertLedger(from, 7500)
	if got := e.countType(from, "consume"); got != 1 {
		t.Fatalf("consume count = %d, want 1", got)
	}

	var dup map[string]any
	e.post("/transfers", map[string]any{
		"from_account_id": from,
		"to_account_id":   to,
		"amount":          amountOf(2500),
		"idempotency_key": "itest-transfer-1",
	}).mustStatus(e, http.StatusCreated).json(e, &dup)
	if dup["id"] != created["id"] || e.countType(from, "consume") != 1 {
		t.Fatalf("dup = %+v created=%+v", dup, created)
	}

	var reviewed map[string]any
	e.post("/transfers/"+idString(created["id"])+"/review", map[string]any{
		"decision":    "approved",
		"reviewed_by": "赵子奕",
		"note":        "通过",
	}).mustStatus(e, http.StatusOK).json(e, &reviewed)
	if reviewed["status"] != "approved" || reviewed["issued_voucher_id"] == nil || centsOf(reviewed["issued_amount"]) != 2500 {
		t.Fatalf("reviewed = %+v", reviewed)
	}
	vouchers := e.vouchers(to)
	if len(vouchers) != 1 || centsOf(vouchers[0]["amount"]) != 2500 {
		t.Fatalf("vouchers = %+v", vouchers)
	}
	e.assertLedger(to, 0) // 发券不改变账户余额，但会进入账本和对账链路。

	var listed struct {
		Transfers []map[string]any `json:"transfers"`
	}
	e.get("/transfers?account_id="+to).mustStatus(e, http.StatusOK).json(e, &listed)
	if len(listed.Transfers) != 1 || listed.Transfers[0]["status"] != "approved" {
		t.Fatalf("listed = %+v", listed)
	}
}

func TestVoucherTransferInsufficientBalanceAPI(t *testing.T) {
	e := newEnv(t)
	from := e.createAccount("transfer_empty")
	to := e.createAccount("transfer_target")
	e.post("/transfers", map[string]any{
		"from_account_id": from,
		"to_account_id":   to,
		"amount":          amountOf(100),
		"idempotency_key": "itest-transfer-insufficient",
	}).mustStatus(e, http.StatusUnprocessableEntity)
	if txs := e.transactions(from); len(txs) != 0 {
		t.Fatalf("txs = %+v, want none", txs)
	}
}

func TestVoucherTransferRejectAPI(t *testing.T) {
	e := newEnv(t)
	from := e.createAccount("transfer_reject_from")
	to := e.createAccount("transfer_reject_to")
	e.recharge(from, 3000, "transfer-reject-recharge")
	var created map[string]any
	e.post("/transfers", map[string]any{
		"from_account_id": from,
		"to_account_id":   to,
		"amount":          amountOf(1000),
		"idempotency_key": "itest-transfer-reject",
	}).mustStatus(e, http.StatusCreated).json(e, &created)
	var reviewed map[string]any
	e.post("/transfers/"+idString(created["id"])+"/review", map[string]any{
		"decision":    "rejected",
		"reviewed_by": "刘婧怡",
	}).mustStatus(e, http.StatusOK).json(e, &reviewed)
	if reviewed["status"] != "rejected" || reviewed["issued_voucher_id"] != nil {
		t.Fatalf("reviewed = %+v", reviewed)
	}
	if vouchers := e.vouchers(to); len(vouchers) != 0 {
		t.Fatalf("vouchers = %+v, want none", vouchers)
	}
}

func idString(v any) string {
	return strconv.FormatInt(int64(v.(float64)), 10)
}
