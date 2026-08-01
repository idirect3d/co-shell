#!/usr/bin/env bash
# output_audit.sh - Audit co-shell output calls for architecture compliance.
#
# Purpose:
#   Verify that output calls follow the co-shell output architecture
#   (see docs/output-architecture.md). Used as a regression gate for each
#   migration phase (P2 onward).
#
# Checks:
#   1. Direct fmt.Print/Printf/Println in user-interaction paths
#      (should use UserIO / Out instead).
#   2. Magic string stream events: cb("event", ...)
#      (should use event constants from agent/events.go once P1 lands).
#   3. Hardcoded Chinese strings in Go source outside i18n files.
#   4. Sync-blocking ReadLine/ReadKey calls (should migrate to InputSource
#      event consumption after P2.5 input unification).
#   5. i18n key zh/en coverage (every key in keys.go must have both zh & en).
#
# Usage:
#   bin/output_audit.sh           # report non-compliant items + summary
#   bin/output_audit.sh --list    # full listing of every non-compliant item
#   bin/output_audit.sh --strict  # exit 1 if any non-compliance found (CI gate)
#
# Exit codes:
#   0  all clean (or --strict not set and items found)
#   1  non-compliance found and --strict is set
#
# Author: L.Shuang
# Created: 2026-08-01
# MIT License - Copyright (c) 2026 L.Shuang

set -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="summary"       # summary | list
STRICT=0

for arg in "$@"; do
    case "$arg" in
        --list)   MODE="list" ;;
        --strict) STRICT=1 ;;
        *)
            echo "Unknown option: $arg" >&2
            echo "Usage: $0 [--list] [--strict]" >&2
            exit 2
            ;;
    esac
done

cd "$ROOT" || exit 2

# ---------------------------------------------------------------------------
# Helper: collect Go source files, excluding tests / submodules / infra files.
# ---------------------------------------------------------------------------
# Note: find outputs paths with ./ prefix (e.g. ./agent/io.go), so patterns
# must include the leading ./ to match.
EXCLUDE_PATTERNS=(
    '*_test.go'
    './mobile/'
    './hub/'
    './agent/io.go'        # UserIO implementation - legal fmt usage
    './repl/userio.go'     # UserIO implementation - legal fmt usage
    './log/log.go'         # logger itself
    './repl/enhanced_input.go' # terminal control sequences - infra
    './bin/'
)

build_file_list() {
    local pattern
    local list=()
    while IFS= read -r f; do
        local excluded=0
        for pattern in "${EXCLUDE_PATTERNS[@]}"; do
            if [[ "$f" == $pattern ]]; then
                excluded=1
                break
            fi
        done
        [[ $excluded -eq 0 ]] && list+=("$f")
    done < <(find . -name '*.go' -not -path './.git/*' | sort)
    printf '%s\n' "${list[@]}"
}

print_item() {
    # Output to stderr so it does NOT pollute $(...) command substitution
    # results used for counting (bash printf would choke on non-numeric input).
    if [[ "$MODE" == "list" ]]; then
        echo "  $1" >&2
    fi
}

# ---------------------------------------------------------------------------
# Check 1: direct fmt output in user-interaction paths.
# ---------------------------------------------------------------------------
check_fmt_output() {
    local files
    local count=0
    files=$(build_file_list)

    while IFS= read -r f; do
        [[ -z "$f" ]] && continue
        # Only flag *user-facing* fmt calls. Ignore:
        #  - fmt.Errorf / fmt.Sprintf / fmt.Sprint* (non-printing)
        #  - log.* calls
        #  - go:embed markers
        while IFS=: read -r lineno line; do
            [[ -z "$lineno" ]] && continue
            # Skip lines inside comments
            trimmed="$(echo "$line" | sed 's/^[[:space:]]*//')"
            [[ "$trimmed" == \//* ]] && continue
            [[ "$trimmed" == \* ]] && continue
            # Skip UserIO implementation lines (legal fmt usage)
            if echo "$line" | grep -qE 'func \(f \*fmtIO\) (Print|Printf|Println)'; then
                continue
            fi
            count=$((count + 1))
            print_item "$f:$lineno: $line"
        done < <(grep -nE '(^|[^[:alnum:]_.])fmt\.(Print|Printf|Println)\(' "$f")
    done <<< "$files"

    echo "$count"
}

# ---------------------------------------------------------------------------
# Check 2: magic string stream events cb("event", ...)
# ---------------------------------------------------------------------------
check_event_magic() {
    local files
    local count=0
    files=$(build_file_list)

    while IFS= read -r f; do
        [[ -z "$f" ]] && continue
        while IFS=: read -r lineno line; do
            [[ -z "$lineno" ]] && continue
            count=$((count + 1))
            print_item "$f:$lineno: $line"
        done < <(grep -nE 'cb\("[a-z_]+"' "$f")
    done <<< "$files"

    echo "$count"
}

# ---------------------------------------------------------------------------
# Check 3: hardcoded Chinese in Go strings outside i18n package.
# ---------------------------------------------------------------------------
check_hardcoded_chinese() {
    local count=0

    while IFS= read -r f; do
        [[ -z "$f" ]] && continue
        while IFS=: read -r lineno line; do
            [[ -z "$lineno" ]] && continue
            # Skip comment lines
            trimmed="$(echo "$line" | sed 's/^[[:space:]]*//')"
            [[ "$trimmed" == \//* ]] && continue
            [[ "$trimmed" == \* ]] && continue
            # Only count lines containing CJK chars inside string literals.
            # Rough heuristic: a quote before and after the CJK run.
            if echo "$line" | grep -qE '"[^"]*[一-鿿][^"]*"'; then
                count=$((count + 1))
                print_item "$f:$lineno: $line"
            fi
        done < <(grep -nE '[一-鿿]' "$f")
    done <<< "$(find . -name '*.go' -not -path './i18n/*' -not -path './.git/*' | grep -v _test.go | sort)"

    echo "$count"
}

# ---------------------------------------------------------------------------
# Check 4: sync-blocking input calls (ReadLine/ReadKey) that should migrate
# to InputSource event consumption after P2.5 (input unification).
# ---------------------------------------------------------------------------
check_sync_input() {
    local files
    local count=0
    files=$(build_file_list)

    while IFS= read -r f; do
        [[ -z "$f" ]] && continue
        while IFS=: read -r lineno line; do
            [[ -z "$lineno" ]] && continue
            # Skip UserIO implementation files' own ReadLine/ReadKey definitions
            if echo "$line" | grep -qE 'func \((s \*StdioIO|e \*EnhancedIO|d \*DefaultUserIO|f \*fmtIO)\) (ReadLine|ReadKey)'; then
                continue
            fi
            count=$((count + 1))
            print_item "$f:$lineno: $line"
        done < <(grep -nE '\.(ReadLine|ReadKey)\(' "$f")
    done <<< "$files"

    echo "$count"
}

# ---------------------------------------------------------------------------
# Check 5: i18n key zh/en coverage.
# Every key string declared in i18n/keys.go (KeyXxx = "str_key") must have a
# translation in both zhMessages (zh*.go) and enMessages (en*.go).
# Detection covers both map literal ("key": "...") and indexed-init form
# (zhMessages[KeyLoopJudgeSystemPrompt] = `...`) — the key string appears in
# the file as "str_key" in either form.
# ---------------------------------------------------------------------------
check_i18n_coverage() {
    local count=0

    builtin_print() {
        print_item "$1"
    }

    # Extract key strings from the main map files (map-literal form "key": ...).
    # These are the only files where the key string literal appears in a
    # greppable form (zh_loop/en_loop/zh_system/en_system use indexed-init).
    local zh_keys en_keys
    zh_keys=$(grep -oE '"[a-zA-Z_0-9]+"[[:space:]]*:' i18n/zh.go 2>/dev/null | sed 's/[":[:space:]]//g' | sort -u)
    en_keys=$(grep -oE '"[a-zA-Z_0-9]+"[[:space:]]*:' i18n/en.go 2>/dev/null | sed 's/[":[:space:]]//g' | sort -u)

    # 1) Every zh.go key must exist in en.go
    while IFS= read -r k; do
        [[ -z "$k" ]] && continue
        if ! grep -qxF "$k" <<< "$en_keys"; then
            count=$((count + 1))
            builtin_print "i18n key '$k' present in zh.go but missing in en.go"
        fi
    done <<< "$zh_keys"

    # 2) Every en.go key must exist in zh.go
    while IFS= read -r k; do
        [[ -z "$k" ]] && continue
        if ! grep -qxF "$k" <<< "$zh_keys"; then
            count=$((count + 1))
            builtin_print "i18n key '$k' present in en.go but missing in zh.go"
        fi
    done <<< "$en_keys"

    # 3) Indexed-init forms: zh_loop.go <-> en_loop.go, zh_system.go <-> en_system.go
    #    Each zhMessages[KeyXxx] must have a matching enMessages[KeyXxx].
    local pair
    for pair in "zh_loop.go:en_loop.go" "zh_system.go:en_system.go"; do
        local zhfile="${pair%%:*}" enfile="${pair##*:}"
        local zh_consts en_consts
        zh_consts=$(grep -oE 'zhMessages\[[A-Za-z0-9_]+\]' "i18n/$zhfile" 2>/dev/null | sed 's/zhMessages\[//; s/\]//' | sort -u)
        en_consts=$(grep -oE 'enMessages\[[A-Za-z0-9_]+\]' "i18n/$enfile" 2>/dev/null | sed 's/enMessages\[//; s/\]//' | sort -u)
        while IFS= read -r c; do
            [[ -z "$c" ]] && continue
            if ! grep -qxF "$c" <<< "$en_consts"; then
                count=$((count + 1))
                builtin_print "i18n constant '$c' in $zhfile missing in $enfile"
            fi
        done <<< "$zh_consts"
        while IFS= read -r c; do
            [[ -z "$c" ]] && continue
            if ! grep -qxF "$c" <<< "$zh_consts"; then
                count=$((count + 1))
                builtin_print "i18n constant '$c' in $enfile missing in $zhfile"
            fi
        done <<< "$en_consts"
    done

    echo "$count"
}

# ---------------------------------------------------------------------------
# Run checks
# ---------------------------------------------------------------------------
fmt_count=$(check_fmt_output)
event_count=$(check_event_magic)
chinese_count=$(check_hardcoded_chinese)
input_count=$(check_sync_input)
i18n_missing=$(check_i18n_coverage)

echo "=============================================="
echo " co-shell output audit"
echo "=============================================="
printf '  Direct fmt output (user paths):  %d\n' "$fmt_count"
printf '  Magic string stream events:      %d\n' "$event_count"
printf '  Hardcoded Chinese (non-i18n):    %d\n' "$chinese_count"
printf '  Sync-blocking ReadLine/ReadKey:  %d\n' "$input_count"
printf '  i18n keys missing zh/en:         %d\n' "$i18n_missing"
echo "----------------------------------------------"

if [[ $MODE == "summary" ]]; then
    echo "  Tip: re-run with --list for full detail."
fi
if [[ $STRICT -eq 1 ]]; then
    echo "  (strict mode)"
fi
echo "=============================================="

if [[ $STRICT -eq 1 && $((fmt_count + event_count + chinese_count + input_count + i18n_missing)) -gt 0 ]]; then
    exit 1
fi
exit 0
