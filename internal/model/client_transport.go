package model

import "strings"

// EffectiveTLSFingerprint preserves existing clients unless explicitly changed.
func (p *Port) EffectiveTLSFingerprint() string {
	if p.TLSFingerprint == "" {
		return "chrome"
	}
	return strings.ToLower(strings.TrimSpace(p.TLSFingerprint))
}

// EffectiveNaiveTransport preserves HTTPS for existing entries.
func (p *Port) EffectiveNaiveTransport() string {
	if p.NaiveTransport == "" {
		return "https"
	}
	return strings.ToLower(strings.TrimSpace(p.NaiveTransport))
}
