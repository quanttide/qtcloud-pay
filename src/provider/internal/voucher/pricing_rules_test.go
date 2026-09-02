package voucher_test

import (
	"context"
	"errors"
	"testing"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

func validPricingPayload() string {
	return `{
  "meta": {"source": "journal qtclass/2026-09-01.md", "updated_at": "2026-09-01"},
  "issuance": {
    "channels": [
      {"name": "课堂实训任务", "trigger": "实训任务验收通过", "voucher": {"amount_cents": 10000, "scope": "all", "expires_at_rule": "发放时确定"}, "count_per_event": 1, "entry": "class.quanttide.com/learn"},
      {"name": "众包任务", "trigger": "众包任务交付验收", "voucher": {"amount_cents": 100000, "scope": "all", "expires_at_rule": "发放时确定"}, "count_per_event": 1, "entry": "crowd.quanttide.com"},
      {"name": "实训基地代表履职（课题立项/结项初审）", "trigger": "完成一次初审", "voucher": {"amount_cents": 10000, "scope": "all", "expires_at_rule": "发放时确定"}, "count_per_event": 1, "entry": "任命公告 2026-09-01"},
      {"name": "课题通过初审奖励", "trigger": "课题初审通过", "voucher": {"amount_cents": 50000, "scope": "all", "expires_at_rule": "发放时确定"}, "count_per_event": 1, "entry": "课题评审流程"}
    ]
  },
  "redemption": {
    "scenarios": [
      {"scenario": "one_on_one_consultation", "name": "一对一咨询预约", "pricing_model": "per_hour_by_rank", "rank_prices_cents": [
        {"rank": "chief", "price_cents": 50000},
        {"rank": "senior", "price_cents": 40000},
        {"rank": "advanced", "price_cents": 30000},
        {"rank": "intermediate", "price_cents": 20000},
        {"rank": "junior", "price_cents": 10000}
      ]},
      {"scenario": "extra_application_quota", "name": "超额申请额度（考核限额）", "pricing_model": "per_count_flat", "quotas": [
        {"application_type": "project_proposal", "name": "立项申请", "free_limit": 1, "exceed_price_cents": 10000},
        {"application_type": "delivery_review", "name": "交付申请", "free_limit": 3, "exceed_price_cents": 10000}
      ]}
    ]
  },
  "billing_semantics": {"voucher_is_money": true, "open_questions": ["代金券与余额的核销顺序"]}
}`
}

func TestPricingRuleSet_UpsertGetList(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	got, err := svc.UpsertPricingRuleSet(ctx, &voucher.PricingRuleSet{
		ID:      "qtclass-voucher-pricing",
		Source:  "payment-engineering/qtclass/voucher-pricing.json",
		Version: "2026-09-01",
		Payload: validPricingPayload(),
	})
	if err != nil {
		t.Fatalf("UpsertPricingRuleSet: %v", err)
	}
	if got.ID != "qtclass-voucher-pricing" || got.Version != "2026-09-01" {
		t.Fatalf("rule set = %+v", got)
	}

	got, err = svc.UpsertPricingRuleSet(ctx, &voucher.PricingRuleSet{
		ID:      "qtclass-voucher-pricing",
		Source:  "payment-engineering/qtclass/voucher-pricing.json",
		Version: "2026-09-02",
		Payload: validPricingPayload(),
	})
	if err != nil {
		t.Fatalf("UpsertPricingRuleSet(update): %v", err)
	}
	if got.Version != "2026-09-02" {
		t.Fatalf("version = %q, want updated", got.Version)
	}

	list, err := svc.ListPricingRuleSets(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListPricingRuleSets = %d, %v", len(list), err)
	}
	fetched, err := svc.GetPricingRuleSet(ctx, "qtclass-voucher-pricing")
	if err != nil || fetched.Source == "" {
		t.Fatalf("GetPricingRuleSet = %+v, %v", fetched, err)
	}
}

func TestPricingRuleSet_Validation(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	cases := []struct {
		name    string
		payload string
	}{
		{"bad json", `not-json`},
		{"amount not cents", `{"issuance":{"channels":[{"name":"x","trigger":"x","voucher":{"amount_cents":0,"scope":"all","expires_at_rule":"发放时确定"},"count_per_event":1}]},"redemption":{"scenarios":[{"scenario":"s","name":"n","pricing_model":"per_count_flat","quotas":[{"application_type":"a","name":"n","free_limit":0,"exceed_price_cents":10000}]}]},"billing_semantics":{"voucher_is_money":true}}`},
		{"bad scope", `{"issuance":{"channels":[{"name":"x","trigger":"x","voucher":{"amount_cents":10000,"scope":"bad","expires_at_rule":"发放时确定"},"count_per_event":1}]},"redemption":{"scenarios":[{"scenario":"s","name":"n","pricing_model":"per_count_flat","quotas":[{"application_type":"a","name":"n","free_limit":0,"exceed_price_cents":10000}]}]},"billing_semantics":{"voucher_is_money":true}}`},
		{"missing rank dimension", `{"issuance":{"channels":[{"name":"x","trigger":"x","voucher":{"amount_cents":10000,"scope":"all","expires_at_rule":"发放时确定"},"count_per_event":1}]},"redemption":{"scenarios":[{"scenario":"one_on_one_consultation","name":"n","pricing_model":"per_hour_by_rank"}]},"billing_semantics":{"voucher_is_money":true}}`},
		{"voucher not money", `{"issuance":{"channels":[{"name":"x","trigger":"x","voucher":{"amount_cents":10000,"scope":"all","expires_at_rule":"发放时确定"},"count_per_event":1}]},"redemption":{"scenarios":[{"scenario":"s","name":"n","pricing_model":"per_count_flat","quotas":[{"application_type":"a","name":"n","free_limit":0,"exceed_price_cents":10000}]}]},"billing_semantics":{"voucher_is_money":false}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.UpsertPricingRuleSet(ctx, &voucher.PricingRuleSet{
				ID:      "qtclass-voucher-pricing",
				Payload: c.payload,
			})
			if !errors.Is(err, voucher.ErrInvalidRuleSet) {
				t.Fatalf("err = %v, want ErrInvalidRuleSet", err)
			}
		})
	}
}

func TestPricingRuleSet_NotFound(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.GetPricingRuleSet(context.Background(), "missing"); !errors.Is(err, voucher.ErrRuleSetNotFound) {
		t.Fatalf("err = %v, want ErrRuleSetNotFound", err)
	}
}
