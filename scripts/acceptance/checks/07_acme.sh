#!/usr/bin/env bash
# Check 07: ACME (read-only by default)
# Verifies domain resolution, certificate presence, validity, and ACME history
# Does NOT trigger certificate issuance unless --with-acme-issuance is set

run_check() {
    local systemd_dir="/etc/systemd/system"
    local status="PASS"
    local failures=()
    local warnings=()
    local details=()

    # Check if active testing is enabled
    local with_issuance="${WITH_ACME_ISSUANCE:-false}"

    details+=("ACME test mode: $(${with_issuance} && echo "ACTIVE (--with-acme-issuance)" || echo "READ-ONLY (default)")")
    details+=("")

    # Discover NaïveProxy units (they use ACME)
    local units=()
    if [[ -d "$systemd_dir" ]]; then
        while IFS= read -r unit_file; do
            units+=("$(basename "$unit_file" .service)")
        done < <(find "$systemd_dir" -maxdepth 1 \( -name "merit-naive-*.service" -o -name "naive-proxy-*.service" \) -type f 2>/dev/null || true)
    fi

    if [[ ${#units[@]} -eq 0 ]]; then
        echo "STATUS: SKIP"
        echo "No NaïveProxy units found (ACME only applies to NaïveProxy)"
        return 2
    fi

    details+=("Found ${#units[@]} NaïveProxy units with potential ACME usage")
    details+=("")

    # Check each unit
    for unit in "${units[@]}"; do
        local port_id domain cert_dir cert_file

        # Extract port ID
        if [[ "$unit" =~ merit-naive-(.+) ]]; then
            port_id="${BASH_REMATCH[1]}"
        elif [[ "$unit" =~ naive-proxy-(.+) ]]; then
            port_id="${BASH_REMATCH[1]}"
        fi

        details+=("Unit: $unit (port ID: $port_id)")

        # Try to extract domain from config file
        local config_file="/etc/merit/naive/${port_id}.json"
        if [[ -f "$config_file" ]]; then
            # Parse domain from Caddy JSON config
            if command -v jq >/dev/null 2>&1; then
                domain="$(jq -r '.apps.tls.certificates.automate[0]? // empty' "$config_file" 2>/dev/null || echo "")"
            else
                # Fallback: grep for domain pattern
                domain="$(grep -oP '"automate":\s*\[\s*"\K[^"]+' "$config_file" 2>/dev/null | head -n1 || echo "")"
            fi
        fi

        if [[ -z "$domain" ]]; then
            details+=("  domain: UNKNOWN (cannot extract from config)")
            details+=("  ACME check: SKIP")
            details+=("")
            continue
        fi

        details+=("  domain: $domain")

        # 1. Check domain resolution
        if command -v dig >/dev/null 2>&1; then
            local resolved_ips
            resolved_ips="$(dig +short "$domain" A 2>/dev/null || echo "")"
            if [[ -n "$resolved_ips" ]]; then
                details+=("  DNS resolution: YES ($(echo "$resolved_ips" | tr '\n' ' '))")
            else
                warnings+=("$unit ($domain): DNS resolution returned no A records")
                details+=("  DNS resolution: NO A records")
            fi
        elif command -v nslookup >/dev/null 2>&1; then
            if nslookup "$domain" >/dev/null 2>&1; then
                details+=("  DNS resolution: YES")
            else
                warnings+=("$unit ($domain): DNS resolution failed")
                details+=("  DNS resolution: FAIL")
            fi
        else
            details+=("  DNS resolution: SKIP (no dig/nslookup)")
        fi

        # 2. Check certificate presence and validity
        cert_dir="/etc/merit/naive/${port_id}_certs"
        cert_file=""

        if [[ -d "$cert_dir" ]]; then
            cert_file="$(find "$cert_dir" -name "*.crt" -o -name "*.pem" 2>/dev/null | head -n1 || echo "")"
        fi

        if [[ -z "$cert_file" || ! -f "$cert_file" ]]; then
            details+=("  certificate: NOT FOUND")
            details+=("  validity: N/A")

            if ! ${with_issuance}; then
                warnings+=("$unit ($domain): certificate not found (use --with-acme-issuance to test issuance)")
            fi
        else
            details+=("  certificate: FOUND ($cert_file)")

            # Check validity
            if command -v openssl >/dev/null 2>&1; then
                local not_before not_after subject issuer days_to_expire

                not_before="$(openssl x509 -in "$cert_file" -noout -startdate 2>/dev/null | cut -d= -f2 || echo "")"
                not_after="$(openssl x509 -in "$cert_file" -noout -enddate 2>/dev/null | cut -d= -f2 || echo "")"
                subject="$(openssl x509 -in "$cert_file" -noout -subject 2>/dev/null | cut -d= -f2- || echo "")"
                issuer="$(openssl x509 -in "$cert_file" -noout -issuer 2>/dev/null | cut -d= -f2- || echo "")"

                details+=("  subject: $subject")
                details+=("  issuer: $issuer")
                details+=("  valid from: $not_before")
                details+=("  valid until: $not_after")

                if [[ -n "$not_after" ]]; then
                    local expiry_epoch current_epoch
                    expiry_epoch="$(date -d "$not_after" +%s 2>/dev/null || date -j -f "%b %d %H:%M:%S %Y %Z" "$not_after" +%s 2>/dev/null || echo "0")"
                    current_epoch="$(date +%s)"

                    if [[ "$expiry_epoch" -gt 0 ]]; then
                        days_to_expire=$(( (expiry_epoch - current_epoch) / 86400 ))
                        details+=("  days to expire: $days_to_expire")

                        if [[ $days_to_expire -lt 0 ]]; then
                            failures+=("$unit ($domain): certificate EXPIRED")
                            status="FAIL"
                        elif [[ $days_to_expire -lt 7 ]]; then
                            warnings+=("$unit ($domain): certificate CRITICAL (<7 days)")
                        elif [[ $days_to_expire -lt 30 ]]; then
                            warnings+=("$unit ($domain): certificate WARNING (<30 days)")
                        fi
                    fi
                fi
            else
                details+=("  validity: SKIP (no openssl)")
            fi
        fi

        # 3. Check ACME event history in journal
        if command -v journalctl >/dev/null 2>&1; then
            local success_count failure_count

            # Count ACME success events
            success_count="$(journalctl -u "$unit.service" --since "30 days ago" --no-pager 2>/dev/null | \
                grep -ciE 'tls\.issuance\.acme.*certificate obtained successfully|tls\.renew.*renewed certificate' || echo "0")"

            # Count ACME failure events
            failure_count="$(journalctl -u "$unit.service" --since "30 days ago" --no-pager 2>/dev/null | \
                grep -ciE 'tls\.obtain.*could not get certificate|acme.*(error|authorization error)' || echo "0")"

            details+=("  ACME history (30d):")
            details+=("    success events: $success_count")
            details+=("    failure events: $failure_count")

            if [[ $failure_count -gt 0 ]]; then
                # Extract last error message
                local last_error
                last_error="$(journalctl -u "$unit.service" --since "30 days ago" --no-pager 2>/dev/null | \
                    grep -iE 'acme.*(error|authorization error)' | tail -n1 || echo "")"

                if [[ -n "$last_error" ]]; then
                    details+=("    last error: ${last_error:0:200}...")
                fi

                warnings+=("$unit ($domain): $failure_count ACME failures in last 30 days")
            fi
        else
            details+=("  ACME history: SKIP (no journalctl)")
        fi

        # 4. Active testing (only if --with-acme-issuance)
        if ${with_issuance}; then
            details+=("  active issuance test: SKIPPED (not implemented - manual trigger required)")
            details+=("    to manually test: systemctl restart $unit")
        fi

        details+=("")
    done

    # Output
    if [[ "$status" == "PASS" ]]; then
        echo "STATUS: PASS"
        if [[ ${#warnings[@]} -gt 0 ]]; then
            echo "ACME checks passed with ${#warnings[@]} warnings"
        else
            echo "All ACME checks passed (read-only mode)"
        fi
    else
        echo "STATUS: FAIL"
        echo "ACME check failed: ${#failures[@]} critical issues found"
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
