# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态

belater v0.1.0 — DeepSeek 峰谷计费延迟执行器（Go 1.25 纯标准库，无第三方依赖）。
已部署 `~/.local/bin/belater`，systemd 用户服务 `belater.service` 已安装并运行中（active）。

## 最后一次完成的工作（2026-08-24）

- 完整实现：schedule（峰谷窗口计算，含周末规则）/ queue（JSON 持久化 + flock 原子写）/ runner（sh -c 执行 + 时间戳日志 + 超时杀进程组）/ daemon（调度循环 + 崩溃恢复 + 指数退避重试 + Every 每日重复 + GC）/ cli（10 个子命令）
- 端到端验证：`belater add --pi "..."` → systemd 服务自动执行 → 日志落盘 → done ✅
- 实施计划：`docs/plans/2026-08-24-belater.md`（自足、无占位符）
- git 历史：7 commits（初始到 v0.1.0 验证）
- graphify-out/ 已构建（185 节点，God Nodes: Store/Task/Schedule）

## 遗留问题 / 待办

- [ ] 计费规则硬编码在 `internal/schedule`（DefaultSchedule）——DeepSeek 再改规则时需改代码；可考虑 config.json 外部化（v0.2.0 候选）
- [ ] `--every` 任务首次 RunAt 默认排到下次空闲时段而非下一个 every 时刻——语义上可用但注意：`--every 02:00` 在白天入队会先在下个空闲（18:00 后）跑一次，再按 02:00 循环。若要严格首次即 02:00 需加 `--first-at` 选项
- [ ] 并发测试 `TestConcurrentAdds` 依赖 flock，慢机可能 flaky（200 个 Add 串行 flock，~2s）
- [ ] daemon 无单独 PID/状态文件——多实例同时 `belater daemon` 会重复派发（flock 防损坏但不防双派发）；真实场景由 systemd 保证单实例
- [ ] runner 用 `/bin/sh -c`——含中文路径的命令在非 UTF-8 locale 环境可能异常（本机 OK）

## 远程资源

无远程依赖。本地：`~/.config/belater/` 状态目录（queue.json + logs/）。

## 知识图谱

- graphify-out/: 存在（2026-08-24 构建）
- God Nodes: Store(26) / Task(18) / Schedule(15)

## 最后更新时间

2026-08-24 15:10
