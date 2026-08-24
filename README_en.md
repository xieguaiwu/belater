# belater — Defer big tasks to DeepSeek off-peak billing hours

Queue large jobs (like pi-agent multi-agent work) and run them automatically when DeepSeek billing is off-peak — **half price**. Peak hours: Mon–Fri 09:00–12:00 and 14:00–18:00 (Beijing time); weekends are off-peak all day.

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
| `belater status` | Current window, next switch, queue summary |
| `belater add <cmd...>` | Enqueue (default: next off-peak start) |
| `belater list` | Show queue (`--all`, `--json`) |
| `belater results` | Finished tasks: status/exit code/duration (`--days N`, `--json`) |
| `belater run <id>` | Run immediately in the foreground |
| `belater rm <id>` / `pause` / `resume` | Manage tasks |
| `belater log <id>` | Task log (`--tail N` also works after the ID) |
| `belater daemon` | Scheduler (`--once` for cron; `--install`/`--uninstall` for systemd) |
| `belater config` | Show scheduling config |

### add options

| Option | Description |
|:---|:---|
| `--pi "prompt"` | pi-agent mode: builds `pi -p 'prompt'` |
| `--pi-args "..."` | Extra pi args (e.g. `--no-session`, `--model ...`) |
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

- `~/.config/belater/` (`BELATER_STATE_DIR` overrides): `queue.json` (atomic + flock), `logs/<id>.log`, `daemon.lock`
- Crash recovery: interrupted `running` tasks reset to `queued`; tasks whose last pid is still alive are marked `failed` (no double execution)
- GC: terminal tasks older than 7 days are removed automatically

## Design

```
main.go → internal/cli       subcommands
        → internal/schedule  peak/off-peak windows (weekend rule)
        → internal/queue     task model + JSON persistence (flock, CAS claim)
        → internal/runner    exec + timestamped logs + timeout group-kill
        → internal/daemon    scheduler loop + retry + daily repeat + GC
```

Pure Go standard library, zero third-party dependencies. Implementation plan: `docs/plans/2026-08-24-belater.md`.

## License

MIT — see [LICENSE](LICENSE).
