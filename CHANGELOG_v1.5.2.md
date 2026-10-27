# Changelog v1.5.2

## 🐛 Critical Bug Fixes

### P1 - Blocking Issues

1. **NaïveProxy Authentication Error** - Fixed authentication encoding mismatch causing all clients to fail
2. **Installation Path Inconsistency** - Fixed detection using PATH while systemd uses fixed paths
3. **ShadowTLS Backend Port Conflict** - Fixed port collision between managed backends and new listeners

### P2 - Important Issues

4. **Mieru UDP Detection False Positives** - Fixed protocol-specific connectivity checks
5. **Mixed Protocol Sync Short-circuit** - Fixed Mieru failure preventing other protocols from deploying

## 🛡️ Feature Boundary Enforcement

- Added 4-layer protection for Mieru-only features (traffic stats, quotas, egress rules)
- API returns 400 when attempting to set traffic limits on non-Mieru protocols
- Frontend hides unsupported options with clear messaging

## 🔧 Other Improvements

- IPv6 address handling fixes
- ARM build documentation corrections
- UI messaging for ShadowTLS backend credentials
- Enhanced regression test coverage

## ⚠️ Breaking Changes

None - Fully backward compatible

## 📋 Upgrade Requirements

**Must upgrade both Manager and Agent simultaneously**
- Retry sync for all nodes after upgrade
- Refresh client subscriptions (especially NaïveProxy)

## 🧪 Test Coverage

- ✅ All unit tests passing
- ✅ Regression tests for all 5 fixes
- ✅ Upstream NaïveProxy integration test
- ✅ Cross-compilation verified (amd64/arm64)
- ❌ Production deployment validation pending (requires VPS + domain)

## 📖 Documentation

- Added `docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md` with detailed findings
- Updated README.md with capability boundaries
- Frontend capability messaging improved

---

**Release Date:** 2024-12-XX  
**Git Commit:** TBD  
**Full Release Notes:** `dist/RELEASE_NOTES_v1.5.2.md`
