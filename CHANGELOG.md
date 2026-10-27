# Changelog

## [1.6.0] - 2026-10-27

### Added

- **Unified Preflight Dependency Checker**: Protocol and port-scoped dependency validation that fails early with actionable install commands for Debian/Ubuntu, RHEL/CentOS, and Arch Linux
- **Certificate Lifecycle Visibility**: Expiry countdown with three-tier warnings (60d/30d/7d), ACME status & failure reason display, and per-port staging toggle for Let's Encrypt testing
- **Production Acceptance Test Suite**: Read-only by default with explicit switches for active tests (`--mieru-tcp`, `--naiveproxy-http`), three-state conclusion (PASS/FAIL/INCONCLUSIVE), and structured exit codes (0/1/2) with JSON output

### Changed

- **Port Conflict Detection**: Now uses instance identity (systemd unit MainPID) instead of process name matching; unknown occupants always block deployment (replaces previous warning-only behavior)

### Known Limitations

- Traffic statistics and monthly quotas are only available for Mieru protocol
- Automatic installation supports Linux amd64/arm64 only
- NaïveProxy self-installation is amd64-only (arm64 requires manual setup)
- UDP support: Mieru and AnyTLS (UoT) only; ShadowTLS does not support UDP
- ShadowTLS requires a handshake backend service to be configured

---

## [1.5.2] - See CHANGELOG_v1.5.2.md

Critical bug fixes for NaïveProxy authentication, installation path detection, ShadowTLS port conflicts, and protocol sync logic.
