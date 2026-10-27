package naive

import (
	"encoding/json"
	"fmt"
	"merit/internal/model"
	"testing"
)

// TestPrintStagingVsProduction prints configuration examples for documentation.
// This is not a real test but a helper to generate examples.
func TestPrintStagingVsProduction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping example generation in short mode")
	}

	port := &model.Port{
		ID:          "test123",
		Port:        443,
		Domain:      "example.com",
		Username:    "user1",
		Password:    "pass1",
		AutoACME:    true,
		ACMEStaging: false,
	}

	// Production CA
	prodConfig, err := GenerateConfig(port)
	if err != nil {
		t.Fatal(err)
	}

	// Parse the JSON string back to object for pretty printing
	var prodObj map[string]interface{}
	if err := json.Unmarshal([]byte(prodConfig), &prodObj); err != nil {
		t.Fatalf("Failed to parse production config: %v", err)
	}
	prodJSON, _ := json.MarshalIndent(prodObj, "", "  ")
	fmt.Println("\n=== Production CA (ACMEStaging=false) ===")
	fmt.Println(string(prodJSON))

	// Staging CA
	port.ACMEStaging = true
	stagingConfig, err := GenerateConfig(port)
	if err != nil {
		t.Fatal(err)
	}

	var stagingObj map[string]interface{}
	if err := json.Unmarshal([]byte(stagingConfig), &stagingObj); err != nil {
		t.Fatalf("Failed to parse staging config: %v", err)
	}
	stagingJSON, _ := json.MarshalIndent(stagingObj, "", "  ")
	fmt.Println("\n=== Staging CA (ACMEStaging=true) ===")
	fmt.Println(string(stagingJSON))

	// Show the diff
	fmt.Println("\n=== Key difference: staging CA URL ===")
	if apps, ok := stagingObj["apps"].(map[string]interface{}); ok {
		if tls, ok := apps["tls"].(map[string]interface{}); ok {
			if auto, ok := tls["automation"].(map[string]interface{}); ok {
				if policies, ok := auto["policies"].([]interface{}); ok && len(policies) > 0 {
					if policy, ok := policies[0].(map[string]interface{}); ok {
						if issuers, ok := policy["issuers"].([]interface{}); ok && len(issuers) > 0 {
							issuerJSON, _ := json.MarshalIndent(issuers[0], "", "  ")
							fmt.Println(string(issuerJSON))
						}
					}
				}
			}
		}
	}
}
