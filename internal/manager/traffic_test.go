package manager

import (
	"testing"
	"time"

	"merit/internal/model"
)

func TestUpdatePortTrafficAccumulatesDelta(t *testing.T) {
	p := &model.Port{TrafficRaw: 100, TrafficResetAt: time.Now()}
	if updatePortTraffic(p, 250, time.Now()) {
		t.Fatal("unexpected sync request without limit transition")
	}
	if p.TrafficUsed != 150 || p.TrafficRaw != 250 {
		t.Fatalf("unexpected counters: used=%d raw=%d", p.TrafficUsed, p.TrafficRaw)
	}
	updatePortTraffic(p, 40, time.Now())
	if p.TrafficUsed != 190 || p.TrafficRaw != 40 {
		t.Fatalf("restart delta should use new raw value: used=%d raw=%d", p.TrafficUsed, p.TrafficRaw)
	}
}

func TestUpdatePortTrafficDisablesAtLimitAndResetReenables(t *testing.T) {
	now := time.Now()
	p := &model.Port{TrafficLimit: 100, TrafficResetAt: now, Enabled: true}
	if !updatePortTraffic(p, 100, now) || p.Enabled || !p.AutoDisabled {
		t.Fatalf("expected automatic disable: %+v", p)
	}
	// Use the first day of the previous month so the test remains valid across
	// month lengths and at any time of day.
	previousMonth := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	p.TrafficResetAt = previousMonth
	if !updatePortTraffic(p, 100, now) || !p.Enabled || p.AutoDisabled || p.TrafficUsed != 0 {
		t.Fatalf("expected monthly reset and re-enable: %+v", p)
	}
}

func TestUpdatePortTrafficIgnoresUnavailableZeroSample(t *testing.T) {
	p := &model.Port{TrafficUsed: 55, TrafficRaw: 40, TrafficResetAt: time.Now()}
	if updatePortTraffic(p, 0, time.Now()) {
		t.Fatal("zero sample should not change state")
	}
	if p.TrafficUsed != 55 || p.TrafficRaw != 40 {
		t.Fatalf("zero sample changed counters: %+v", p)
	}
}
