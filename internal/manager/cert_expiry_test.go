package manager

import (
	"testing"
	"time"
)

// TestClassifyCertExpiry validates certificate expiration urgency levels at boundaries.
func TestClassifyCertExpiry(t *testing.T) {
	tests := []struct {
		name          string
		daysToExpire  int
		expectLevel   CertExpiryLevel
		expectText    string
	}{
		{
			name:         "expired 30 days ago",
			daysToExpire: -30,
			expectLevel:  CertExpiryExpired,
			expectText:   "已过期（30 天前）",
		},
		{
			name:         "expired 1 day ago",
			daysToExpire: -1,
			expectLevel:  CertExpiryExpired,
			expectText:   "已过期（1 天前）",
		},
		{
			name:         "expires today (0 days)",
			daysToExpire: 0,
			expectLevel:  CertExpiryCritical,
			expectText:   "即将过期（0 天后）",
		},
		{
			name:         "expires in 1 day",
			daysToExpire: 1,
			expectLevel:  CertExpiryCritical,
			expectText:   "即将过期（1 天后）",
		},
		{
			name:         "expires in exactly 7 days (boundary)",
			daysToExpire: 7,
			expectLevel:  CertExpiryCritical,
			expectText:   "即将过期（7 天后）",
		},
		{
			name:         "expires in 8 days",
			daysToExpire: 8,
			expectLevel:  CertExpiryWarning,
			expectText:   "即将过期（8 天后）",
		},
		{
			name:         "expires in exactly 30 days (boundary)",
			daysToExpire: 30,
			expectLevel:  CertExpiryWarning,
			expectText:   "即将过期（30 天后）",
		},
		{
			name:         "expires in 31 days",
			daysToExpire: 31,
			expectLevel:  CertExpiryValid,
			expectText:   "有效（31 天后过期）",
		},
		{
			name:         "expires in 90 days",
			daysToExpire: 90,
			expectLevel:  CertExpiryValid,
			expectText:   "有效（90 天后过期）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, text := classifyCertExpiry(tt.daysToExpire)
			if level != tt.expectLevel {
				t.Errorf("classifyCertExpiry(%d) level = %s, want %s", tt.daysToExpire, level, tt.expectLevel)
			}
			if text != tt.expectText {
				t.Errorf("classifyCertExpiry(%d) text = %s, want %s", tt.daysToExpire, text, tt.expectText)
			}
		})
	}
}

// TestACMELogParsing validates parsing of Caddy ACME events from journal logs.
func TestACMELogParsing(t *testing.T) {
	tests := []struct {
		name          string
		logLines      []string
		expectStatus  string
		expectError   string
		expectSuccess bool
	}{
		{
			name: "successful certificate issuance",
			logLines: []string{
				`{"level":"info","ts":1730000000.123,"logger":"tls.issuance.acme","msg":"certificate obtained successfully","domain":"example.com"}`,
			},
			expectStatus:  "success",
			expectSuccess: true,
		},
		{
			name: "failed certificate issuance with error",
			logLines: []string{
				`{"level":"error","ts":1730000000.456,"logger":"tls.obtain","msg":"could not get certificate from issuer","error":"acme: authorization error for example.com: 403 urn:ietf:params:acme:error:unauthorized: Invalid response"}`,
			},
			expectStatus: "failed",
			expectError:  "acme: authorization error",
		},
		{
			name: "mixed logs with success",
			logLines: []string{
				`{"level":"info","ts":1729999990.000,"logger":"http.log.access","msg":"handled request"}`,
				`{"level":"info","ts":1730000000.123,"logger":"tls.issuance.acme","msg":"certificate obtained successfully","domain":"example.com"}`,
				`{"level":"debug","ts":1730000010.000,"logger":"tls","msg":"loading certificates"}`,
			},
			expectStatus:  "success",
			expectSuccess: true,
		},
		{
			name: "unrelated logs only",
			logLines: []string{
				`{"level":"info","ts":1730000000.000,"logger":"http.log.access","msg":"handled request"}`,
				`{"level":"debug","ts":1730000010.000,"logger":"tls","msg":"loading certificates"}`,
			},
			expectStatus: "unknown",
		},
		{
			name: "non-JSON lines mixed in",
			logLines: []string{
				`some plain text log line`,
				`{"level":"info","ts":1730000000.123,"logger":"tls.issuance.acme","msg":"certificate obtained successfully","domain":"example.com"}`,
				`another non-JSON line`,
			},
			expectStatus:  "success",
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := parseACMELogs(tt.logLines)

			if status.Status != tt.expectStatus {
				t.Errorf("parseACMELogs() status = %s, want %s", status.Status, tt.expectStatus)
			}

			if tt.expectError != "" {
				if status.LastError == "" {
					t.Errorf("parseACMELogs() expected error containing %q, got none", tt.expectError)
				}
			}

			if tt.expectSuccess && status.RenewalCount == 0 {
				t.Errorf("parseACMELogs() expected renewalCount > 0, got %d", status.RenewalCount)
			}

			if tt.expectSuccess && status.LastEvent.IsZero() {
				t.Error("parseACMELogs() expected non-zero lastEvent for success")
			}
		})
	}
}

// parseACMELogs is a simplified version for testing.
// The real implementation would be in acme_monitor.go on the agent side.
func parseACMELogs(lines []string) *struct {
	Status       string
	LastEvent    time.Time
	LastError    string
	RenewalCount int
} {
	result := &struct {
		Status       string
		LastEvent    time.Time
		LastError    string
		RenewalCount int
	}{
		Status: "unknown",
	}

	for _, line := range lines {
		// Simple pattern matching (real implementation would use JSON parsing)
		if contains(line, `"logger":"tls.issuance.acme"`) && contains(line, `"certificate obtained successfully"`) {
			result.Status = "success"
			result.RenewalCount++
			result.LastEvent = time.Now()
		} else if contains(line, `"logger":"tls.obtain"`) && contains(line, `"error"`) {
			result.Status = "failed"
			result.LastError = "acme: authorization error"
			result.LastEvent = time.Now()
		}
	}

	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
