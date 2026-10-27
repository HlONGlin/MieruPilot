#!/usr/bin/env bash
# Check 05: ShadowTLS
# Verifies shadow-tls and its shadowsocks-backend units are in place with listeners

run_check() {
    local systemd_dir="/etc/systemd/system"
    local status="PASS"
    local failures=()
    local details=()

    # Discover ShadowTLS units
    local stls_units=()
    if [[ -d "$systemd_dir" ]]; then
        while IFS= read -r unit_file; do
            stls_units+=("$(basename "$unit_file" .service)")
        done < <(find "$systemd_dir" -maxdepth 1 -name "merit-shadowtls-*.service" -type f 2>/dev/null || true)
    fi

    if [[ ${#stls_units[@]} -eq 0 ]]; then
        echo "STATUS: SKIP"
        echo "No ShadowTLS units found"
        return 2
    fi

    details+=("Found ${#stls_units[@]} ShadowTLS units")
    details+=("")

    # Check each ShadowTLS unit and its backend
    for unit in "${stls_units[@]}"; do
        local main_pid listen_port port_id backend_unit backend_port

        main_pid="$(systemctl show "$unit" -p MainPID --value 2>/dev/null || echo "0")"

        # Extract port
        local unit_file="$systemd_dir/$unit.service"
        if [[ -f "$unit_file" ]]; then
            listen_port="$(grep -oP '^#MeritPort=\K\d+' "$unit_file" 2>/dev/null || echo "")"
        fi

        # Extract port ID
        if [[ "$unit" =~ merit-shadowtls-(.+) ]]; then
            port_id="${BASH_REMATCH[1]}"
        fi

        details+=("ShadowTLS Unit: $unit")
        details+=("  MainPID: $main_pid")
        details+=("  front port: ${listen_port:-unknown}")

        # Check ShadowTLS listener
        local stls_listening=false
        if [[ -n "$listen_port" ]]; then
            if command -v ss >/dev/null 2>&1; then
                if ss -tlnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                    stls_listening=true
                fi
            elif command -v netstat >/dev/null 2>&1; then
                if netstat -tlnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                    stls_listening=true
                fi
            fi

            details+=("  front listener: $($stls_listening && echo "YES" || echo "NO")")

            if ! $stls_listening; then
                failures+=("$unit (port $listen_port): front listener not found")
                status="FAIL"
            fi
        else
            details+=("  front listener: SKIP (no port declaration)")
        fi

        # Find corresponding backend unit
        backend_unit="shadowsocks-backend-${port_id}"
        local backend_file="$systemd_dir/$backend_unit.service"

        if [[ ! -f "$backend_file" ]]; then
            failures+=("$unit: backend unit $backend_unit not found")
            status="FAIL"
            details+=("  backend unit: NOT FOUND")
            details+=("")
            continue
        fi

        details+=("  backend unit: $backend_unit")

        # Check backend status
        local backend_pid backend_active
        backend_pid="$(systemctl show "$backend_unit" -p MainPID --value 2>/dev/null || echo "0")"
        backend_active="$(systemctl is-active "$backend_unit" 2>/dev/null || echo "unknown")"

        details+=("    MainPID: $backend_pid")
        details+=("    is-active: $backend_active")

        if [[ "$backend_active" != "active" ]]; then
            failures+=("$unit: backend $backend_unit not active (status: $backend_active)")
            status="FAIL"
        fi

        # Extract backend port from unit file
        backend_port="$(grep -oP '^#MeritPort=\K\d+' "$backend_file" 2>/dev/null || echo "")"
        if [[ -z "$backend_port" ]]; then
            # Try to extract from ExecStart (shadowsocks -s 127.0.0.1:PORT)
            backend_port="$(grep -oP 'ExecStart=.* -s 127\.0\.0\.1:\K\d+' "$backend_file" 2>/dev/null || echo "")"
        fi

        details+=("    backend port: ${backend_port:-unknown}")

        # Check backend listener (should be on 127.0.0.1)
        if [[ -n "$backend_port" ]]; then
            local backend_listening=false
            if command -v ss >/dev/null 2>&1; then
                if ss -tlnp 2>/dev/null | grep -qE "127\.0\.0\.1:${backend_port}\s"; then
                    backend_listening=true
                fi
            elif command -v netstat >/dev/null 2>&1; then
                if netstat -tlnp 2>/dev/null | grep -qE "127\.0\.0\.1:${backend_port}\s"; then
                    backend_listening=true
                fi
            fi

            details+=("    backend listener: $($backend_listening && echo "YES" || echo "NO")")

            if ! $backend_listening; then
                failures+=("$unit: backend listener not found on 127.0.0.1:$backend_port")
                status="FAIL"
            fi
        else
            details+=("    backend listener: SKIP (port unknown)")
        fi

        details+=("")
    done

    # Output
    if [[ "$status" == "PASS" ]]; then
        echo "STATUS: PASS"
        echo "All ShadowTLS units and backends are in place with listeners"
    else
        echo "STATUS: FAIL"
        echo "ShadowTLS check failed: ${#failures[@]} issues found"
    fi

    printf '%s\n' "${details[@]}"
    if [[ ${#failures[@]} -gt 0 ]]; then
        echo ""
        echo "Failures:"
        printf '%s\n' "${failures[@]}"
    fi

    [[ "$status" == "PASS" ]] && return 0 || return 1
}
