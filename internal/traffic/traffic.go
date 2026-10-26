// Package traffic provides traffic monitoring and control.
package traffic

import (
	"fmt"
	"time"
)

// FormatBytes formats bytes into human-readable format.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}

// ShouldResetTraffic checks if traffic counter should be reset based on reset time.
func ShouldResetTraffic(lastReset time.Time) bool {
	if lastReset.IsZero() {
		return true
	}
	now := time.Now()
	// Reset monthly on the same day
	return now.Year() > lastReset.Year() ||
		(now.Year() == lastReset.Year() && now.Month() > lastReset.Month())
}

// IsTrafficExceeded checks if traffic limit is exceeded.
func IsTrafficExceeded(used, limit int64) bool {
	if limit == 0 {
		return false // unlimited
	}
	return used >= limit
}

// CalculatePercentage calculates the percentage of traffic used.
func CalculatePercentage(used, limit int64) float64 {
	if limit == 0 {
		return 0
	}
	return float64(used) / float64(limit) * 100
}
