#!/usr/bin/env bash
# Check 01: Systemd units status
# Verifies that all deployed MeritPilot service units are active and enabled

run_check() {
    local systemd_dir="/etc/systemd/system"
    local status="PASS"
    local failures=()
    local details=()

    # Known unit prefixes (must sync with Go code: internal/agent/instances_proxy.go:proxyServiceName)
    # - merit-mita-*: Mieru protocol
    # - merit-naive-* / naive-proxy-*: NaïveProxy (new/legacy)
    # - merit-anytls-*: AnyTLS protocol
    # - merit-shadowtls-*: ShadowTLS protocol
    # - shadowsocks-backend-*: Shadowsocks backend for ShadowTLS
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
        echo "No MeritPilot units found in $systemd_dir"
        return 2
    fi

    details+=("Found ${#units[@]} units:")
    for unit in "${units[@]}"; do
        details+=("  - $unit")
    done
    details+=("")

    # Check each unit
    for unit in "${units[@]}"; do
        local is_active is_enabled main_pid

        is_active="$(systemctl is-active "$unit" 2>/dev/null || echo "unknown")"
        is_enabled="$(systemctl is-enabled "$unit" 2>/dev/null || echo "unknown")"
        main_pid="$(systemctl show "$unit" -p MainPID --value 2>/dev/null || echo "0")"

        details+=("Unit: $unit")
        details+=("  is-active: $is_active")
        details+=("  is-enabled: $is_enabled")
        details+=("  MainPID: $main_pid")

        if [[ "$is_active" != "active" ]]; then
            failures+=("$unit: not active (status: $is_active)")
            status="FAIL"
        fi

        if [[ "$is_enabled" == "disabled" ]]; then
            failures+=("$unit: not enabled (will not auto-start on boot)")
            status="FAIL"
        fi

        if [[ "$main_pid" == "0" || -z "$main_pid" ]]; then
            failures+=("$unit: MainPID missing or zero")
            status="FAIL"
        fi

        details+=("")
    done

    # Output
    if [[ "$status" == "PASS" ]]; then
        echo "STATUS: PASS"
        echo "All ${#units[@]} units are active, enabled, and have valid MainPID"
    else
        echo "STATUS: FAIL"
        echo "Systemd unit check failed: ${#failures[@]} issues found"
    fi

    printf '%s\n' "${details[@]}"
    if [[ ${#failures[@]} -gt 0 ]]; then
        echo ""
        echo "Failures:"
        printf '%s\n' "${failures[@]}"
    fi

    [[ "$status" == "PASS" ]] && return 0 || return 1
}
