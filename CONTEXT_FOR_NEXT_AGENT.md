# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态

belater v0.2.2 — DeepSeek 峰谷计费延迟执行器（Go 1.25 纯标准库，零第三方依赖）。
**公开仓库**: https://github.com/xieguaiwu/belater（PUBLIC, master, 8 topics, MIT, CI 全绿）。
已部署 `~/.local/bin/belater`，systemd 用户服务运行中。

**已实战验证**：2026-08-24/25 完成首个真实任务闭环——rebirth 漏洞检查任务（pi 36 分钟完成：复现挂起 → 定位根因 → v0.8.0 修复熔断器 → 测试 → 推送 → 部署），全链路排队→调度→执行→日志→结果可用。当天唯一失败（18:00 exit 127）为 PATH bug，已修复并验证。

## 最后一次完成的工作（2026-08-25）

- **v0.2.1**：
  - PATH 修复：runner 自动补用户 bin 目录到 PATH（systemd 环境缺 `~/.npm-global/bin` 导致 pi command not found / exit 127）；unit 加 `Environment=PATH=%h/...`
  - daemon 锁冲突预检：横幅不再先于错误打印；`--once` 同样处理
- **v0.2.0 结果查看**：`results` 聚合视图、`log <id>` 元信息头、`--on-done`/`--notify` 钩子
- **UX**：status 显示调度器存活状态；add 检测调度器在跑时提示"任务将自动执行"；`log <id> --tail N` 支持 ID 在前
- 版本号与 CHANGELOG.md 对齐（此前 fix 未升版）

## 已验证的关键行为

- 峰谷窗口：周二 11:52 add 默认排 12:00 午间空隙 ✓；周末全天空闲 ✓
- 单实例锁：手动 `daemon` 被拒并给出指引 ✓
- 服务环境执行 pi：73595fb5 "路径修复成功" exit 0（9s）✓
- 大任务：7f1d18c8 rebirth 检查修复 exit 0（36m）✓

## 遗留问题 / 待办

- [ ] 计费规则硬编码 `internal/schedule`（DefaultSchedule）——DeepSeek 改规则需改代码；config.json 外部化（v0.3.0 候选）
- [ ] `--every HH:MM` 首次 RunAt 排下次空闲而非下一个 every 时刻——`--first-at` 选项候选
- [ ] 并发测试 `TestConcurrentAdds` 依赖 flock，慢机可能 flaky
（v0.2.2 已修：pause/resume/run TOCTOU CAS 化；--every 首次语义改为下一个 every 时刻；--pi-args 文档标注；CI actions 升级；results 空文案）

## 远程资源

无远程依赖。本地状态目录 `~/.config/belater/`（queue.json + logs/ + daemon.lock + queue.lock）。
GitHub: xieguaiwu/belater（gh CLI 已认证）。

## 知识图谱

- graphify-out/: 存在（随代码变更重建）

## 最后更新时间

2026-08-25 12:10
