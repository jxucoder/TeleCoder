#!/usr/bin/env bash
# Verification — run tests and linters in a session workspace.

session_verify() {
    local id="$1"
    local row
    row=$(db_query "SELECT workspace, test_cmd, lint_cmd FROM sessions WHERE id='${id}';")

    if [ -z "$row" ]; then
        echo "Session not found: $id" >&2
        return 1
    fi

    IFS='|' read -r workspace test_cmd lint_cmd <<< "$row"
    local any_failed=0

    if [ -z "$test_cmd" ] && [ -z "$lint_cmd" ]; then
        echo "No verification commands configured for this session."
        echo "Use --test-cmd or --lint-cmd when creating a session."
        return 0
    fi

    if [ -n "$test_cmd" ]; then
        echo "=== Tests: $test_cmd ==="
        if (cd "$workspace" && eval "$test_cmd"); then
            echo "[PASS] tests"
        else
            echo "[FAIL] tests (exit $?)"
            any_failed=1
        fi
        echo ""
    fi

    if [ -n "$lint_cmd" ]; then
        echo "=== Lint: $lint_cmd ==="
        if (cd "$workspace" && eval "$lint_cmd"); then
            echo "[PASS] lint"
        else
            echo "[FAIL] lint (exit $?)"
            any_failed=1
        fi
        echo ""
    fi

    if [ "$any_failed" -eq 0 ]; then
        echo "Overall: ALL PASSED"
    else
        echo "Overall: SOME FAILED"
    fi
    return $any_failed
}
