#!/usr/bin/env bash
# Production Acceptance Test Suite for MeritPilot
#
# Default mode: READ-ONLY
# - No service restart
# - No real certificate issuance
# - No configuration changes
#
# Active test flags (must be explicitly enabled):
# - --with-restart: Test service restart (RISK: brief downtime)
# - --with-acme-issuance: Test ACME certificate issuance (RISK: rate limits, prefers staging)
#
# Exit codes:
# - 0: All checks passed or skipped
# - 1: At least one check failed

# DO NOT use 'set -e' - we want to run all checks even if some fail
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKS_DIR="$SCRIPT_DIR/checks"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
LOG_FILE="${ACCEPTANCE_LOG:-$SCRIPT_DIR/acceptance_${TIMESTAMP}.log}"
RESULT_JSON="${ACCEPTANCE_RESULT:-$SCRIPT_DIR/acceptance_${TIMESTAMP}.json}"

# Feature flags
WITH_RESTART=false
WITH_ACME_ISSUANCE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --with-restart)
            WITH_RESTART=true
            shift
            ;;
        --with-acme-issuance)
            WITH_ACME_ISSUANCE=true
            shift
            ;;
        --log)
            LOG_FILE="$2"
            shift 2
            ;;
        --output)
            RESULT_JSON="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1" >&2
            echo "Usage: $0 [--with-restart] [--with-acme-issuance] [--log FILE] [--output FILE]" >&2
            exit 1
            ;;
    esac
done

export WITH_RESTART WITH_ACME_ISSUANCE

# JSON escape helper for when jq is not available
json_escape() {
    local input="$1"
    local output=""
    local i char hex

    for ((i = 0; i < ${#input}; i++)); do
        char="${input:i:1}"

        # Get hex value to reliably detect special chars
        printf -v hex '%02x' "'$char" 2>/dev/null || hex="00"

        case "$hex" in
            22) output="$output"'\"' ;;   # "
            5c) output="$output"'\\' ;;   # \
            0a) output="$output"'\n' ;;   # newline
            09) output="$output"'\t' ;;   # tab
            0d) output="$output"'\r' ;;   # carriage return
            *)  output="$output$char" ;;
        esac
    done

    printf '%s' "$output"
}

# Test json_escape with problematic input
test_json_escape() {
    local test_input='Error: "failed" with \backslash and
newline'
    local escaped
    escaped="$(json_escape "$test_input")"
    local json_test="{\"error\": \"$escaped\"}"

    # Only validate if python3 is functional
    if command -v python3 >/dev/null 2>&1 && python3 -c "import json" 2>/dev/null; then
        if echo "$json_test" | python3 -m json.tool >/dev/null 2>&1; then
            return 0
        else
            echo "ERROR: json_escape validation failed" >&2
            echo "Input: $test_input" >&2
            echo "Output: $json_test" >&2
            return 1
        fi
    fi
    return 0
}

# Initialize
echo "=== MeritPilot Production Acceptance Test ===" | tee "$LOG_FILE"
echo "Start time: $(date -Iseconds)" | tee -a "$LOG_FILE"
echo "Log file: $LOG_FILE" | tee -a "$LOG_FILE"
echo "Result JSON: $RESULT_JSON" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

if ! test_json_escape; then
    echo "FATAL: json_escape validation failed" | tee -a "$LOG_FILE"
    exit 1
fi

# Check modes
echo "Test modes:" | tee -a "$LOG_FILE"
if $WITH_RESTART; then
    echo "  ⚠️  Service restart tests: ENABLED (may cause brief downtime)" | tee -a "$LOG_FILE"
else
    echo "  ✓  Service restart tests: DISABLED (read-only)" | tee -a "$LOG_FILE"
fi

if $WITH_ACME_ISSUANCE; then
    echo "  ⚠️  ACME issuance tests: ENABLED (may hit rate limits, prefers staging)" | tee -a "$LOG_FILE"
else
    echo "  ✓  ACME issuance tests: DISABLED (read-only)" | tee -a "$LOG_FILE"
fi
echo "" | tee -a "$LOG_FILE"

# Discover check scripts
CHECK_SCRIPTS=(
    "$CHECKS_DIR/01_systemd.sh"
    "$CHECKS_DIR/02_pid_consistency.sh"
    "$CHECKS_DIR/03_mieru.sh"
    "$CHECKS_DIR/04_naiveproxy.sh"
    "$CHECKS_DIR/05_shadowtls.sh"
    "$CHECKS_DIR/06_ipv6.sh"
    "$CHECKS_DIR/07_acme.sh"
)

# Result tracking
declare -A CHECK_RESULTS
declare -A CHECK_MESSAGES
declare -A CHECK_DETAILS
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
SKIPPED_CHECKS=0

# Run each check
for check_script in "${CHECK_SCRIPTS[@]}"; do
    if [[ ! -f "$check_script" ]]; then
        echo "⚠️  Check script not found: $check_script" | tee -a "$LOG_FILE"
        continue
    fi

    check_name="$(basename "$check_script" .sh)"
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

    echo "Running check: $check_name" | tee -a "$LOG_FILE"

    # Run check and capture output
    check_output_file=$(mktemp)
    check_result=0

    # Source the check script and call its main function
    # Each check script should define a 'run_check' function
    (
        # shellcheck source=/dev/null
        source "$check_script"
        run_check
    ) > "$check_output_file" 2>&1 || check_result=$?

    check_output="$(cat "$check_output_file")"
    rm -f "$check_output_file"

    # Parse result (check scripts should output: STATUS: PASS|FAIL|SKIP on the first line)
    status_line="$(echo "$check_output" | head -n1)"
    if [[ "$status_line" =~ ^STATUS:\ (PASS|FAIL|SKIP) ]]; then
        status="${BASH_REMATCH[1]}"
    else
        # Fallback: infer from exit code
        if [[ $check_result -eq 0 ]]; then
            status="PASS"
        elif [[ $check_result -eq 2 ]]; then
            status="SKIP"
        else
            status="FAIL"
        fi
    fi

    CHECK_RESULTS["$check_name"]="$status"
    CHECK_MESSAGES["$check_name"]="$(echo "$check_output" | tail -n +2 | head -n1 || echo "")"
    CHECK_DETAILS["$check_name"]="$check_output"

    case "$status" in
        PASS)
            echo "  ✓ PASS: $check_name" | tee -a "$LOG_FILE"
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            ;;
        FAIL)
            echo "  ✗ FAIL: $check_name" | tee -a "$LOG_FILE"
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
            ;;
        SKIP)
            echo "  - SKIP: $check_name" | tee -a "$LOG_FILE"
            SKIPPED_CHECKS=$((SKIPPED_CHECKS + 1))
            ;;
    esac

    # Write detailed output to log
    echo "$check_output" >> "$LOG_FILE"
    echo "" >> "$LOG_FILE"
done

# Summary
echo "" | tee -a "$LOG_FILE"
echo "=== Summary ===" | tee -a "$LOG_FILE"
echo "Total checks: $TOTAL_CHECKS" | tee -a "$LOG_FILE"
echo "Passed: $PASSED_CHECKS" | tee -a "$LOG_FILE"
echo "Failed: $FAILED_CHECKS" | tee -a "$LOG_FILE"
echo "Skipped: $SKIPPED_CHECKS" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Determine overall status
OVERALL_STATUS="pass"
if [[ $FAILED_CHECKS -gt 0 ]]; then
    OVERALL_STATUS="fail"
elif [[ $PASSED_CHECKS -eq 0 ]] && [[ $FAILED_CHECKS -eq 0 ]]; then
    # All checks were skipped - inconclusive
    OVERALL_STATUS="inconclusive"
    echo "⚠️  INCONCLUSIVE: No checks passed or failed (all skipped)" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"
    echo "Possible causes:" | tee -a "$LOG_FILE"
    echo "  - UnifiedProxy not deployed on this machine" | tee -a "$LOG_FILE"
    echo "  - Auto-discovery found no systemd units matching known prefixes" | tee -a "$LOG_FILE"
    echo "  - Insufficient permissions to query service status" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"
fi

# Generate JSON result
if command -v jq >/dev/null 2>&1; then
    # Use jq to construct JSON safely
    jq -n \
        --arg timestamp "$(date -Iseconds)" \
        --argjson total "$TOTAL_CHECKS" \
        --argjson passed "$PASSED_CHECKS" \
        --argjson failed "$FAILED_CHECKS" \
        --argjson skipped "$SKIPPED_CHECKS" \
        --argjson with_restart "$WITH_RESTART" \
        --argjson with_acme "$WITH_ACME_ISSUANCE" \
        --arg overall "$OVERALL_STATUS" \
        '{
            timestamp: $timestamp,
            summary: {
                total: $total,
                passed: $passed,
                failed: $failed,
                skipped: $skipped
            },
            overall: $overall,
            modes: {
                with_restart: $with_restart,
                with_acme_issuance: $with_acme
            },
            checks: []
        }' > "$RESULT_JSON"

    # Add each check result
    for check_name in "${!CHECK_RESULTS[@]}"; do
        status="${CHECK_RESULTS[$check_name]}"
        message="${CHECK_MESSAGES[$check_name]}"
        details="${CHECK_DETAILS[$check_name]}"

        jq --arg name "$check_name" \
           --arg status "$status" \
           --arg message "$message" \
           --arg details "$details" \
           '.checks += [{
               name: $name,
               status: $status,
               message: $message,
               details: $details
           }]' "$RESULT_JSON" > "$RESULT_JSON.tmp" && mv "$RESULT_JSON.tmp" "$RESULT_JSON"
    done
else
    # Fallback: manual JSON construction with proper escaping
    {
        echo "{"
        echo "  \"timestamp\": \"$(date -Iseconds)\","
        echo "  \"summary\": {"
        echo "    \"total\": $TOTAL_CHECKS,"
        echo "    \"passed\": $PASSED_CHECKS,"
        echo "    \"failed\": $FAILED_CHECKS,"
        echo "    \"skipped\": $SKIPPED_CHECKS"
        echo "  },"
        echo "  \"overall\": \"$OVERALL_STATUS\","
        echo "  \"modes\": {"
        echo "    \"with_restart\": $WITH_RESTART,"
        echo "    \"with_acme_issuance\": $WITH_ACME_ISSUANCE"
        echo "  },"
        echo "  \"checks\": ["

        first=true
        for check_name in "${!CHECK_RESULTS[@]}"; do
            if ! $first; then
                echo ","
            fi
            first=false

            status="${CHECK_RESULTS[$check_name]}"
            message="$(json_escape "${CHECK_MESSAGES[$check_name]}")"
            details="$(json_escape "${CHECK_DETAILS[$check_name]}")"

            echo "    {"
            echo "      \"name\": \"$(json_escape "$check_name")\","
            echo "      \"status\": \"$status\","
            echo "      \"message\": \"$message\","
            echo "      \"details\": \"$details\""
            echo -n "    }"
        done
        echo ""
        echo "  ]"
        echo "}"
    } > "$RESULT_JSON"
fi

echo "Results written to: $RESULT_JSON" | tee -a "$LOG_FILE"

# Validate JSON
json_valid=false
if command -v jq >/dev/null 2>&1; then
    if jq empty "$RESULT_JSON" >/dev/null 2>&1; then
        json_valid=true
    fi
elif command -v node >/dev/null 2>&1; then
    # Use eval to properly handle the path in the node command
    if node -e "JSON.parse(require('fs').readFileSync(process.argv[1], 'utf8'))" "$RESULT_JSON" >/dev/null 2>&1; then
        json_valid=true
    fi
elif command -v python3 >/dev/null 2>&1 && python3 -c "import json" 2>/dev/null; then
    if python3 -m json.tool "$RESULT_JSON" >/dev/null 2>&1; then
        json_valid=true
    fi
fi

if $json_valid; then
    echo "✓ JSON validation: PASS" | tee -a "$LOG_FILE"
else
    echo "✗ JSON validation: FAIL (or no validator available)" | tee -a "$LOG_FILE"
    # Don't count as failure if no validator is available
    if command -v jq >/dev/null 2>&1 || command -v node >/dev/null 2>&1 || (command -v python3 >/dev/null 2>&1 && python3 -c "import json" 2>/dev/null); then
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    fi
fi

# Exit code
if [[ $FAILED_CHECKS -gt 0 ]]; then
    echo "" | tee -a "$LOG_FILE"
    echo "✗ Acceptance test FAILED ($FAILED_CHECKS failures)" | tee -a "$LOG_FILE"
    exit 1
elif [[ "$OVERALL_STATUS" == "inconclusive" ]]; then
    echo "⚠️  Acceptance test INCONCLUSIVE (no checks produced pass/fail results)" | tee -a "$LOG_FILE"
    exit 2
else
    echo "" | tee -a "$LOG_FILE"
    echo "✓ Acceptance test PASSED" | tee -a "$LOG_FILE"
    exit 0
fi
