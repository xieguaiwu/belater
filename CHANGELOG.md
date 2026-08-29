# Changelog

All notable changes to belater are documented here.

## [0.3.0] - 2026-08-30

### Added

- **Start work automatically at an exact date and time**: `--at` now accepts
  `YYYY-MM-DD HH:MM` in addition to the old `HH:MM`. A `T` separator works
  too (`2026-09-01T02:00`), and a bare date (`2026-09-01`) means 00:00 that
  day. Times are interpreted in the billing timezone (Asia/Shanghai), like
  every other belater time.
  - A bare `HH:MM` keeps the old rule: today, or tomorrow when already past.
  - An absolute time that has already passed is rejected with a clear error
    instead of silently moving — a pinned moment must stay pinned.
  - No scheduler change was needed: dispatch already fires on `RunAt <= now`,
    so a dated task simply waits until its moment. Combine with `--every` for
    a first run at the exact moment, then daily.

## [0.2.4] - 2026-08-29

### Fixed

- **The next window switch had no date**: `belater status` printed
  `下次切换: 09:00 高峰` even when that switch was 41 hours away (Saturday →
  Monday). Labels now name the day: `12:00` for today, `明天 09:00`, `周一 09:00`.
  Day counting is rounded, so a 23/25-hour DST day still counts as one day.
- **"New tasks run immediately" was not true**: the scheduler polls once per
  tick, so the off-peak hint promised instant execution. It now states the
  running scheduler's actual tick interval (e.g. 约 15s 内).
- **Scheduler liveness was lock-only**: holding `daemon.lock` was the whole
  test, so a wedged dispatch loop — or a daemon started before the binary was
  upgraded — still reported `调度器: 运行中 — 到期任务会自动执行`. The scheduler
  now writes a heartbeat each pass (`daemon.heartbeat`: pid, version, tick,
  last tick), and `status` reports heartbeat age plus three new warnings: a
  stalled loop, a scheduler older than the running command, and queued tasks
  whose run time has passed. Each names the recovery command
  (`systemctl --user restart belater`).
- **Reads waited behind the write lock**: `Store.Load` took a blocking
  `flock(LOCK_EX)`, so `status` / `list` / `results` / `log` could stall for as
  long as a scheduler mutation was in flight. Writers publish atomically
  (temp file + rename), so reads never needed the lock. Writes are unchanged.
- **A busy scheduler looked wedged**: the wait after dispatching (`--once`, and
  the graceful-shutdown wait) held the instance lock but stopped writing
  heartbeats, so any task running longer than the grace window made a
  concurrent `belater status` claim the loop had stopped. The wait now pulses
  on its own tick.
- **No more guessed tick**: without heartbeat data `status` no longer claims
  tasks run "约 15s 内" — the real interval of the running scheduler is unknown
  in that case, and an older `--tick 60s` daemon would have made it false.

### Added

- `belater status --json` reports `scheduler_running`, `scheduler_stalled`,
  `scheduler_has_heartbeat`, `scheduler_version`, `scheduler_version_behind`,
  `scheduler_heartbeat_age_s`, `scheduler_tick_s`, `overdue_queued`,
  `overdue_max_s`, `next_switch_label`, `next_switch_kind`, and queue counts.
  Scheduler keys are always present (empty or `0` when unknown) so a consumer
  never has to tell "unknown" apart from "unsupported".
- `belater list` shows an overdue task as `已到期 Xm`; the old output printed
  the same figure as a positive "time remaining".
- Weekend status says `空闲 · 周末全天空闲`, so an all-day off-peak window is
  not mistaken for a display that missed a switch.

## [0.2.3] - 2026-08-27

### Fixed

- **Lock conflict misreported as an error**: `belater daemon` / `belater daemon --once`
  printed `错误: 另一个 belater daemon 实例正在运行` and exited 1 whenever a scheduler
  (e.g. the systemd service) was already running. That is a normal state — the queue
  is already being serviced — so it now prints a neutral info message
  (`调度器已在运行 …`) and exits 0. `--once` under cron no longer looks like a
  failure when the main daemon holds the lock.

## [0.2.2] - 2026-08-25

### Fixed

- **pause/resume/run TOCTOU**: state checks and transitions are now atomic (`CompareAndStatus` / `ClaimForRun` compare-and-swap), so the scheduler can no longer dispatch a task between a user command's check and update.
- `--every HH:MM` first-run semantics: the first run is now the next occurrence of the every time itself (e.g. `--every 02:00` added during the day runs tonight at 02:00), instead of being pushed to the next off-peak window — matching user intuition.
- `--pi-args` documented as spliced verbatim into the shell command.
- CI: bump actions/checkout v5, actions/setup-go v6 (Node 20 deprecation).
- `results` empty message no longer says "最近 0 天" when `--days 0`.

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
