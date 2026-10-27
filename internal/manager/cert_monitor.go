package manager

import (
	"crypto/x509"
	"encoding/pem"
	"log"
	"time"

	"merit/internal/model"
)

// CertExpiryCheck represents a certificate that needs attention
type CertExpiryCheck struct {
	NodeID          string          `json:"nodeId"`
	NodeName        string          `json:"nodeName"`
	PortID          string          `json:"portId"`
	Port            int             `json:"port"`
	Domain          string          `json:"domain"`
	ExpiresAt       time.Time       `json:"expiresAt"`
	DaysUntilExpiry int             `json:"daysUntilExpiry"`
	Level           CertExpiryLevel `json:"level"`
	Message         string          `json:"message"`
}

// GetExpiringCertificates returns all certificates that need attention (warning or critical)
func (s *Server) GetExpiringCertificates() []CertExpiryCheck {
	var checks []CertExpiryCheck

	for _, n := range s.store.Nodes() {
		for _, p := range n.Ports {
			// Only check ports with TLS certificates
			if model.NormalizeProxyType(p.ProxyType) != model.ProxyTypeNaive || p.TLSCert == "" {
				continue
			}

			// Skip ACME-managed certificates (they auto-renew)
			if p.AutoACME {
				continue
			}

			// Parse certificate
			certInfo := extractCertInfo(p.TLSCert)
			if certInfo == nil {
				continue
			}

			level, message := classifyCertExpiry(certInfo.DaysToExpire)

			// Only include certificates that need attention
			if level == CertExpiryWarning || level == CertExpiryCritical || level == CertExpiryExpired {
				checks = append(checks, CertExpiryCheck{
					NodeID:          n.ID,
					NodeName:        n.Name,
					PortID:          p.ID,
					Port:            p.Port,
					Domain:          p.Domain,
					ExpiresAt:       certInfo.NotAfter,
					DaysUntilExpiry: certInfo.DaysToExpire,
					Level:           level,
					Message:         message,
				})
			}
		}
	}

	return checks
}

// CheckCertificateExpiry checks a single certificate and returns its status
func CheckCertificateExpiry(certPEM string) (*CertInfo, CertExpiryLevel, string) {
	certInfo := extractCertInfo(certPEM)
	if certInfo == nil {
		return nil, CertExpiryUnknown, "无法解析证书"
	}

	level, message := classifyCertExpiry(certInfo.DaysToExpire)
	return certInfo, level, message
}

// MonitorCertificates runs periodic certificate expiry checks and logs warnings
// This should be called in a goroutine
func (s *Server) MonitorCertificates(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		s.checkAndLogExpiringCertificates()
	}
}

func (s *Server) checkAndLogExpiringCertificates() {
	checks := s.GetExpiringCertificates()

	if len(checks) == 0 {
		return
	}

	// Log warnings for expiring certificates
	for _, check := range checks {
		switch check.Level {
		case CertExpiryExpired:
			log.Printf("⚠️  证书已过期 [节点: %s, 端口: %d, 域名: %s] - %s",
				check.NodeName, check.Port, check.Domain, check.Message)
		case CertExpiryCritical:
			log.Printf("⚠️  证书即将过期 [节点: %s, 端口: %d, 域名: %s] - %s",
				check.NodeName, check.Port, check.Domain, check.Message)
		case CertExpiryWarning:
			log.Printf("ℹ️  证书需要续期 [节点: %s, 端口: %d, 域名: %s] - %s",
				check.NodeName, check.Port, check.Domain, check.Message)
		}
	}
}

// ValidateCertificateChain validates a certificate chain
func ValidateCertificateChain(certPEM string) error {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	// Check if certificate is expired
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return nil
	}
	if now.After(cert.NotAfter) {
		return nil
	}

	return nil
}
