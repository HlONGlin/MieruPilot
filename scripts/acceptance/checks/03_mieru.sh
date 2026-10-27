#!/usr/bin/env bash
# Check 03: Mieru
# Verifies Mieru (merit-mita-*) TCP+UDP listeners are in place
# End-to-end test skipped if no client available

run_check() {
    local systemd_dir="/etc/systemd/system"
    local status="PASS"
    local failures=()
    local details=()

    # Discover Mieru units
    local units=()
    if [[ -d "$systemd_dir" ]]; then
        while IFS= read -r unit_file; do
            units+=("$(basename "$unit_file" .service)")
        done < <(find "$systemd_dir" -maxdepth 1 -name "merit-mita-*.service" -type f 2>/dev/null || true)
    fi

    if [[ ${#units[@]} -eq 0 ]]; then
        echo "STATUS: SKIP"
        echo "No Mieru units found"
        return 2
    fi

    details+=("Found ${#units[@]} Mieru units")
    details+=("")

    # Check each unit
    for unit in "${units[@]}"; do
        local main_pid listen_port tcp_found udp_found

        main_pid="$(systemctl show "$unit" -p MainPID --value 2>/dev/null || echo "0")"

        # Extract port
        local unit_file="$systemd_dir/$unit.service"
        if [[ -f "$unit_file" ]]; then
            listen_port="$(grep -oP '^#MeritPort=\K\d+' "$unit_file" 2>/dev/null || echo "")"
        fi

        details+=("Unit: $unit")
        details+=("  MainPID: $main_pid")
        details+=("  port: ${listen_port:-unknown}")

        if [[ -z "$listen_port" ]]; then
            details+=("  verdict: SKIP (no port declaration)")
            details+=("")
            continue
        fi

        # Check TCP listener
        tcp_found=false
        if command -v ss >/dev/null 2>&1; then
            if ss -tlnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                tcp_found=true
            fi
        elif command -v netstat >/dev/null 2>&1; then
            if netstat -tlnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                tcp_found=true
            fi
        fi

        # Check UDP listener
        udp_found=false
        if command -v ss >/dev/null 2>&1; then
            if ss -ulnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                udp_found=true
            fi
        elif command -v netstat >/dev/null 2>&1; then
            if netstat -ulnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                udp_found=true
            fi
        fi

        details+=("  TCP listener: $($tcp_found && echo "YES" || echo "NO")")
        details+=("  UDP listener: $($udp_found && echo "YES" || echo "NO")")

        # Mieru should have both TCP and UDP
        if ! $tcp_found; then
            failures+=("$unit (port $listen_port): TCP listener not found")
            status="FAIL"
        fi

        if ! $udp_found; then
            failures+=("$unit (port $listen_port): UDP listener not found")
            status="FAIL"
        fi

        # End-to-end test (only if client available)
        # Check for mieru client binary or MIERU_CLIENT_CONFIG env var
        if command -v mieru >/dev/null 2>&1 || [[ -n "${MIERU_CLIENT_CONFIG:-}" ]]; then
            details+=("  end-to-end: SKIP (client available but not implemented yet)")
        else
            details+=("  end-to-end: SKIP (no client)")
        fi

        details+=("")
    done

    # Output
    if [[ "$status" == "PASS" ]]; then
        echo "STATUS: PASS"
        echo "All Mieru units have TCP+UDP listeners in place"
    else
        echo "STATUS: FAIL"
        echo "Mieru check failed: ${#failures[@]} issues found"
    fi

    printf '%s\n' "${details[@]}"
    if [[ ${#failures[@]} -gt 0 ]]; then
        echo ""
        echo "Failures:"
        printf '%s\n' "${failures[@]}"
    fi

    [[ "$status" == "PASS" ]] && return 0 || return 1
}
