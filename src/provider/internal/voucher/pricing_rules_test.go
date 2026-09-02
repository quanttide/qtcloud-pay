package voucher_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

func validPricingPayload() string {
	b, err := os.ReadFile("testdata/qtclass-voucher-pricing.json")
	if err != nil {
		panic(err)
	}
	return string(b)
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
	var payload struct {
		Issuance struct {
			Channels []struct {
				BonusType string `json:"bonus_type"`
				Voucher   struct {
					AmountCents int64  `json:"amount_cents"`
					AmountRule  string `json:"amount_rule"`
				} `json:"voucher"`
			} `json:"channels"`
		} `json:"issuance"`
	}
	if err := json.Unmarshal([]byte(got.Payload), &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if len(payload.Issuance.Channels) != 7 {
		t.Fatalf("channels = %d, want 7", len(payload.Issuance.Channels))
	}
	if payload.Issuance.Channels[4].BonusType != "first_completion" || payload.Issuance.Channels[4].Voucher.AmountRule == "" {
		t.Fatalf("bonus channel = %+v", payload.Issuance.Channels[4])
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
		{"bonus without denomination rule", `{"issuance":{"channels":[{"name":"x","trigger":"x","bonus_type":"first_completion","voucher":{"amount_rule":"等额追加","scope":"all","expires_at_rule":"发放时确定"},"count_per_event":"多张券组合"}]},"redemption":{"scenarios":[{"scenario":"s","name":"n","pricing_model":"per_count_flat","quotas":[{"application_type":"a","name":"n","free_limit":0,"exceed_price_cents":10000}]}]},"billing_semantics":{"voucher_is_money":true}}`},
		{"bad bonus type", `{"issuance":{"channels":[{"name":"x","trigger":"x","bonus_type":"bad","voucher":{"amount_rule":"等额追加","scope":"all","expires_at_rule":"发放时确定"},"count_per_event":"多张券组合"}],"bonus_denomination_rule":"追加奖励当前仅使用 100 元、500 元两种面额"},"redemption":{"scenarios":[{"scenario":"s","name":"n","pricing_model":"per_count_flat","quotas":[{"application_type":"a","name":"n","free_limit":0,"exceed_price_cents":10000}]}]},"billing_semantics":{"voucher_is_money":true}}`},
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
