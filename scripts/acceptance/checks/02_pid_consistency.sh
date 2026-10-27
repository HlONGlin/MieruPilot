#!/usr/bin/env bash
# Check 02: PID Consistency
# Verifies that the actual listening process PID matches the systemd unit MainPID
# This validates the preflight port occupation logic from Task 1

run_check() {
    local systemd_dir="/etc/systemd/system"
    local status="PASS"
    local failures=()
    local details=()

    # Known unit prefixes and their port mapping
    local prefixes=("merit-mita-" "merit-naive-" "naive-proxy-" "merit-anytls-" "merit-shadowtls-" "shadowsocks-backend-")

    # Discover units
    local units=()
    if [[ -d "$systemd_dir" ]]; then
        for prefix in "${prefixes[@]}"; do
            while IFS= read -r unit_file; do
                units+=("$(basename "$unit_file" .service)")
            done < <(find "$systemd_dir" -maxdepth 1 -name "${prefix}*.service" -type f 2>/dev/null || true)
        done
    fi

    if [[ ${#units[@]} -eq 0 ]]; then
        echo "STATUS: SKIP"
        echo "No MeritPilot units found"
        return 2
    fi

    details+=("Checking PID consistency for ${#units[@]} units")
    details+=("")

    # Check each unit
    for unit in "${units[@]}"; do
        local main_pid listen_port actual_pid

        # Get systemd MainPID
        main_pid="$(systemctl show "$unit" -p MainPID --value 2>/dev/null || echo "0")"

        # Extract port from unit file comment or environment
        # Format: #MeritPort=<port>
        local unit_file="$systemd_dir/$unit.service"
        if [[ -f "$unit_file" ]]; then
            listen_port="$(grep -oP '^#MeritPort=\K\d+' "$unit_file" 2>/dev/null || echo "")"
        fi

        details+=("Unit: $unit")
        details+=("  systemd MainPID: $main_pid")
        details+=("  declared port: ${listen_port:-unknown}")

        if [[ "$main_pid" == "0" || -z "$main_pid" ]]; then
            failures+=("$unit: MainPID is missing or zero")
            status="FAIL"
            details+=("  actual PID: N/A (no MainPID)")
            details+=("  verdict: FAIL (no MainPID)")
            details+=("")
            continue
        fi

        if [[ -z "$listen_port" ]]; then
            details+=("  actual PID: SKIP (no port declaration)")
            details+=("  verdict: SKIP")
            details+=("")
            continue
        fi

        # Find process actually listening on the port
        # Use ss (socket statistics) or netstat
        if command -v ss >/dev/null 2>&1; then
            actual_pid="$(ss -tlnp 2>/dev/null | grep -E ":${listen_port}\s" | grep -oP 'pid=\K\d+' | head -n1 || echo "")"
            if [[ -z "$actual_pid" ]]; then
                actual_pid="$(ss -ulnp 2>/dev/null | grep -E ":${listen_port}\s" | grep -oP 'pid=\K\d+' | head -n1 || echo "")"
            fi
        elif command -v netstat >/dev/null 2>&1; then
            actual_pid="$(netstat -tlnp 2>/dev/null | grep -E ":${listen_port}\s" | awk '{print $7}' | cut -d/ -f1 | head -n1 || echo "")"
            if [[ -z "$actual_pid" ]]; then
                actual_pid="$(netstat -ulnp 2>/dev/null | grep -E ":${listen_port}\s" | awk '{print $7}' | cut -d/ -f1 | head -n1 || echo "")"
            fi
        else
            details+=("  actual PID: SKIP (no ss/netstat)")
            details+=("  verdict: SKIP")
            details+=("")
            continue
        fi

        details+=("  actual listening PID: ${actual_pid:-none}")

        if [[ -z "$actual_pid" ]]; then
            failures+=("$unit (port $listen_port): no process listening")
            status="FAIL"
            details+=("  verdict: FAIL (no listener)")
        elif [[ "$actual_pid" != "$main_pid" ]]; then
            failures+=("$unit (port $listen_port): PID mismatch (systemd=$main_pid, actual=$actual_pid)")
            status="FAIL"
            details+=("  verdict: FAIL (PID mismatch: systemd=$main_pid vs actual=$actual_pid)")
        else
            details+=("  verdict: PASS (PID matches)")
        fi

        details+=("")
    done

    # Output
    if [[ "$status" == "PASS" ]]; then
        echo "STATUS: PASS"
        echo "All units have consistent PIDs (systemd MainPID == actual listener PID)"
    else
        echo "STATUS: FAIL"
        echo "PID consistency check failed: ${#failures[@]} mismatches found"
    fi

    printf '%s\n' "${details[@]}"
    if [[ ${#failures[@]} -gt 0 ]]; then
        echo ""
        echo "Failures:"
        printf '%s\n' "${failures[@]}"
    fi

    [[ "$status" == "PASS" ]] && return 0 || return 1
}
