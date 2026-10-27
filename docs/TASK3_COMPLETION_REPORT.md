# Task 3 Completion Report: Production Acceptance Test Scripts

## Commit Information
- **Commit Hash**: `6a99a76`
- **Commit Title**: feat: add production acceptance test scripts
- **Branch**: upgrade/v1.6.0
- **Date**: 2026-10-27

---

## Deliverables

### 1. Script Files Created

```
scripts/acceptance/
├── README.md                         # Usage guide, safety warnings, result interpretation
├── run-acceptance.sh                 # Main entry point
└── checks/
    ├── 01_systemd.sh                 # Systemd unit status (active/enabled/MainPID)
    ├── 02_pid_consistency.sh         # PID consistency (socket owner vs systemd MainPID)
    ├── 03_mieru.sh                   # Mieru TCP/UDP listeners, optional e2e
    ├── 04_naiveproxy.sh              # NaïveProxy Caddy process, cert expiry, ACME events
    ├── 05_shadowtls.sh               # ShadowTLS + ss-backend dual-unit validation
    ├── 06_ipv6.sh                    # IPv6 listener checks
    └── 07_acme.sh                    # ACME DNS/cert/history (read-only by default)
```

**Total**: 9 files, 1745 lines added

---

## Safety Design

### Zero Side-Effects by Default

The acceptance script runs in **read-only mode** by default:

- ❌ **No service restarts** unless `--with-restart` flag is passed
- ❌ **No ACME certificate issuance** unless `--with-acme-issuance` flag is passed
- ❌ **No configuration changes** at any time
- ✅ Safe to run on production VPS without disruption

### Explicit Destructive Test Flags

```bash
# Default: read-only checks only
./scripts/acceptance/run-acceptance.sh

# Enable service restart tests
./scripts/acceptance/run-acceptance.sh --with-restart

# Enable ACME issuance tests (prefers staging CA)
./scripts/acceptance/run-acceptance.sh --with-acme-issuance

# Enable both
./scripts/acceptance/run-acceptance.sh --with-restart --with-acme-issuance
```

**README Warning** (scripts/acceptance/README.md lines 32-49):
> ⚠️ **--with-restart**: 会实际重启服务,生产环境慎用
> ⚠️ **--with-acme-issuance**: 会触发真实证书签发,可能消耗速率限制,优先使用 staging

---

## Test Coverage

### 1. Systemd Unit Status (01_systemd.sh)
- Check `is-active`, `is-enabled` status
- Verify MainPID exists
- Auto-discover units by known prefixes (see below)

### 2. PID Consistency (02_pid_consistency.sh)
**Critical validation from Task 1/2 audit**:
- For each deployed port, verify socket owner PID == systemd MainPID
- FAIL if mismatch → indicates preflight port binding logic issue
- This validates that the port occupation prevention mechanism works on real systems

### 3. Mieru (03_mieru.sh)
- TCP + UDP listeners present
- End-to-end test if client config available (else SKIP)
- Listener checks always run regardless

### 4. NaïveProxy (04_naiveproxy.sh)
- Caddy process running
- Certificate expiry:
  - < 30 days: WARNING (not FAIL)
  - Expired: FAIL
- ACME journal events (last 7 days):
  - Check for failure events in systemd journal

### 5. ShadowTLS (05_shadowtls.sh)
- Both shadow-tls and ss-backend units present
- Both listener segments present

### 6. IPv6 (06_ipv6.sh)
- SKIP if no IPv6 addresses found
- Otherwise check IPv6 listeners for each service

### 7. ACME (07_acme.sh - read-only by default)
- DNS resolution check
- Certificate presence and validity
- ACME history from journal
- **No actual issuance** unless `--with-acme-issuance` passed

---

## Auto-Discovery Logic

### Unit Prefix Mapping

Scripts auto-discover systemd units using prefixes synchronized with Go code:

| Protocol | Systemd Unit Prefixes | Go Code Reference |
|----------|----------------------|-------------------|
| **Mieru** | `merit-mita-*` | `internal/agent/instances_proxy.go:50-61` |
| **NaïveProxy** | `merit-naive-*`, `naive-proxy-*` (legacy) | Same as above |
| **AnyTLS** | `merit-anytls-*` | Same as above |
| **ShadowTLS** | `merit-shadowtls-*` | Same as above |
| **Shadowsocks backend** | `shadowsocks-backend-*` | `internal/agent/install_shadowtls.go` |

**Synchronization Note** (from scripts/acceptance/README.md):
> 这些前缀需与 Go 侧保持同步,若 `internal/agent/instances_proxy.go:proxyServiceName` 或 ShadowTLS 后端逻辑修改,需同步更新 shell 脚本。

Environment variables can supplement/override auto-discovered units:
```bash
export MERIT_MIERU_UNITS="merit-mita-1234.service merit-mita-5678.service"
export MERIT_NAIVE_UNITS="merit-naive-8443.service"
# ... etc
```

---

## Execution Behavior

### Failure Handling

**No `set -e`** in main script:
- Each check runs to completion even if others fail
- Results tracked as PASS/FAIL/SKIP
- Final exit code:
  - `0` if no FAILs (SKIP doesn't count as failure)
  - `1` if any check FAIL

### Output Format

**Two output files per run**:
1. **Log file**: `acceptance_YYYYMMDD_HHMMSS.log`
   - Human-readable, timestamped
   - Includes all check output
   
2. **JSON summary**: `acceptance_YYYYMMDD_HHMMSS.json`
   - Machine-readable
   - Properly escaped (quotes, backslashes, newlines)
   - Structure:
     ```json
     {
       "timestamp": "2026-10-27T17:02:46+08:00",
       "summary": {
         "total": 7,
         "passed": 0,
         "failed": 0,
         "skipped": 7
       },
       "modes": {
         "with_restart": false,
         "with_acme_issuance": false
       },
       "checks": [
         {
           "name": "03_mieru",
           "status": "SKIP",
           "message": "No Mieru units found",
           "details": "..."
         }
       ]
     }
     ```

---

## Validation Results

### Syntax Check
```bash
bash -n scripts/acceptance/run-acceptance.sh        # ✓ PASS
bash -n scripts/acceptance/checks/*.sh              # ✓ PASS (all 7 checks)
```

**shellcheck**: Not available on test machine, but `bash -n` passed for all scripts

### Local Test Execution

**Environment**: Windows Git Bash, no services deployed

**Results**:
```
Total checks: 7
Passed: 0
Failed: 0
Skipped: 7
```

**Reason for all SKIP**: Expected behavior on development machine without services

**JSON Validation**: PASS (validated with node.js)

### JSON Escaping Test

**Helper function**: `json_escape()` in run-acceptance.sh

**Test cases verified**:
- ✓ Double quotes: `"test"` → `\"test\"`
- ✓ Backslashes: `C:\path\to\file` → `C:\\path\\to\\file`
- ✓ Newlines: Multi-line error messages → `\n` literals
- ✓ Control characters: Properly escaped

**Validation method**:
```bash
# Uses first available validator:
1. jq empty (preferred)
2. node -e "JSON.parse(...)"
3. python3 -m json.tool
```

---

## Integration with Task 2

### ACME Status Sample (Supplement)

**File created**: `docs/examples/acme_status_failed.json`

This provides the ACME failure scenario referenced in Task 2:

```json
{
  "autoACME": true,
  "hasCert": false,
  "domain": "failed.example.com",
  "expiryLevel": "unknown",
  "expiryText": "等待 ACME 签发",
  "acmeStatus": {
    "status": "failed",
    "lastEvent": "2026-10-27T16:32:57.9159571+08:00",
    "lastError": "acme: authorization error for failed.example.com: 403 urn:ietf:params:acme:error:unauthorized: Invalid response from http://failed.example.com/.well-known/acme-challenge/xyz123: 404 Not Found"
  }
}
```

**Source**: `docs/TASK2_COMPLETION_REPORT.md` lines 228-241

---

## Usage Examples

### Safe Production Run (Read-Only)

```bash
# On production VPS
cd /opt/merit-pilot  # or wherever project is deployed
./scripts/acceptance/run-acceptance.sh

# Check results
cat scripts/acceptance/acceptance_*.log | tail -20
cat scripts/acceptance/acceptance_*.json | jq '.summary'
```

**Expected**: Most checks PASS, some SKIP if protocols not deployed

### Development/CI Run with Destructive Tests

```bash
# On staging server with full test authority
./scripts/acceptance/run-acceptance.sh \
  --with-restart \
  --with-acme-issuance

# Exit code check
if [ $? -eq 0 ]; then
  echo "All checks passed or skipped"
else
  echo "Some checks failed, review logs"
fi
```

### Single Protocol Validation

```bash
# Only check Mieru
bash scripts/acceptance/checks/03_mieru.sh

# Only check certificate expiry
bash scripts/acceptance/checks/04_naiveproxy.sh
```

---

## README Documentation

**File**: `scripts/acceptance/README.md`

**Sections**:
1. **Purpose**: What acceptance tests validate
2. **Usage**: Command-line examples
3. **Safety Warnings**: Clear documentation of `--with-restart` and `--with-acme-issuance` risks
4. **Result Interpretation**: How to read log and JSON files
5. **Auto-Discovery**: How units are found
6. **Troubleshooting**: Common issues and solutions

**Key Safety Section** (lines 32-49):
> ⚠️ **默认只读**: 主脚本不会重启服务、不会触发证书签发、不会修改配置
> 
> ⚠️ **主动测试开关**:
> - `--with-restart`: 会实际重启服务,生产环境慎用
> - `--with-acme-issuance`: 会触发真实证书签发,优先使用 staging CA 但仍可能消耗速率限制

---

## Task Requirements Checklist

### ✅ Core Requirements

- [x] Main entry point: `scripts/acceptance/run-acceptance.sh`
- [x] Modular checks: `scripts/acceptance/checks/*.sh`
- [x] README with usage, warnings, and result interpretation
- [x] Default read-only mode (no side effects)
- [x] Explicit `--with-restart` and `--with-acme-issuance` flags
- [x] Warnings in README about flag risks
- [x] No `set -e` in main script
- [x] Continue on individual check failure
- [x] Summary at end with exit code
- [x] JSON output with proper escaping
- [x] Auto-discovery of units/ports
- [x] Known prefix mapping documented and synchronized with Go code
- [x] Environment variable override support

### ✅ Validation

- [x] `bash -n` all scripts: PASS
- [x] shellcheck (if available): N/A on test machine
- [x] Local test run: Complete, 7/7 checks ran, JSON valid
- [x] JSON validator test: PASS (node.js)
- [x] JSON escaping test: PASS (quotes, backslashes, newlines)

### ✅ Deliverables

- [x] Commit hash: `6a99a76`
- [x] File list: 9 files in `scripts/acceptance/`
- [x] Local test summary: 7 checks, 0 pass / 0 fail / 7 skip (expected)
- [x] JSON validation result: PASS
- [x] ACME failure sample JSON: `docs/examples/acme_status_failed.json`
- [x] README safety documentation: Complete

---

## Next Steps (Not Part of This Task)

1. Run acceptance tests on real Linux VPS with deployed services
2. Validate PID consistency check catches real port conflicts
3. Test `--with-restart` on non-production environment
4. Test `--with-acme-issuance` with staging CA
5. Integrate into CI/CD pipeline
6. Update version numbers and CHANGELOG (separate task)
7. Release process (separate task)

---

## Notes

- **PID Consistency Check**: This is the key validation for Task 1 residual risks. It verifies on real systems that the preflight port binding logic correctly prevents deployments when ports are occupied.
  
- **Unit Prefix Synchronization**: The shell scripts replicate the mapping from `internal/agent/instances_proxy.go:proxyServiceName`. If that Go function changes, scripts must be updated accordingly. This is documented in the README.

- **JSON Escaping**: The `json_escape()` function handles all special characters properly. Tested with multi-line error messages containing quotes and backslashes.

- **Windows Git Bash Compatibility**: Scripts were developed and tested on Windows Git Bash, but target Linux systemd environments. Path handling and command availability checked appropriately.
