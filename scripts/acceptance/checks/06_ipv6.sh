#!/usr/bin/env bash
# Check 06: IPv6
# Verifies IPv6 listeners for services when IPv6 is available

run_check() {
    local systemd_dir="/etc/systemd/system"
    local status="PASS"
    local failures=()
    local details=()

    # Check if IPv6 is available
    local has_ipv6=false
    if ip -6 addr show 2>/dev/null | grep -q 'scope global'; then
        has_ipv6=true
        details+=("IPv6 availability: YES")
    else
        details+=("IPv6 availability: NO")
    fi

    if ! $has_ipv6; then
        echo "STATUS: SKIP"
        echo "No IPv6 addresses found on this system"
        printf '%s\n' "${details[@]}"
        return 2
    fi

    details+=("")

    # Discover all MeritPilot units
    local prefixes=("merit-mita-" "merit-naive-" "naive-proxy-" "merit-anytls-" "merit-shadowtls-")
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
        printf '%s\n' "${details[@]}"
        return 2
    fi

    details+=("Found ${#units[@]} units to check for IPv6 listeners")
    details+=("")

    # Check each unit for IPv6 listeners
    for unit in "${units[@]}"; do
        local listen_port ipv6_listening

        # Extract port
        local unit_file="$systemd_dir/$unit.service"
        if [[ -f "$unit_file" ]]; then
            listen_port="$(grep -oP '^#MeritPort=\K\d+' "$unit_file" 2>/dev/null || echo "")"
        fi

        details+=("Unit: $unit")
        details+=("  port: ${listen_port:-unknown}")

        if [[ -z "$listen_port" ]]; then
            details+=("  IPv6 listener: SKIP (no port declaration)")
            details+=("")
            continue
        fi

        # Check for IPv6 listener (both TCP and UDP)
        ipv6_listening=false
        local listen_type=""

        if command -v ss >/dev/null 2>&1; then
            if ss -6tlnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                ipv6_listening=true
                listen_type="TCP"
            elif ss -6ulnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                ipv6_listening=true
                listen_type="UDP"
            fi
        elif command -v netstat >/dev/null 2>&1; then
            if netstat -6tlnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                ipv6_listening=true
                listen_type="TCP"
            elif netstat -6ulnp 2>/dev/null | grep -qE ":${listen_port}\s"; then
                ipv6_listening=true
                listen_type="UDP"
            fi
        fi

        details+=("  IPv6 listener: $($ipv6_listening && echo "YES ($listen_type)" || echo "NO")")

        # Determine expected behavior per protocol
        # - Mieru: should have IPv6 if available
        # - NaïveProxy: should have IPv6 if available
        # - AnyTLS: currently binds to 0.0.0.0 (IPv4 only per Task 1 audit)
        # - ShadowTLS: dual-stack support depends on bindv6only kernel param

        if [[ "$unit" =~ merit-mita- ]]; then
            # Mieru: expect IPv6
            if ! $ipv6_listening; then
                failures+=("$unit (port $listen_port): expected IPv6 listener (Mieru)")
                status="FAIL"
            fi
        elif [[ "$unit" =~ (merit-naive-|naive-proxy-) ]]; then
            # NaïveProxy: expect IPv6
            if ! $ipv6_listening; then
                failures+=("$unit (port $listen_port): expected IPv6 listener (NaïveProxy)")
                status="FAIL"
            fi
        elif [[ "$unit" =~ merit-anytls- ]]; then
            # AnyTLS: currently IPv4 only (0.0.0.0), not a failure
            if ! $ipv6_listening; then
                details+=("  note: AnyTLS currently binds to 0.0.0.0 (IPv4 only)")
            fi
        elif [[ "$unit" =~ merit-shadowtls- ]]; then
            # ShadowTLS: dual-stack depends on system config
            if ! $ipv6_listening; then
                details+=("  note: ShadowTLS IPv6 depends on bindv6only kernel param")
            fi
        fi

        details+=("")
    done

    # Output
    if [[ "$status" == "PASS" ]]; then
        echo "STATUS: PASS"
        echo "IPv6 listeners found for all expected services"
    else
        echo "STATUS: FAIL"
        echo "IPv6 check failed: ${#failures[@]} missing listeners"
    fi

    printf '%s\n' "${details[@]}"
    if [[ ${#failures[@]} -gt 0 ]]; then
        echo ""
        echo "Failures:"
        printf '%s\n' "${failures[@]}"
    fi

    [[ "$status" == "PASS" ]] && return 0 || return 1
}
