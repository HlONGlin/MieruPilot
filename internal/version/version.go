// Package version provides version compatibility checking for Merit components.
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major int
	Minor int
	Patch int
	Pre   string // e.g., "rc1", "beta"
}

// Parse parses a semantic version string (e.g., "v1.6.0", "1.6.0-rc1").
func Parse(v string) (*Version, error) {
	// Remove leading 'v' if present
	v = strings.TrimPrefix(v, "v")

	// Handle pre-release versions
	var pre string
	if idx := strings.IndexAny(v, "-+"); idx > 0 {
		pre = v[idx+1:]
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch := 0
	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}

	return &Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Pre:   pre,
	}, nil
}

// String returns the string representation of the version.
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// IsCompatibleWith checks if this version is compatible with the target version.
// Compatible means:
// - Major version must match exactly
// - Minor version difference must be <= 1
// - Pre-release versions are considered compatible if major.minor matches
func (v *Version) IsCompatibleWith(target *Version) bool {
	if v.Major != target.Major {
		return false
	}

	minorDiff := abs(v.Minor - target.Minor)
	return minorDiff <= 1
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// CheckAgentCompatibility checks if an agent version is compatible with the manager version.
// Returns nil if compatible, otherwise returns an error with a descriptive message.
func CheckAgentCompatibility(agentVersion, managerVersion string) error {
	// Handle development/test versions
	if agentVersion == "" || agentVersion == "dev" || agentVersion == "unknown" {
		return nil // Allow dev versions for testing
	}

	agent, err := Parse(agentVersion)
	if err != nil {
		return fmt.Errorf("无法解析 Agent 版本 %s: %v", agentVersion, err)
	}

	manager, err := Parse(managerVersion)
	if err != nil {
		// If manager version is invalid, allow connection (dev mode)
		return nil
	}

	if !agent.IsCompatibleWith(manager) {
		return fmt.Errorf("Agent 版本 %s 与 Manager 版本 %s 不兼容，请升级 Agent",
			agent.String(), manager.String())
	}

	return nil
}

// GetMinimumAgentVersion returns the minimum agent version required by this manager version.
func GetMinimumAgentVersion(managerVersion string) string {
	manager, err := Parse(managerVersion)
	if err != nil {
		return "1.6.0" // Fallback to a safe minimum
	}

	// Minimum agent version is one minor version back
	minMinor := manager.Minor - 1
	if minMinor < 0 {
		minMinor = 0
	}

	return fmt.Sprintf("%d.%d.0", manager.Major, minMinor)
}

// IsUpgradeRecommended checks if an agent should be upgraded.
// Returns true if the agent is more than one minor version behind.
func IsUpgradeRecommended(agentVersion, managerVersion string) bool {
	agent, err := Parse(agentVersion)
	if err != nil {
		return false
	}

	manager, err := Parse(managerVersion)
	if err != nil {
		return false
	}

	if agent.Major != manager.Major {
		return true
	}

	// Recommend upgrade if agent is behind
	return agent.Minor < manager.Minor ||
		(agent.Minor == manager.Minor && agent.Patch < manager.Patch)
}

// ExtractVersionFromBuildInfo extracts version from buildinfo string format.
// Handles formats like "v1.6.0 (abc123f, 2024-01-15)" or just "v1.6.0"
func ExtractVersionFromBuildInfo(buildInfo string) string {
	// Try to extract version using regex
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(buildInfo)
	if len(matches) > 1 {
		return matches[1]
	}
	return buildInfo
}
