# Shell aliases

Add this Bash function to your shell config:

```bash
za() {
    local repo="${HOME}/src/zoneaware"
    local runner="${repo}/run.sh"

    if [[ ! -x "$runner" ]]; then
        printf 'zoneaware: missing runner at %s\n' "$runner" >&2
        return 1
    fi

    if [[ -n "${ZELLIJ:-}" ]] && command -v zellij >/dev/null 2>&1; then
        local width height
        if ! read -r width height < <("$runner" -print-zellij-pane-size -pane-hours 24 "$@"); then
            return 1
        fi

        zellij action new-pane \
            --floating \
            --close-on-exit \
            --pinned true \
            --name zoneaware \
            --cwd "$repo" \
            --width "$width" \
            --height "$height" \
            -- "$runner" "$@"
        return $?
    fi

    "$runner" "$@"
}
```

Behavior:

- Outside Zellij, `za` runs `~/src/zoneaware/run.sh`
- Inside Zellij, `za` opens a **pinned floating pane**
- The pane size is calculated to show **exactly 24 hours**
- The pane height is sized for the current team list, headers, controls, and footers
- Quitting ZoneAware closes the floating pane because it is opened with `--close-on-exit`
- The current Zellij theme is left alone because the function does not change theme settings
