# Changelog

All notable changes to belater are documented here.

## [0.2.1] - 2026-08-25

### Fixed

- **Task PATH in daemon environments**: systemd/cron environments lack per-user bin dirs (`~/.npm-global/bin`, `~/.local/bin`, `~/go/bin`), so tasks calling tools like `pi` failed with exit 127 (`command not found`). The runner now appends missing user bin dirs to PATH; an explicit `--env PATH=...` wins. The systemd unit also sets `Environment=PATH=...` (`%h` expanded).
- **Misleading daemon banner**: `belater daemon` printed its "running" banner before the instance-lock conflict error. A lock pre-check now fails fast with the actionable message only (both `daemon` and `daemon --once`).
- `belater log <id> --tail N` — flags after the positional ID now parse correctly.
- Last log line without a trailing newline is written exactly once (was duplicated).
- Misleading hint when adding with `--delay` during peak hours.

### Added

- Scheduler liveness in `belater status`: "调度器: 运行中 / 未运行" via the instance lock.
- Actionable guidance when the instance lock is held (view/stop commands).
- `belater add` detects a running scheduler and says tasks will execute automatically.

## [0.2.0] - 2026-08-24

### Added

- `belater results`: finished-task summary (status, exit code, duration, finished time), `--days N`, `--json`.
- Metadata header in `belater log <id>`: status, exit code, duration, command.
- `--on-done "cmd"` post-completion hook (env: `BELATER_TASK_ID/NAME/STATUS/EXIT`); runs for both daemon dispatch and foreground `run`.
- `--notify`: desktop notification on completion (`notify-send`).

## [0.1.1] - 2026-08-24

Critical review round (momus): P0 ×1, P1 ×3, P2 ×9 — all fixed.

### Fixed

- **P0 double-dispatch race**: `Store.ClaimDue` compare-and-swap + `daemon.lock` single-instance flock. Verified: second instance exits with an error.
- Pipe drain unbounded wait → bounded (10 s) so an escaped child cannot hang a task forever.
- Orphan recovery: task pid persisted; on restart a still-alive pid marks the task `failed` instead of re-running it.
- Retries clamped into off-peak windows (`NextOffPeakStart`); `--every` warns when it lands in peak.
- Hardening: single-write last line, billing-timezone parsing for `--at`/`--every`, 0600 file permissions, task-ID validation, corrupt `queue.json` quarantine, fsync before rename, dead code removal.

## [0.1.0] - 2026-08-24

Initial release.

- DeepSeek peak/off-peak schedule (Mon–Fri 09:00–12:00 & 14:00–18:00 Beijing time; weekends all off-peak at 50%).
- Queue with JSON persistence (atomic write + flock), runner (`sh -c`, timestamped logs, timeout kills process group), scheduler loop (crash recovery, exponential-backoff retries, daily repeat, 7-day GC).
- CLI: status/add/list/rm/pause/resume/run/log/daemon/config.
- pi-agent mode (`--pi` / `--pi-args`), systemd user service install, cron `--once` mode.
