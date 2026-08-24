# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态

belater v0.2.0 — DeepSeek 峰谷计费延迟执行器（Go 1.25 纯标准库，零第三方依赖）。
**公开仓库**: https://github.com/xieguaiwu/belater（PUBLIC, master 分支, 8 topics, CI 全绿）。
已部署 `~/.local/bin/belater`，systemd 用户服务 `belater.service` 运行中（active）。

## 最后一次完成的工作（2026-08-24）

- **v0.1.0 初版 + momus 审查（P0×1/P1×3/P2×9）+ v0.1.1 修复轮**（详见历史）
- **v0.2.0 结果查看增强**：
  - `belater results`：已完成任务汇总（状态/退出码/耗时/完成时间，`--days N`、`--json`，按完成时间倒序）
  - `belater log <id>`：头部元信息（状态/退出码/耗时/完成时间/命令）；`--tail N` 支持放 ID 后（`log <id> --tail 3`）
  - `--on-done "cmd"`：完成钩子（env: BELATER_TASK_ID/NAME/STATUS/EXIT；daemon 与前台 run 均触发；钩子失败不影响任务状态）
  - `--notify`：桌面通知糖（notify-send，需桌面 DBus 会话）
  - Task 新增 DurationSec/OnDone 字段；测试 45 个全绿
- **公开发布**：gh repo create --public（xieguaiwu/belater）+ README.md/README_en.md 双语 + MIT LICENSE + GitHub Actions CI（vet/test/build/smoke，34s 全绿）+ topics（go/cli/scheduler/deepseek/pi-agent/off-peak/queue/automation）
- **坑**：CI workflow `branches: [main]` 不触发（默认分支 master）→ 改为任意分支触发

## 遗留问题 / 待办

- [ ] 计费规则硬编码在 `internal/schedule`（DefaultSchedule）——DeepSeek 再改规则需改代码；config.json 外部化（v0.3.0 候选）
- [ ] `--every` 首次 RunAt 默认排下次空闲而非下一个 every 时刻（白天入队 `--every 02:00` 先在下个空闲跑一次再循环）——`--first-at` 选项候选
- [ ] CI warning：actions/checkout@v4/setup-go@v5 的 Node 20 deprecation（不影响功能，可升 v5/v6）
- [ ] `--notify` 在无桌面 DBus 会话时静默失败（钩子错误记日志）——文档已说明
- [ ] `belater run <id>` 前台执行不受 daemon.lock 约束（显式用户操作，接受）
- [ ] 并发测试 `TestConcurrentAdds` 依赖 flock，慢机可能 flaky

## 远程资源

无远程依赖。本地：`~/.config/belater/`（queue.json + logs/ + daemon.lock）。
GitHub: xieguaiwu/belater（gh CLI 已认证）。

## 知识图谱

- graphify-out/: 存在（2026-08-24 v0.2.0 后重建，225 节点）

## 最后更新时间

2026-08-24 15:45
