# belater — Defer big tasks to DeepSeek off-peak billing hours

Queue large jobs (like pi-agent multi-agent work) and run them automatically when DeepSeek billing is off-peak — **half price**. Peak hours: Mon–Fri 09:00–12:00 and 14:00–18:00 (Beijing time); everything else, weekends included, is off-peak. Rules re-checked against the official pricing page on 2026-08-29.

## Install

```bash
git clone https://github.com/xieguaiwu/belater.git
cd belater
go build -o belater .
install -m 0755 belater ~/.local/bin/belater
```

## Quick start

```bash
# Queue a pi-agent task for the next off-peak window (today 18:00 or weekend)
belater add --pi "Refactor the auth module and add tests"

# Run the scheduler (foreground, or install as a service)
belater daemon            # foreground
belater daemon --install  # systemd user service (auto-start)

# Inspect
belater status            # current billing window + countdown
belater list              # queue
belater results           # finished-task summary (exit code, duration)
belater log <id>          # per-task log with metadata header
```

## Commands

| Command | Description |
|:---|:---|
| `belater status` | Current window, next switch (tagged with the day when it is not today), **scheduler liveness from its heartbeat**, overdue tasks, queue summary |
| `belater add <cmd...>` | Enqueue (default: next off-peak start) |
| `belater list` | Show queue (`--all`, `--json`) |
| `belater results` | Finished tasks: status/exit code/duration (`--days N`, `--json`) |
| `belater run <id>` | Run immediately in the foreground |
| `belater rm <id>` / `pause` / `resume` | Manage tasks |
| `belater log <id>` | Task log (`--tail N` also works after the ID) |
| `belater daemon` | Scheduler (`--once` for cron; prints a hint and exits 0 when another scheduler already runs; `--install`/`--uninstall` for systemd) |
| `belater config` | Show scheduling config |

### add options

| Option | Description |
|:---|:---|
| `--pi "prompt"` | pi-agent mode: builds `pi -p 'prompt'` |
| `--pi-args "..."` | Extra pi args, spliced verbatim into the shell command (e.g. `--no-session`, `--model ...`) |
| `--at HH:MM` | Wall-clock time (tomorrow if past; may land in peak — explicit) |
| `--delay 2h` | Run after a delay |
| `--now` | Queue for the next tick |
| `--every HH:MM` | Repeat daily (`--max-runs N` to bound) |
| `--retries N` | Retry on failure (exponential backoff 30s×2ⁿ, clamped to off-peak) |
| `--timeout 30m` | Timeout (kills the process group) |
| `--cwd DIR` / `--env K=V` | Working dir / env (repeatable) |
| `--parallel` | Does not occupy a serial slot |
| `--on-done "cmd"` | Post-completion hook (env: `BELATER_TASK_ID/NAME/STATUS/EXIT`) |
| `--notify` | Desktop notification on completion (`notify-send`) |

## Notifications & results

```bash
belater results                       # summary of the last 7 days
belater add --notify --pi "big job"   # desktop notification when done
belater add --on-done "curl -X POST http://.../hook" --pi "deploy"
```

## Scheduler deployment

**systemd user service (recommended):**
```bash
belater daemon --install   # writes ~/.config/systemd/user/belater.service, enables it
belater daemon --uninstall
```

**cron polling:**
```bash
*/5 * * * * /home/USER/.local/bin/belater daemon --once
```

> ⚠️ **Mutually exclusive.** The scheduler holds a single-instance lock (`daemon.lock`); a second scheduler exits with an error instead of double-dispatching tasks. Stop one before switching to the other.

## State & logs

```
当前时段: 空闲 (0.5x 价格) · 周末全天空闲
下次切换: 周一 09:00 高峰 (1d 16h 55m 后)
现在就是空闲时段 — 新任务将在下次调度检查（约 15s 内）执行
调度器: 运行中 (v0.2.4 · tick 15s · 心跳 1s 前) — 到期任务会自动执行
队列: 0 待执行 · 0 运行中 · 2 已完成
```

`belater status` output is Chinese by design (CLI strings are not localized). Line 2 names the day when the next switch is not today — `明天 09:00` tomorrow, `周一 09:00` Monday — instead of a bare clock time 41 hours out. Line 4 is read from the scheduler heartbeat: warnings are added when the loop stopped ticking, when the running scheduler is an older build than the command, or when a queued task has passed its run time. `--json` exposes the same (`scheduler_running`, `scheduler_stalled`, `overdue_queued`, `next_switch_label`).

- `~/.config/belater/` (`BELATER_STATE_DIR` overrides): `queue.json` (atomic write + flock; **reads take no lock**), `logs/<id>.log`, `daemon.lock` (single instance), `daemon.heartbeat` (pid / version / tick / last pass)
- **PATH handling**: when the daemon environment (systemd/cron) lacks per-user bin dirs (`~/.npm-global/bin`, `~/.local/bin`, `~/go/bin`), task execution auto-appends them to PATH — tools like `pi` installed in user dirs work out of the box; an explicit `--env PATH=...` wins
- Crash recovery: interrupted `running` tasks reset to `queued`; tasks whose last pid is still alive are marked `failed` (no double execution)
- GC: terminal tasks older than 7 days are removed automatically
- **After upgrading the binary, restart the scheduler** (`systemctl --user restart belater`). A running service keeps executing the build it was started from; `belater status` warns when the two differ.

## Design

```
main.go → internal/cli       subcommands
        → internal/schedule  peak/off-peak windows (weekend rule)
        → internal/queue     task model + JSON persistence (flock, CAS claim)
        → internal/runner    exec + timestamped logs + timeout group-kill
        → internal/daemon    scheduler loop + retry + daily repeat + GC + heartbeat
```

Pure Go standard library, zero third-party dependencies. Implementation plan: `docs/plans/2026-08-24-belater.md`.

## Development

```bash
go vet ./...
go test ./...
go build -o belater .
```

Three contracts to keep when touching scheduling display:

1. Billing rules come from the official pricing page only. Changing them means `internal/schedule`, both READMEs, and the constraint block in `docs/plans`.
2. "Running" reads the instance lock, but a clean report also needs a fresh heartbeat and a version matching the command — lock presence alone must never be called healthy.
3. Queue reads must not take `queue.lock`: writers publish with temp-file + rename, so a lock in the read path only queues CLI commands behind the scheduler.

## License

MIT — see [LICENSE](LICENSE).
