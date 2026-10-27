package manager

import (
	"encoding/json"
	"fmt"
	"merit/internal/model"
	"testing"
	"time"
)

// TestPrintCertStatusExample prints an example CertStatusResponse for documentation.
func TestPrintCertStatusExample(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping example generation in short mode")
	}

	// Example 1: Certificate with warning level
	response1 := CertStatusResponse{
		AutoACME:    true,
		ACMEStaging: false,
		HasCert:     true,
		Domain:      "proxy.example.com",
		CertInfo: &CertInfo{
			Issuer:       "R3",
			Subject:      "CN=proxy.example.com",
			NotBefore:    time.Now().Add(-60 * 24 * time.Hour),
			NotAfter:     time.Now().Add(25 * 24 * time.Hour),
			DaysToExpire: 25,
			DNSNames:     []string{"proxy.example.com"},
		},
		ExpiryLevel:  CertExpiryWarning,
		ExpiryText:   "即将过期（25 天后）",
		ACMEStatus: &model.ACMEStatus{
			Status:       "success",
			LastEvent:    time.Now().Add(-48 * time.Hour),
			RenewalCount: 3,
		},
		ACMEDisabled: false,
	}

	json1, _ := json.MarshalIndent(response1, "", "  ")
	fmt.Println("\n=== Example 1: Certificate with WARNING level (25 days remaining) ===")
	fmt.Println(string(json1))

	// Example 2: Certificate with critical level
	response2 := CertStatusResponse{
		AutoACME:    true,
		ACMEStaging: true,
		HasCert:     true,
		Domain:      "test.example.com",
		CertInfo: &CertInfo{
			Issuer:       "(STAGING) Artificial Apricot R3",
			Subject:      "CN=test.example.com",
			NotBefore:    time.Now().Add(-85 * 24 * time.Hour),
			NotAfter:     time.Now().Add(5 * 24 * time.Hour),
			DaysToExpire: 5,
			DNSNames:     []string{"test.example.com"},
		},
		ExpiryLevel:  CertExpiryCritical,
		ExpiryText:   "即将过期（5 天后）",
		ACMEStatus: &model.ACMEStatus{
			Status:       "success",
			LastEvent:    time.Now().Add(-24 * time.Hour),
			RenewalCount: 1,
		},
		ACMEDisabled: false,
	}

	json2, _ := json.MarshalIndent(response2, "", "  ")
	fmt.Println("\n=== Example 2: Staging certificate with CRITICAL level (5 days remaining) ===")
	fmt.Println(string(json2))

	// Example 3: ACME failure
	response3 := CertStatusResponse{
		AutoACME:     true,
		ACMEStaging:  false,
		HasCert:      false,
		Domain:       "failed.example.com",
		ExpiryLevel:  CertExpiryUnknown,
		ExpiryText:   "等待 ACME 签发",
		ACMEStatus: &model.ACMEStatus{
			Status:    "failed",
			LastEvent: time.Now().Add(-10 * time.Minute),
			LastError: "acme: authorization error for failed.example.com: 403 urn:ietf:params:acme:error:unauthorized: Invalid response from http://failed.example.com/.well-known/acme-challenge/...",
		},
		ACMEDisabled: false,
	}

	json3, _ := json.MarshalIndent(response3, "", "  ")
	fmt.Println("\n=== Example 3: ACME failure (no certificate yet) ===")
	fmt.Println(string(json3))
}
