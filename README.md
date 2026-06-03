# ZoneAware

ZoneAware is a mouse-capable terminal app for visualizing team working hours across time zones. It reads a human-editable YAML config, renders a rolling hourly timeline, and lets you hide team members or timezone labels directly in the TUI.

## Run

```bash
cp examples/zoneaware.yaml ~/.config/zoneaware.yaml
./run.sh
```

Use `./run.sh -config /path/to/zoneaware.yaml` to point at a different config file. `run.sh` builds a local binary under `.bin/` when needed, then launches the TUI from the repository root.

## Controls

- Click a team member name to hide that row
- Click a timeline cell to toggle that local hour for the team member and save it back to config
- Click a member name to edit it inline and save the new name
- Click the right-side timezone on a member row to change that member's timezone
- Click a right-side timezone label in the header to hide that timezone row
- Click `Show All` to reset hidden rows and timezones
- Click `[ Add ]` at the bottom to create a new team member
- Click `Local Reference` to open a timezone picker; typing filters it live
- Use the mouse wheel to scroll the timeline horizontally through time
- Press `Ctrl+L` to refresh, `r` to reset, or `q` to quit

## Config

```yaml
reference_timezone: UTC
window_hours: 48
team:
  - name: Alice
    timezone: America/Los_Angeles
    working_hours:
      - start: "09:00"
        end: "17:00"
```

- The visible window is pinned to the current local hour and refreshes on `Ctrl+L`
- The default visible window shrinks or grows to match the terminal width; the mouse wheel scrolls earlier/later through the full timeline
- `reference_timezone` overrides the shell's current timezone for the local reference row; if omitted, the shell timezone is used
- `window_hours` defaults to 48; the terminal width may reduce the visible range that fits on screen
- `working_hours` supports multiple ranges per person and allows overnight ranges such as `22:00` to `02:00`
