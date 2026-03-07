#!/usr/bin/env bash
# Session management — create, run, stop, list, inspect via tmux + sqlite.

session_create() {
    local repo_url="$1" repo_path="$2" branch="$3" test_cmd="$4" lint_cmd="$5"
    local id
    id=$(head -c6 /dev/urandom | xxd -p)
    local workspace="${TELECODER_DATA}/workspaces/${id}"
    mkdir -p "$workspace"

    # Clone or copy repo into workspace
    if [ -n "$repo_url" ]; then
        git clone "$repo_url" "$workspace/repo" 2>&1
        workspace="${workspace}/repo"
    elif [ -n "$repo_path" ]; then
        # Use git worktree if inside a git repo, otherwise just set the path
        if git -C "$repo_path" rev-parse --git-dir >/dev/null 2>&1; then
            git -C "$repo_path" worktree add "$workspace/repo" -b "${TELECODER_BRANCH_PREFIX}${id}" 2>&1
            workspace="${workspace}/repo"
        else
            workspace="$repo_path"
        fi
    fi

    if [ -n "$branch" ] && [ -d "${workspace}/.git" ]; then
        git -C "$workspace" checkout -b "$branch" 2>/dev/null || git -C "$workspace" checkout "$branch" 2>/dev/null
    fi

    db_exec "INSERT INTO sessions (id, repo_url, repo_path, workspace, branch, status, test_cmd, lint_cmd)
             VALUES ('${id}', '${repo_url}', '${repo_path}', '${workspace}', '${branch}', 'created', '${test_cmd}', '${lint_cmd}');"

    echo "$id"
}

session_run() {
    local id="$1" prompt="$2"
    local workspace
    workspace=$(db_query "SELECT workspace FROM sessions WHERE id='${id}';")

    if [ -z "$workspace" ]; then
        echo "Session not found: $id" >&2
        return 1
    fi

    local logs_dir="${TELECODER_DATA}/logs"
    mkdir -p "$logs_dir"

    # Launch claude in a tmux session
    local tmux_name="tc-${id}"
    tmux new-session -d -s "$tmux_name" -c "$workspace" \
        "${TELECODER_RUNTIME} -p '${prompt}' --output-format stream-json 2>'${logs_dir}/${id}.stderr.log' | tee '${logs_dir}/${id}.stdout.log'"

    local pid
    pid=$(tmux list-panes -t "$tmux_name" -F '#{pane_pid}' 2>/dev/null | head -1)

    db_exec "UPDATE sessions SET status='running', prompt='$(echo "$prompt" | sed "s/'/''/g")', pid=${pid:-0}, updated_at=datetime('now') WHERE id='${id}';"

    echo "running in tmux session: $tmux_name (pid ${pid:-unknown})"
}

session_stop() {
    local id="$1"
    local tmux_name="tc-${id}"

    if tmux has-session -t "$tmux_name" 2>/dev/null; then
        tmux send-keys -t "$tmux_name" C-c
        sleep 1
        tmux kill-session -t "$tmux_name" 2>/dev/null || true
    fi

    db_exec "UPDATE sessions SET status='stopped', updated_at=datetime('now') WHERE id='${id}';"
    echo "stopped"
}

session_list() {
    local status_filter="$1"
    local query="SELECT id, status, COALESCE(repo_url, repo_path, '-'), created_at FROM sessions"
    if [ -n "$status_filter" ]; then
        query="${query} WHERE status='${status_filter}'"
    fi
    query="${query} ORDER BY created_at DESC;"

    printf "%-14s %-12s %-35s %s\n" "ID" "STATUS" "REPO" "CREATED"
    printf '%s\n' "$(printf '%.0s-' {1..80})"

    db_query "$query" | while IFS='|' read -r sid sstatus srepo screated; do
        # Truncate long repo strings
        if [ ${#srepo} -gt 33 ]; then
            srepo="...${srepo: -30}"
        fi
        printf "%-14s %-12s %-35s %s\n" "$sid" "$sstatus" "$srepo" "$screated"
    done
}

session_inspect() {
    local id="$1"
    local row
    row=$(db_query "SELECT id, status, repo_url, repo_path, workspace, branch, prompt, pid, test_cmd, lint_cmd, created_at, updated_at FROM sessions WHERE id='${id}';")

    if [ -z "$row" ]; then
        echo "Session not found: $id" >&2
        return 1
    fi

    IFS='|' read -r sid sstatus srepo_url srepo_path sworkspace sbranch sprompt spid stest slint screated supdated <<< "$row"

    # Check if tmux session is still alive and update status
    local tmux_name="tc-${id}"
    if [ "$sstatus" = "running" ] && ! tmux has-session -t "$tmux_name" 2>/dev/null; then
        db_exec "UPDATE sessions SET status='completed', updated_at=datetime('now') WHERE id='${id}';"
        sstatus="completed"
    fi

    echo "Session:    $sid"
    echo "Status:     $sstatus"
    echo "Workspace:  $sworkspace"
    echo "Repo URL:   ${srepo_url:--}"
    echo "Repo Path:  ${srepo_path:--}"
    echo "Branch:     ${sbranch:--}"
    echo "PID:        ${spid:--}"
    echo "Test cmd:   ${stest:--}"
    echo "Lint cmd:   ${slint:--}"
    echo "Created:    $screated"
    echo "Updated:    $supdated"

    if [ -n "$sprompt" ]; then
        echo ""
        echo "Last prompt:"
        echo "  $sprompt"
    fi

    # Show changed files if workspace has git
    if [ -d "${sworkspace}/.git" ]; then
        local changed
        changed=$(git -C "$sworkspace" status --porcelain 2>/dev/null)
        if [ -n "$changed" ]; then
            echo ""
            echo "Changed files:"
            echo "$changed" | sed 's/^/  /'
        fi
    fi
}

session_logs() {
    local id="$1" stream="${2:-stdout}" tail_n="${3:-100}"
    local logfile="${TELECODER_DATA}/logs/${id}.${stream}.log"

    if [ ! -f "$logfile" ]; then
        echo "No logs yet."
        return
    fi
    tail -n "$tail_n" "$logfile"
}

session_attach() {
    local id="$1"
    local tmux_name="tc-${id}"

    if ! tmux has-session -t "$tmux_name" 2>/dev/null; then
        echo "No active tmux session for $id" >&2
        return 1
    fi
    tmux attach-session -t "$tmux_name"
}

session_delete() {
    local id="$1"
    session_stop "$id" >/dev/null 2>&1 || true

    local workspace
    workspace=$(db_query "SELECT workspace FROM sessions WHERE id='${id}';")

    db_exec "DELETE FROM sessions WHERE id='${id}';"

    if [ -n "$workspace" ] && [[ "$workspace" == *"${TELECODER_DATA}"* ]]; then
        rm -rf "$workspace"
    fi
    echo "deleted"
}
