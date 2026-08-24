# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态

belater v0.1.1 — DeepSeek 峰谷计费延迟执行器（Go 1.25 纯标准库，无第三方依赖）。
已部署 `~/.local/bin/belater`，systemd 用户服务 `belater.service` 运行中（active）。

## 最后一次完成的工作（2026-08-24）

- **v0.1.0 初版**：schedule（峰谷窗口，周末规则）/ queue（JSON + flock 原子写）/ runner（sh -c + 日志 + 超时杀组）/ daemon（tick + 恢复 + 重试 + Every + GC）/ cli 10 子命令；端到端验证通过
- **momus 批判性审查（2026-08-24）**：P0×1 + P1×3 + P2×9，结论"修复后发布"
- **v0.1.1 修复轮（全部落地）**：
  - P0-1 双派发竞态：`Store.ClaimDue`（CAS 条件更新）+ `daemon.lock` 单实例锁（LOCK_EX|LOCK_NB）——实测第二个实例报错退出
  - P1-1 管道排空上界：`<-done` 改 10s 有界等待，逃逸进程不再永久阻塞
  - P1-2 孤儿恢复：runner Result 带 Pid → Task.LastPid；recover 时 /proc 存活检查——存活标记 Failed，不重跑
  - P1-3 重试落高峰：重试时刻经 `NextOffPeakStart` 钳制；`--every` 落高峰时 add 警告
  - P2：日志末行去重、时区统一（--at/--every 按 Asia/Shanghai）、误导提示修复、文件 0600、ID 8-hex 校验、损坏 queue.json 隔离、fsync、死代码清理、NewID fallback 修正
- 测试 41 个全绿（新增：ClaimDue 竞态、孤儿恢复、重试钳制、逃逸进程有界排空、末行去重、实例锁实测）
- 端到端复验通过（systemd 服务执行 v0.1.1 任务）

## 遗留问题 / 待办

- [ ] 计费规则硬编码在 `internal/schedule`（DefaultSchedule）——DeepSeek 再改规则时需改代码；可考虑 config.json 外部化（v0.2.0 候选）
- [ ] `--every` 任务首次 RunAt 默认排到下次空闲时段而非下一个 every 时刻——白天入队 `--every 02:00` 会先在下个空闲（18:00 后）跑一次再循环；要严格首次即 02:00 需加 `--first-at` 选项
- [ ] 并发测试 `TestConcurrentAdds` 依赖 flock，慢机可能 flaky（200 个 Add 串行 flock，~2s）
- [ ] daemon 单实例锁已实现（daemon.lock）——但 `belater run <id>` 前台执行不受锁约束，与 daemon 并行跑同一任务仍可能双执行（`run` 是显式用户操作，接受此风险）
- [ ] runner 用 `/bin/sh -c`——含中文路径的命令在非 UTF-8 locale 环境可能异常（本机 OK）
- [ ] `--once` 模式下 waitRunning 上限 10 分钟——超长任务用 systemd 常驻方式

## 远程资源

无远程依赖。本地：`~/.config/belater/` 状态目录（queue.json + logs/ + daemon.lock）。

## 知识图谱

- graphify-out/: 存在（2026-08-24 v0.1.1 后需重建）

## 最后更新时间

2026-08-24 15:30
