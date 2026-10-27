package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"merit/internal/model"
)

// ACMEEvent represents a parsed ACME lifecycle event from Caddy logs.
type ACMEEvent struct {
	Timestamp time.Time
	Level     string // info, error, etc.
	Logger    string // tls.issuance.acme, tls.obtain, tls.renew
	Message   string
	Error     string // Present when level=error
	Success   bool   // Inferred from level and message
}

// QueryACMEStatus reads the systemd journal for the given port's NaïveProxy unit
// and extracts recent ACME signing/renewal events. It returns nil if the unit has
// no journal logs or ACME events cannot be found.
//
// Journal query is limited to the most recent 500 lines to avoid unbounded reads
// on long-running units.
func (a *Agent) QueryACMEStatus(port *model.Port) *model.ACMEStatus {
	if model.NormalizeProxyType(port.ProxyType) != model.ProxyTypeNaive || !port.AutoACME {
		return nil
	}

	unitName := a.proxyServiceName(port)
	if unitName == "" {
		return nil
	}

	// Query last 500 lines from this unit's journal, newest first
	cmd := exec.Command("journalctl", "-u", unitName+".service", "-n", "500", "--no-pager", "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		// Unit may not exist yet or journal is unavailable
		return &model.ACMEStatus{Status: "unknown"}
	}

	events := parseACMEEvents(out)
	if len(events) == 0 {
		return &model.ACMEStatus{Status: "unknown"}
	}

	// Most recent event is first (we queried newest first and parsed in order)
	latest := events[0]
	status := &model.ACMEStatus{
		LastEvent: latest.Timestamp,
	}

	if latest.Success {
		status.Status = "success"
		// Count successful renewals
		for _, e := range events {
			if e.Success {
				status.RenewalCount++
			}
		}
	} else {
		status.Status = "failed"
		status.LastError = latest.Message
		if latest.Error != "" {
			status.LastError = latest.Error
		}
	}

	return status
}

// parseACMEEvents scans journalctl JSON output for ACME-related log entries.
// It tolerates non-JSON lines and unrelated log entries, returning only recognized
// ACME events sorted newest-first.
func parseACMEEvents(raw []byte) []ACMEEvent {
	var events []ACMEEvent
	scanner := bufio.NewScanner(bytes.NewReader(raw))

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry struct {
			Message   string `json:"MESSAGE"`
			Timestamp string `json:"__REALTIME_TIMESTAMP"` // microseconds since epoch
		}

		if err := json.Unmarshal(line, &entry); err != nil {
			// Tolerate non-JSON lines (e.g. from systemd metadata)
			continue
		}

		// Caddy may log plain text or JSON. Try to parse as JSON first.
		var caddyLog struct {
			Level   string `json:"level"`
			Ts      float64 `json:"ts"`
			Logger  string `json:"logger"`
			Msg     string `json:"msg"`
			Error   string `json:"error"`
		}

		if err := json.Unmarshal([]byte(entry.Message), &caddyLog); err != nil {
			// Not JSON structured log, skip
			continue
		}

		// Filter for ACME-related loggers
		if !isACMELogger(caddyLog.Logger) {
			continue
		}

		event := ACMEEvent{
			Level:   caddyLog.Level,
			Logger:  caddyLog.Logger,
			Message: caddyLog.Msg,
			Error:   caddyLog.Error,
		}

		// Parse timestamp from Caddy log (UNIX timestamp with fractional seconds)
		if caddyLog.Ts > 0 {
			event.Timestamp = time.Unix(int64(caddyLog.Ts), int64((caddyLog.Ts-float64(int64(caddyLog.Ts)))*1e9))
		} else if entry.Timestamp != "" {
			// Fallback to systemd journal timestamp (microseconds)
			if usec, err := parseJournalTimestamp(entry.Timestamp); err == nil {
				event.Timestamp = time.Unix(usec/1000000, (usec%1000000)*1000)
			}
		}

		// Classify success/failure
		event.Success = classifyACMESuccess(caddyLog.Level, caddyLog.Msg, caddyLog.Error)

		events = append(events, event)
	}

	return events
}

// isACMELogger returns true if the logger name indicates an ACME lifecycle event.
func isACMELogger(logger string) bool {
	// Caddy ACME loggers: tls.issuance.acme, tls.obtain, tls.renew, tls.on_demand
	return strings.Contains(logger, "tls.issuance.acme") ||
		strings.Contains(logger, "tls.obtain") ||
		strings.Contains(logger, "tls.renew") ||
		strings.Contains(logger, "acme")
}

// classifyACMESuccess infers whether an ACME event succeeded based on log level
// and message content.
func classifyACMESuccess(level, msg, errStr string) bool {
	if level == "error" || errStr != "" {
		return false
	}
	// Common success indicators in Caddy ACME logs
	successKeywords := []string{
		"certificate obtained successfully",
		"renewed successfully",
		"successfully obtained certificate",
		"successfully renewed",
		"certificate installed",
	}
	msgLower := strings.ToLower(msg)
	for _, kw := range successKeywords {
		if strings.Contains(msgLower, kw) {
			return true
		}
	}
	// If level=info and no error, assume success for obtain/renew messages
	if level == "info" && (strings.Contains(msgLower, "obtain") || strings.Contains(msgLower, "renew")) {
		return true
	}
	return false
}

// parseJournalTimestamp parses systemd's __REALTIME_TIMESTAMP (microseconds).
func parseJournalTimestamp(s string) (int64, error) {
	var usec int64
	_, err := fmt.Sscanf(s, "%d", &usec)
	return usec, err
}
