package agent

import (
	"strings"
	"testing"

	"merit/internal/model"
)

func TestEgressForPortKeepsAllRulesForThatPort(t *testing.T) {
	cfg := &model.EgressConfig{
		Proxies: []model.EgressProxy{{Name: "uk", Enabled: true}, {Name: "de", Enabled: true}},
		Rules: []model.EgressRule{
			{Order: 0, Ports: []int{37065}, Domains: []string{"uk.example"}, Action: model.EgressProxyAction, ProxyNames: []string{"uk"}, Enabled: true},
			{Order: 1, Ports: []int{37065}, Domains: []string{"de.example"}, Action: model.EgressProxyAction, ProxyNames: []string{"de"}, Enabled: true},
			{Order: 2, Ports: []int{37066}, Domains: []string{"other.example"}, Action: model.EgressProxyAction, ProxyNames: []string{"uk"}, Enabled: true},
		},
	}
	got := egressForPort(cfg, 37065)
	if got == nil || len(got.Rules) != 2 || len(got.Proxies) != 2 {
		t.Fatalf("expected both 37065 rules and proxies, got %+v", got)
	}
	if got.Rules[0].Domains[0] != "uk.example" || got.Rules[1].Domains[0] != "de.example" {
		t.Fatalf("rule order not preserved: %+v", got.Rules)
	}
}

func TestInstanceServiceUsesPortScopedConfigAndSocket(t *testing.T) {
	unit := instanceService("merit-mita-37065", "/etc/merit-mita/37065.json")
	for _, expected := range []string{
		"User=mita",
		"MITA_CONFIG_JSON_FILE=/etc/merit-mita/37065.json",
		"MITA_UDS_PATH=/var/run/mita/merit-mita-37065.sock",
		"ExecStart=/usr/bin/mita run",
		"chown mita:mita /etc/merit-mita/37065.json",
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("instance unit missing %q:\n%s", expected, unit)
		}
	}
}

func TestConfigDigestChangesOnlyWhenBytesChange(t *testing.T) {
	one := []byte(`{"portBindings":[{"port":1001}]}`)
	if configDigest(one) != configDigest(append([]byte(nil), one...)) {
		t.Fatal("identical config should retain the same digest")
	}
	if configDigest(one) == configDigest([]byte(`{"portBindings":[{"port":1002}]}`)) {
		t.Fatal("changed config should produce a different digest")
	}
}

func TestEgressForPortAddsWildcardForAllDestinations(t *testing.T) {
	cfg := &model.EgressConfig{Rules: []model.EgressRule{{Order: 0, Ports: []int{37065}, Action: model.EgressDirect, Enabled: true}}}
	got := egressForPort(cfg, 37065)
	if got == nil || len(got.Rules) != 1 || len(got.Rules[0].Domains) != 1 || got.Rules[0].Domains[0] != "*" || got.Rules[0].IPRanges[0] != "*" {
		t.Fatalf("expected wildcard catch-all rule, got %+v", got)
	}
}
