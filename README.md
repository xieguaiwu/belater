# belater — DeepSeek 峰谷计费延迟执行器

> GitHub: https://github.com/xieguaiwu/belater · 中文文档 | [English](README_en.md)

把大型任务（如 pi-agent 多 agent 工作）排队到 DeepSeek **空闲时段**自动执行，避开高峰计费。空闲时段价格 = 高峰的 50%。

## 计费规则（2026-08-17 生效，2026-08-23 更新周末；2026-08-29 对照官方定价页复核一致）

| 时段 | 北京时间 | 价格倍率 |
|:---|:---|:---:|
| 高峰 | 周一至周五 09:00-12:00、14:00-18:00 | 1.0x |
| 空闲 | 其余全部时间（含周末全天） | 0.5x |

规则由 `internal/schedule` 计算，可按 DeepSeek 官方公告调整（https://api-docs.deepseek.com/zh-cn/quick_start/pricing）。

## 安装

```bash
git clone https://github.com/xieguaiwu/belater.git
cd belater
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
| `belater status` | 当前计费窗口 + 下次切换倒计时（跨天带日期）+ **调度器判活（锁 + 心跳）** + 到期未执行告警 + 队列概况 |
| `belater add <命令...>` | 入队任务（默认下次空闲时段执行） |
| `belater list` | 查看队列（`--all` 含已完成，`--json` 脚本输出） |
| `belater results` | **已完成任务结果汇总**（状态/退出码/耗时/完成时间，`--days N`、`--json`） |
| `belater run <id>` | 立即前台执行（绕过调度） |
| `belater rm <id>` | 删除任务 |
| `belater pause <id>` / `resume <id>` | 暂停 / 恢复 |
| `belater log <id>` | 查看任务日志（头部含状态/退出码/耗时元信息；`--tail N`，可放 ID 后） |
| `belater daemon` | 运行调度器（`--once` 配合 cron；已有调度器运行时打印提示并以 0 退出） |
| `belater config` | 显示调度配置 |

### add 选项

| 选项 | 说明 |
|:---|:---|
| `--pi "提示词"` | pi-agent 模式：构造 `pi -p '提示词'` 执行 |
| `--pi-args "..."` | 附加 pi 参数（按 shell 片段原样拼入，如 `--no-session`、`--model ...`） |
| `--at HH:MM` | 指定时间（已过则明天；可落在高峰，用户显式指定） |
| `--delay 2h` | 从现在起延迟执行 |
| `--now` | 立即排队（等下一个 tick） |
| `--every HH:MM` | 每日重复（`--max-runs N` 限次数） |
| `--retries N` | 失败重试（指数退避 30s、60s、120s…） |
| `--timeout 30m` | 超时限制（杀进程组，防子进程残留） |
| `--cwd DIR` | 工作目录（默认当前目录；pi 任务建议保持项目根目录） |
| `--env K=V` | 环境变量（可重复） |
| `--parallel` | 不占用串行槽位（默认同时只跑 1 个串行任务） |
| `--on-done "命令"` | 完成后执行命令（env: `BELATER_TASK_ID/NAME/STATUS/EXIT`） |
| `--notify` | 完成后桌面通知（`notify-send`，等价于预设的 --on-done） |

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

## 查看结果

```bash
belater results        # 最近 7 天完成任务汇总（状态/退出码/耗时）
belater log <id>       # 单任务日志（头部含元信息，--tail 只看尾部）
belater list --all     # 含终态任务的完整队列
```

任务完成时自动通知：

```bash
belater add --notify --pi "跑完整测试矩阵"        # 桌面通知（notify-send）
belater add --on-done "curl -X POST ..." --pi "部署" # 完成回调（env 注入任务信息）
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

> ⚠️ **方式 1 与方式 2 互斥**：调度器持有单实例锁（`daemon.lock`），同时跑两个调度器（如 systemd 服务 + cron --once）时，后启动者会报错退出——防止同一任务被派发两次。切换方式前先停掉另一个（`systemctl --user stop belater` 或移除 cron 行）。

### 方式 3：前台

```bash
belater daemon   # Ctrl+C 优雅退出，运行中任务完成后再退出
```

## 状态与日志

```bash
belater status
```

```text
当前时段: 空闲 (0.5x 价格) · 周末全天空闲
下次切换: 周一 09:00 高峰 (1d 16h 55m 后)
现在就是空闲时段 — 新任务将在下次调度检查（约 15s 内）执行
调度器: 运行中 (v0.2.4 · tick 15s · 心跳 1s 前) — 到期任务会自动执行
队列: 0 待执行 · 0 运行中 · 2 已完成
```

切换时刻超过一天时带上日期（`明天 09:00`、`周一 09:00`）。「调度器」一行读心跳文件：锁持有但心跳过期、调度器版本低于本命令、或有任务到期未执行时，各出一行警告并给出恢复命令。`belater status --json` 供脚本使用（含 `scheduler_running`、`scheduler_stalled`、`overdue_queued`、`next_switch_label`）。

- 状态目录：`~/.config/belater/`（`BELATER_STATE_DIR` 可覆盖）
  - `queue.json` — 任务队列（原子写入 + flock 串行化写；读不加锁）
  - `logs/<id>.log` — 每个任务的输出日志（带时间戳前缀）
  - `daemon.lock` — 单实例锁
  - `daemon.heartbeat` — 调度器每个 tick 写入的存活证据（pid / 版本 / tick / 上次检查时刻）
- **PATH 处理**：daemon 环境（systemd/cron）缺少交互 shell 的用户 bin 目录（`~/.npm-global/bin`、`~/.local/bin`、`~/go/bin`）时，任务执行会自动补全 PATH——`pi` 等安装在用户目录的工具无需额外配置；任务显式设置 `--env PATH=...` 时以任务为准
- 队列读写：写路径持 `queue.lock` 并以 tmp+rename 发布，读路径不加锁，所以 `status` / `list` 不会排在调度器后面
- 到期未执行：`belater list` 把过了计划时刻的任务显示为「已到期 Xm」
- 崩溃恢复：daemon 重启时把遗留 `running` 任务复位为 `queued`；任务 last pid 仍存活时标记 `failed`（不重复执行）
- 自动清理：完成 7 天以上的终态任务自动删除

升级二进制后记得重启调度器：服务不会自动换新版，`systemctl --user restart belater`（`belater status` 会告警提示版本落后）。

## 设计

```
main.go → internal/cli   子命令分发
        → internal/schedule  峰谷窗口计算（周末规则）
        → internal/queue     任务模型 + JSON 持久化
        → internal/runner    执行 + 日志 + 超时杀进程组
        → internal/daemon    调度循环 + 重试 + 每日重复 + GC + 心跳
```

纯 Go 标准库实现，无第三方依赖。实施计划见 `docs/plans/2026-08-24-belater.md`。

## 开发

```bash
go vet ./...
go test ./...
go build -o belater .
```

改动调度相关显示时保持这三条契约：

1. 峰谷规则只依官方定价页，改规则 = 同时改 `internal/schedule`、README 两处与 `docs/plans` 约束段。
2. 「运行中」以持有 `daemon.lock` 为准，但「无告警」还要求心跳新鲜且版本与本命令一致；不得只凭锁在就宣称调度器健康。
3. 队列读路径不得拿 `queue.lock`：写方靠 tmp+rename 发布，读方加锁只会把读命令排到调度器后面。
