# belater — DeepSeek 峰谷计费延迟执行器

把大型任务（如 pi-agent 多 agent 工作）排队到 DeepSeek **空闲时段**自动执行，避开高峰计费。空闲时段价格 = 高峰的 50%。

## 计费规则（2026-08-17 生效，2026-08-23 更新周末）

| 时段 | 北京时间 | 价格倍率 |
|:---|:---|:---:|
| 高峰 | 周一至周五 09:00-12:00、14:00-18:00 | 1.0x |
| 空闲 | 其余全部时间（含周末全天） | 0.5x |

规则由 `internal/schedule` 计算，可按 DeepSeek 官方公告调整。

## 安装

```bash
cd ~/Desktop/go-projects/belater
go build -o belater .
install -m 0755 belater ~/.local/bin/belater
```

## 快速开始

```bash
# 1. 入队一个 pi-agent 多 agent 任务（默认下次空闲时段执行）
belater add --pi "重构 auth 模块并补充测试"

# 2. 启动调度器（前台，或安装为服务）
belater daemon            # 前台运行
belater daemon --install  # 安装为 systemd 用户服务（开机自启）

# 3. 查看状态
belater status
belater list
```

## 命令

| 命令 | 说明 |
|:---|:---|
| `belater status` | 当前计费窗口 + 下次切换倒计时 + 队列概况 |
| `belater add <命令...>` | 入队任务（默认下次空闲时段执行） |
| `belater list` | 查看队列（`--all` 含已完成，`--json` 脚本输出） |
| `belater run <id>` | 立即前台执行（绕过调度） |
| `belater rm <id>` | 删除任务 |
| `belater pause <id>` / `resume <id>` | 暂停 / 恢复 |
| `belater log <id>` | 查看任务日志（`--tail N`） |
| `belater daemon` | 运行调度器（`--once` 配合 cron） |
| `belater config` | 显示调度配置 |

### add 选项

| 选项 | 说明 |
|:---|:---|
| `--pi "提示词"` | pi-agent 模式：构造 `pi -p '提示词'` 执行 |
| `--pi-args "..."` | 附加 pi 参数（如 `--no-session`、`--model ...`） |
| `--at HH:MM` | 指定时间（已过则明天；可落在高峰，用户显式指定） |
| `--delay 2h` | 从现在起延迟执行 |
| `--now` | 立即排队（等下一个 tick） |
| `--every HH:MM` | 每日重复（`--max-runs N` 限次数） |
| `--retries N` | 失败重试（指数退避 30s、60s、120s…） |
| `--timeout 30m` | 超时限制（杀进程组，防子进程残留） |
| `--cwd DIR` | 工作目录（默认当前目录；pi 任务建议保持项目根目录） |
| `--env K=V` | 环境变量（可重复） |
| `--parallel` | 不占用串行槽位（默认同时只跑 1 个串行任务） |

## 示例

```bash
# pi-agent 大任务放到晚上空闲时段跑
belater add --pi "对 quant 项目做全面 code review，输出发现清单"

# 每天凌晨 2 点自动生成报告，跑 7 次
belater add --every 02:00 --max-runs 7 --pi "生成今日代码审查报告"

# 延迟 2 小时后执行本地脚本
belater add --delay 2h ./long_task.sh

# 周末全天都是空闲 — 周五晚上入队即可
belater add --pi "跑完整测试矩阵并修复发现的问题"
```

## 调度器运行方式

### 方式 1：systemd 用户服务（推荐）

```bash
belater daemon --install   # 写入 ~/.config/systemd/user/belater.service 并启用
systemctl --user status belater
belater daemon --uninstall
```

服务崩溃自动重启（`Restart=on-failure`）；机器睡眠错过的时间会在唤醒后补跑（任务到期即执行）。

### 方式 2：cron 轮询

```bash
# crontab -e
*/5 * * * * /home/USER/.local/bin/belater daemon --once
```

每次运行检查一次队列，执行到期任务并等待完成后退出（上限 15 分钟）。

### 方式 3：前台

```bash
belater daemon   # Ctrl+C 优雅退出，运行中任务完成后再退出
```

## 状态与日志

- 状态目录：`~/.config/belater/`（`BELATER_STATE_DIR` 可覆盖）
  - `queue.json` — 任务队列（原子写入 + flock 防并发损坏）
  - `logs/<id>.log` — 每个任务的输出日志（带时间戳前缀）
- 崩溃恢复：daemon 重启时把遗留 `running` 任务复位为 `queued`
- 自动清理：完成 7 天以上的终态任务自动删除

## 设计

```
main.go → internal/cli   子命令分发
        → internal/schedule  峰谷窗口计算（周末规则）
        → internal/queue     任务模型 + JSON 持久化
        → internal/runner    执行 + 日志 + 超时杀进程组
        → internal/daemon    调度循环 + 重试 + 每日重复 + GC
```

纯 Go 标准库实现，无第三方依赖。实施计划见 `docs/plans/2026-08-24-belater.md`。

## 开发

```bash
go vet ./...
go test ./...
go build -o belater .
```
