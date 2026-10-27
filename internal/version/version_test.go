package version

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		want    *Version
		wantErr bool
	}{
		{"1.6.0", &Version{1, 6, 0, ""}, false},
		{"v1.6.0", &Version{1, 6, 0, ""}, false},
		{"1.6.1-rc1", &Version{1, 6, 1, "rc1"}, false},
		{"v1.6.0-beta", &Version{1, 6, 0, "beta"}, false},
		{"2.0.0", &Version{2, 0, 0, ""}, false},
		{"1.5", &Version{1, 5, 0, ""}, false},
		{"invalid", nil, true},
		{"1.a.0", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && (got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch || got.Pre != tt.want.Pre) {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCompatibleWith(t *testing.T) {
	tests := []struct {
		agent   string
		manager string
		want    bool
	}{
		{"1.6.0", "1.6.0", true},  // Same version
		{"1.6.0", "1.6.1", true},  // Same minor, different patch
		{"1.6.0", "1.5.0", true},  // One minor version back
		{"1.5.0", "1.6.0", true},  // One minor version forward
		{"1.6.0", "1.7.0", true},  // One minor version forward
		{"1.6.0", "1.8.0", false}, // Two minor versions apart
		{"1.6.0", "2.0.0", false}, // Different major version
		{"2.0.0", "1.6.0", false}, // Different major version
	}

	for _, tt := range tests {
		t.Run(tt.agent+"_"+tt.manager, func(t *testing.T) {
			agent, _ := Parse(tt.agent)
			manager, _ := Parse(tt.manager)
			got := agent.IsCompatibleWith(manager)
			if got != tt.want {
				t.Errorf("IsCompatibleWith(%s, %s) = %v, want %v", tt.agent, tt.manager, got, tt.want)
			}
		})
	}
}

func TestCheckAgentCompatibility(t *testing.T) {
	tests := []struct {
		agent   string
		manager string
		wantErr bool
	}{
		{"1.6.0", "1.6.0", false},
		{"1.6.0", "1.6.1", false},
		{"1.5.0", "1.6.0", false},
		{"1.4.0", "1.6.0", true},  // Too old
		{"2.0.0", "1.6.0", true},  // Different major
		{"dev", "1.6.0", false},   // Dev version allowed
		{"", "1.6.0", false},      // Empty version allowed for testing
		{"unknown", "1.6.0", false}, // Unknown version allowed
	}

	for _, tt := range tests {
		t.Run(tt.agent+"_"+tt.manager, func(t *testing.T) {
			err := CheckAgentCompatibility(tt.agent, tt.manager)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckAgentCompatibility(%s, %s) error = %v, wantErr %v", tt.agent, tt.manager, err, tt.wantErr)
			}
		})
	}
}

func TestIsUpgradeRecommended(t *testing.T) {
	tests := []struct {
		agent   string
		manager string
		want    bool
	}{
		{"1.6.0", "1.6.0", false}, // Same version
		{"1.6.0", "1.6.1", true},  // Patch behind
		{"1.5.0", "1.6.0", true},  // Minor behind
		{"1.6.0", "1.5.0", false}, // Ahead
		{"1.6.0", "2.0.0", true},  // Major behind
	}

	for _, tt := range tests {
		t.Run(tt.agent+"_"+tt.manager, func(t *testing.T) {
			got := IsUpgradeRecommended(tt.agent, tt.manager)
			if got != tt.want {
				t.Errorf("IsUpgradeRecommended(%s, %s) = %v, want %v", tt.agent, tt.manager, got, tt.want)
			}
		})
	}
}

func TestExtractVersionFromBuildInfo(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.6.0 (abc123f, 2024-01-15)", "1.6.0"},
		{"1.6.0", "1.6.0"},
		{"v1.6.1-rc1", "1.6.1"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractVersionFromBuildInfo(tt.input)
			if got != tt.want {
				t.Errorf("ExtractVersionFromBuildInfo(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
