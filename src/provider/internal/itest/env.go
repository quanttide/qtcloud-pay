// Package itest 账本核心集成测试（对齐 roadmap/tests.md）：
// SQLite :memory: 真库 + 全模块真实组装（internal/app.BuildMux）+ 真实 HTTP API 驱动。
// 不 mock、不单独测纯函数——计费等逻辑的正确性由业务旅程的账本断言间接覆盖。
package itest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/app"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
)

// env 集成测试环境。
type env struct {
	t      *testing.T
	db     *gorm.DB // 仅供测试态准备（如回拨过期时间），断言一律走 API
	base   string
	client *http.Client
}

// newEnv 构建测试环境：内存库 + 全模块组装 + HTTP 服务。
func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := app.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1) // :memory: 单连接保证数据一致
	mux, err := app.BuildMux(db, "")
	if err != nil {
		t.Fatalf("build mux: %v", err)
	}
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return &env{t: t, db: db, base: ts.URL, client: ts.Client()}
}

// resp HTTP 响应。
type resp struct {
	status int
	body   []byte
}

// post 发起 JSON POST。
func (e *env) post(path string, body any) *resp {
	e.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		e.t.Fatal(err)
	}
	r, err := e.client.Post(e.base+path, "application/json", bytes.NewReader(b))
	if err != nil {
		e.t.Fatal(err)
	}
	defer r.Body.Close()
	data, _ := io.ReadAll(r.Body)
	return &resp{status: r.StatusCode, body: data}
}

// get 发起 GET。
func (e *env) get(path string) *resp {
	e.t.Helper()
	r, err := e.client.Get(e.base + path)
	if err != nil {
		e.t.Fatal(err)
	}
	defer r.Body.Close()
	data, _ := io.ReadAll(r.Body)
	return &resp{status: r.StatusCode, body: data}
}

// mustStatus 断言状态码。
func (r *resp) mustStatus(e *env, want int) *resp {
	e.t.Helper()
	if r.status != want {
		e.t.Fatalf("status = %d, want %d; body=%s", r.status, want, r.body)
	}
	return r
}

// json 解析响应体。
func (r *resp) json(e *env, v any) *resp {
	e.t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		e.t.Fatalf("decode body: %v; body=%s", err, r.body)
	}
	return r
}

// --- 领域操作辅助（走真实 API） ---

// createAccount 创建账户并返回账户 ID。
func (e *env) createAccount(customerID string) string {
	e.t.Helper()
	var got struct {
		ID string `json:"id"`
	}
	e.post("/accounts", map[string]any{"customer_id": customerID}).
		mustStatus(e, http.StatusCreated).json(e, &got)
	return got.ID
}

// recharge 充值并断言成功。
func (e *env) recharge(accountID string, amount int64, voucherNo string) {
	e.t.Helper()
	e.post("/accounts/"+accountID+"/recharges", map[string]any{
		"amount": amount, "voucher_no": voucherNo,
	}).mustStatus(e, http.StatusOK)
}

// refund 退款（多退）并断言成功。
func (e *env) refund(accountID string, amount int64, voucherNo string) {
	e.t.Helper()
	e.post("/accounts/"+accountID+"/refunds", map[string]any{
		"amount": amount, "voucher_no": voucherNo,
	}).mustStatus(e, http.StatusOK)
}

// account 查询账户。
func (e *env) account(accountID string) map[string]any {
	e.t.Helper()
	var got map[string]any
	e.get("/accounts/"+accountID).mustStatus(e, http.StatusOK).json(e, &got)
	return got
}

// balance 查询账户余额。
func (e *env) balance(accountID string) int64 {
	e.t.Helper()
	return int64(e.account(accountID)["balance"].(float64))
}

// issueCoupon 发放优惠券并断言成功。
func (e *env) issueCoupon(accountID string, c map[string]any) {
	e.t.Helper()
	e.post("/accounts/"+accountID+"/coupons", c).mustStatus(e, http.StatusOK)
}

// issueVoucher 发放代金券并断言成功。
func (e *env) issueVoucher(accountID string, v map[string]any) {
	e.t.Helper()
	e.post("/accounts/"+accountID+"/vouchers", v).mustStatus(e, http.StatusOK)
}

// coupons 查询账户优惠券列表。
func (e *env) coupons(accountID string) []map[string]any {
	e.t.Helper()
	var got struct {
		Coupons []map[string]any `json:"coupons"`
	}
	e.get("/accounts/"+accountID+"/coupons").mustStatus(e, http.StatusOK).json(e, &got)
	return got.Coupons
}

// vouchers 查询账户代金券列表。
func (e *env) vouchers(accountID string) []map[string]any {
	e.t.Helper()
	var got struct {
		Vouchers []map[string]any `json:"vouchers"`
	}
	e.get("/accounts/"+accountID+"/vouchers").mustStatus(e, http.StatusOK).json(e, &got)
	return got.Vouchers
}

// settle 下单结算并断言 201，返回订单。
func (e *env) settle(o map[string]any) map[string]any {
	e.t.Helper()
	var got map[string]any
	e.post("/orders", o).mustStatus(e, http.StatusCreated).json(e, &got)
	return got
}

// order 查询订单。
func (e *env) order(orderID string) map[string]any {
	e.t.Helper()
	var got map[string]any
	e.get("/orders/"+orderID).mustStatus(e, http.StatusOK).json(e, &got)
	return got
}

// transactions 查询账户流水（全量）。
func (e *env) transactions(accountID string) []map[string]any {
	e.t.Helper()
	var got struct {
		Transactions []map[string]any `json:"transactions"`
	}
	e.get("/accounts/"+accountID+"/transactions?limit=100").
		mustStatus(e, http.StatusOK).json(e, &got)
	return got.Transactions
}

// statement 导出账户账单。
func (e *env) statement(accountID string) map[string]any {
	e.t.Helper()
	var got map[string]any
	e.get("/accounts/"+accountID+"/statement").mustStatus(e, http.StatusOK).json(e, &got)
	return got
}

// --- 断言辅助 ---

// deduction 结算明细中的一项（金额与类型可断言，ref_id 为自增 ID 不可预测）。
type deduction struct {
	Kind   string
	Amount int64
}

// detail 解析订单结算明细。
func (e *env) detail(order map[string]any) []map[string]any {
	e.t.Helper()
	raw, ok := order["settle_detail"].([]any)
	if !ok {
		e.t.Fatalf("settle_detail = %v", order["settle_detail"])
	}
	detail := make([]map[string]any, 0, len(raw))
	for _, d := range raw {
		detail = append(detail, d.(map[string]any))
	}
	return detail
}

// assertDetail 断言结算明细的类型与金额序列。
func (e *env) assertDetail(order map[string]any, want []deduction) {
	e.t.Helper()
	got := e.detail(order)
	if len(got) != len(want) {
		e.t.Fatalf("detail = %v, want %+v", got, want)
	}
	for i, w := range want {
		if got[i]["kind"] != w.Kind || int64(got[i]["amount"].(float64)) != w.Amount {
			e.t.Errorf("detail[%d] = %v, want %+v", i, got[i], w)
		}
	}
}

// assertLedger 核对账本：余额正确且与交易一致（不错）。
func (e *env) assertLedger(accountID string, wantBalance int64) {
	e.t.Helper()
	if got := e.balance(accountID); got != wantBalance {
		e.t.Errorf("balance = %d, want %d", got, wantBalance)
	}
	var got struct {
		Discrepancies []map[string]any `json:"discrepancies"`
	}
	e.get("/reconcile/consistency").mustStatus(e, http.StatusOK).json(e, &got)
	for _, d := range got.Discrepancies {
		if d["account_id"] == accountID {
			e.t.Errorf("consistency discrepancy: %v", d)
		}
	}
}

// countType 统计账户流水中指定类型的条数。
func (e *env) countType(accountID, typ string) int {
	e.t.Helper()
	n := 0
	for _, tx := range e.transactions(accountID) {
		if tx["type"] == typ {
			n++
		}
	}
	return n
}

// netFlow 账户净变动：Σ(充值) − Σ(余额支付) − Σ(退款)。
func (e *env) netFlow(accountID string) int64 {
	e.t.Helper()
	var sum int64
	for _, tx := range e.transactions(accountID) {
		switch tx["type"] {
		case "recharge":
			sum += int64(tx["amount"].(float64))
		case "consume", "refund":
			sum -= int64(tx["amount"].(float64))
		}
	}
	return sum
}

// backdateCoupon 将指定批次第一张券的过期时间改为过去（测试过期场景的状态准备）。
// batch_no 在 API 响应中隐藏，直接经测试库按批次号定位。
func (e *env) backdateCoupon(accountID, batchNo string) {
	e.t.Helper()
	var c coupon.Coupon
	if err := e.db.Where("account_id = ? AND batch_no = ?", accountID, batchNo).First(&c).Error; err != nil {
		e.t.Fatalf("coupon batch %s: %v", batchNo, err)
	}
	if err := e.db.Model(&coupon.Coupon{}).
		Where("id = ?", c.ID).
		Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		e.t.Fatal(err)
	}
}
