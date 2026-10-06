package agent

import (
	"testing"

	"merit/internal/model"
)

func TestEgressForPortKeepsAllApplicableRulesInOrder(t *testing.T) {
	cfg := &model.EgressConfig{
		Proxies: []model.EgressProxy{
			{Name: "uk", Enabled: true},
			{Name: "de", Enabled: true},
		},
		Rules: []model.EgressRule{
			{ID: "r1", Order: 0, Action: model.EgressProxyAction, ProxyNames: []string{"uk"}, Domains: []string{"uk.example"}, Enabled: true},
			{ID: "r2", Order: 1, Action: model.EgressProxyAction, ProxyNames: []string{"de"}, Domains: []string{"de.example"}, Enabled: true},
			{ID: "r3", Order: 2, Action: model.EgressDirect, Enabled: true, Ports: []int{1235}},
		},
	}

	got := egressForPort(cfg, 1234)
	if got == nil {
		t.Fatal("expected egress config")
	}
	if len(got.Rules) != 2 || len(got.Proxies) != 2 {
		t.Fatalf("expected two rules and two proxies, got %+v", got)
	}
	if got.Rules[0].Domains[0] != "uk.example" || got.Rules[1].Domains[0] != "de.example" {
		t.Fatalf("rule order was not preserved: %+v", got.Rules)
	}
}

func TestEgressForPortAppliesPortScopeAndAllTargetFallback(t *testing.T) {
	cfg := &model.EgressConfig{
		Proxies: []model.EgressProxy{{Name: "jp", Enabled: true}},
		Rules: []model.EgressRule{
			{ID: "only-1234", Order: 0, Action: model.EgressProxyAction, ProxyNames: []string{"jp"}, Ports: []int{1234}, Enabled: true},
			{ID: "fallback", Order: 1, Action: model.EgressDirect, Enabled: true},
		},
	}

	got := egressForPort(cfg, 1234)
	if got == nil || len(got.Rules) != 2 {
		t.Fatalf("port 1234 should get scoped rule and fallback, got %+v", got)
	}
	if got.Rules[1].Domains[0] != "*" || got.Rules[1].IPRanges[0] != "*" {
		t.Fatalf("empty fallback conditions must become wildcard: %+v", got.Rules[1])
	}

	other := egressForPort(cfg, 5678)
	if other == nil || len(other.Rules) != 1 || other.Rules[0].Action != model.EgressDirect {
		t.Fatalf("port 5678 should only get catch-all direct rule, got %+v", other)
	}
}
