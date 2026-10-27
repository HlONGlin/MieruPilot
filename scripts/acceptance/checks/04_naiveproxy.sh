#!/usr/bin/env bash
# Check 04: NaïveProxy
# Verifies Caddy process, certificate expiry, and ACME health

run_check() {
    local systemd_dir="/etc/systemd/system"
    local status="PASS"
    local failures=()
    local warnings=()
    local details=()

    # Discover NaïveProxy units (both new and legacy prefixes)
    local units=()
    if [[ -d "$systemd_dir" ]]; then
        while IFS= read -r unit_file; do
            units+=("$(basename "$unit_file" .service)")
        done < <(find "$systemd_dir" -maxdepth 1 \( -name "merit-naive-*.service" -o -name "naive-proxy-*.service" \) -type f 2>/dev/null || true)
    fi

    if [[ ${#units[@]} -eq 0 ]]; then
        echo "STATUS: SKIP"
        echo "No NaïveProxy units found"
        return 2
    fi

    details+=("Found ${#units[@]} NaïveProxy units")
    details+=("")

    # Check each unit
    for unit in "${units[@]}"; do
        local main_pid listen_port port_id

        main_pid="$(systemctl show "$unit" -p MainPID --value 2>/dev/null || echo "0")"

        # Extract port
        local unit_file="$systemd_dir/$unit.service"
        if [[ -f "$unit_file" ]]; then
            listen_port="$(grep -oP '^#MeritPort=\K\d+' "$unit_file" 2>/dev/null || echo "")"
        fi

        # Extract port ID from unit name
        if [[ "$unit" =~ merit-naive-(.+) ]]; then
            port_id="${BASH_REMATCH[1]}"
        elif [[ "$unit" =~ naive-proxy-(.+) ]]; then
            port_id="${BASH_REMATCH[1]}"
        fi

        details+=("Unit: $unit")
        details+=("  MainPID: $main_pid")
        details+=("  port: ${listen_port:-unknown}")
        details+=("  port ID: ${port_id:-unknown}")

        # Check Caddy process exists
        if [[ "$main_pid" == "0" || -z "$main_pid" ]]; then
            failures+=("$unit: Caddy process not running (MainPID=0)")
            status="FAIL"
            details+=("  Caddy process: NO")
        else
            if ps -p "$main_pid" >/dev/null 2>&1; then
                local cmdline
                cmdline="$(ps -p "$main_pid" -o comm= 2>/dev/null || echo "")"
                details+=("  Caddy process: YES (pid=$main_pid, cmd=$cmdline)")

                # Verify it's actually caddy
                if [[ ! "$cmdline" =~ caddy ]]; then
                    warnings+=("$unit: MainPID $main_pid does not appear to be caddy (cmd=$cmdline)")
                fi
            else
                failures+=("$unit: MainPID $main_pid not found in process list")
                status="FAIL"
                details+=("  Caddy process: NO (stale PID)")
            fi
        fi

        # Check certificate
        local cert_dir="/etc/merit/naive/${port_id}_certs"
        local cert_file=""

        if [[ -d "$cert_dir" ]]; then
            # Find certificate file (Caddy stores as <domain>.crt or in subdirs)
            cert_file="$(find "$cert_dir" -name "*.crt" -o -name "*.pem" 2>/dev/null | head -n1 || echo "")"
        fi

        if [[ -z "$cert_file" || ! -f "$cert_file" ]]; then
            details+=("  certificate: NOT FOUND (may be pending ACME)")
            details+=("  certificate expiry: N/A")
        else
            details+=("  certificate: $cert_file")

            # Check expiry with openssl
            if command -v openssl >/dev/null 2>&1; then
                local not_after days_to_expire

                not_after="$(openssl x509 -in "$cert_file" -noout -enddate 2>/dev/null | cut -d= -f2 || echo "")"
                if [[ -n "$not_after" ]]; then
                    local expiry_epoch current_epoch
                    expiry_epoch="$(date -d "$not_after" +%s 2>/dev/null || date -j -f "%b %d %H:%M:%S %Y %Z" "$not_after" +%s 2>/dev/null || echo "0")"
                    current_epoch="$(date +%s)"

                    if [[ "$expiry_epoch" -gt 0 ]]; then
                        days_to_expire=$(( (expiry_epoch - current_epoch) / 86400 ))
                        details+=("  certificate expiry: $not_after ($days_to_expire days remaining)")

                        # Apply Task 2 expiry classification
                        if [[ $days_to_expire -lt 0 ]]; then
                            failures+=("$unit: certificate EXPIRED ($days_to_expire days)")
                            status="FAIL"
                        elif [[ $days_to_expire -lt 7 ]]; then
                            warnings+=("$unit: certificate CRITICAL (<7 days: $days_to_expire days)")
                        elif [[ $days_to_expire -lt 30 ]]; then
                            warnings+=("$unit: certificate WARNING (<30 days: $days_to_expire days)")
                        fi
                    else
                        details+=("  certificate expiry: UNKNOWN (parse failed)")
                    fi
                else
                    details+=("  certificate expiry: UNKNOWN (openssl failed)")
                fi
            else
                details+=("  certificate expiry: SKIP (no openssl)")
            fi
        fi

        # Check ACME health in journal (last 7 days)
        local acme_errors=0
        if command -v journalctl >/dev/null 2>&1; then
            # Look for ACME error events
            local journal_output
            journal_output="$(journalctl -u "$unit.service" --since "7 days ago" --no-pager 2>/dev/null || echo "")"

            # Count ACME-related errors
            acme_errors="$(echo "$journal_output" | grep -ciE 'acme.*(error|fail|unable)' || echo "0")"

            if [[ $acme_errors -gt 0 ]]; then
                warnings+=("$unit: found $acme_errors ACME error events in last 7 days")
                details+=("  ACME errors (7d): $acme_errors")
            else
                details+=("  ACME errors (7d): 0")
            fi
        else
            details+=("  ACME errors (7d): SKIP (no journalctl)")
        fi

        details+=("")
    done

    # Output
    if [[ "$status" == "PASS" ]]; then
        echo "STATUS: PASS"
        if [[ ${#warnings[@]} -gt 0 ]]; then
            echo "All NaïveProxy checks passed with ${#warnings[@]} warnings"
        else
            echo "All NaïveProxy units healthy"
        fi
    else
        echo "STATUS: FAIL"
        echo "NaïveProxy check failed: ${#failures[@]} issues found"
    fi

    printf '%s\n' "${details[@]}"

    if [[ ${#warnings[@]} -gt 0 ]]; then
        echo ""
        echo "Warnings (not failures):"
        printf '%s\n' "${warnings[@]}"
    fi

    if [[ ${#failures[@]} -gt 0 ]]; then
        echo ""
        echo "Failures:"
        printf '%s\n' "${failures[@]}"
    fi

    [[ "$status" == "PASS" ]] && return 0 || return 1
}
