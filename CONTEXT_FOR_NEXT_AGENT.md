# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态

belater v0.3.0 — DeepSeek 峰谷计费延迟执行器（Go 1.25 纯标准库，零第三方依赖）。
**公开仓库**: https://github.com/xieguaiwu/belater（PUBLIC, master, 8 topics, MIT, CI 全绿）。
已部署 `~/.local/bin/belater`（v0.3.0，2026-08-30），systemd 用户服务已重启到新二进制（`status` 显示 `v0.3.0 · tick 15s · 心跳新鲜` 无告警）。

**已实战验证**：2026-08-24/25 完成首个真实任务闭环——rebirth 漏洞检查任务（pi 36 分钟完成：复现挂起 → 定位根因 → v0.8.0 修复熔断器 → 测试 → 推送 → 部署），全链路排队→调度→执行→日志→结果可用。当天唯一失败（18:00 exit 127）为 PATH bug，已修复并验证。

## 最后一次完成的工作（2026-08-30）

用户需求「自动在指定时间开始工作」→ 新增 **`--at` 精确日期时刻**（v0.3.0）：

- `--at "YYYY-MM-DD HH:MM"`（含 `T` 分隔；只写日期 = 当天 00:00）——任务到点自动开始执行。时间按计费时区（Asia/Shanghai）解释，与既有 `HH:MM` 分支一致。
- 裸 `HH:MM` 保留原语义（今天，已过则明天）；带日期的时刻**已过即报错退出 2**，不静默顺延——明确指定的时刻不允许漂移。
- 实现集中在 `internal/cli/cmd_add.go` 的 `parseAtTime()`（新增，含错误返回值）；`parseHHMM()` 原样保留给 `--every` 与裸时钟分支。调度器派发条件本就是 `RunAt ≤ now`，**daemon 一行未改**。
- 测试：`TestParseAtTime` 表驱动（7 个合法输入 + 4 个已过时刻必须报错 + 6 个无效输入必须拒绝），`TestParseHHMM` 原样未动（回归证明 `--every` 不受影响）。
- 端到端（真二进制 + 临时状态目录）：未来日期时刻 / 裸日期午夜 / `T` 分隔 / 裸 `HH:MM` 四种入队全部正确显示在 `list`；已过时刻与坏格式均 exit 2 且错误信息可操作（`--at 指定的时间已过: ...` / `--at 格式应为 HH:MM 或 "YYYY-MM-DD HH:MM"`）。
- 版本 0.2.4 → 0.3.0（新语义属功能新增）；`docs/plans` 追加修订记录；两 README、CHANGELOG、help 示例同步。

上一轮（2026-08-29，v0.2.4）：修「下次切换缺日期」「立即排队不实」「判活只看锁」「读路径排在写锁后」四项，新增心跳判活与三条长期契约（见下）。

## 已验证的关键行为

- 峰谷窗口：周二 11:52 add 默认排 12:00 午间空隙 ✓；周末全天空闲 ✓
- 单实例锁：手动 `daemon` 在已有调度器时打印提示并退出 0（不再报错）✓
- 服务环境执行 pi：73595fb5 "路径修复成功" exit 0（9s）✓
- 大任务：7f1d18c8 rebirth 检查修复 exit 0（36m）✓
- **v0.2.4 端到端**（08-29 16:05，真服务）：三行告警（旧版/心跳过期/过期任务）同时命中 ✓；读不加锁回归 ✓
- **v0.3.0 端到端**（08-30，真服务 + 临时目录）：`--at "2026-09-01 09:00"` 入队显示 `高峰时段（用户指定） 执行 (2d 6h 5m 后)` ✓；裸日期 → 00:00 ✓；已过时刻/坏格式 exit 2 ✓；服务重启后 `status` 显示 v0.3.0 心跳新鲜无告警 ✓
- 测试规模：68 个 top-level test（含子测试 103 个），`go test -race -count=1` 全绿（2026-08-30）
- 文档基线：五个文档（README.md / README_en.md / CHANGELOG.md / 本文件 / docs/plans 计划与修订记录）版本均记 0.3.0；graphify 基线 = 最后一个**改代码**的 commit（`graphify-out/` 自身的提交 commit 不改代码，故 `Built from commit` 会比 HEAD 早一个）
- ⚠️ README 命名惯例与 `development-quality-gates.md` 关卡 8（README.md 英 + README_zh.md 中）**相反**：本项目历史采用 README.md 中文主文 + README_en.md，维持现状不翻搅

## 遗留问题 / 待办

- [ ] 计费规则硬编码 `internal/schedule`（DefaultSchedule）——DeepSeek 改规则需改代码；config.json 外部化（候选，原记 v0.3.0 位，本版占用后顺延）。2026-08-29 已人工复核官方页一致，但未做自动校对。
- [ ] `daemon.gc()` 与 `recoverInterrupted()` 是 `Load` 后整体 `Save`：读→写之间入队的新任务会被覆盖丢失。该窗口早于 v0.2.4 就存在。最小修法：导出 `Store.Mutate(fn)` 或 `PruneFinished(cutoff)`，把过滤放进单次 flock 里。
- [ ] 升级二进制后服务不会自动换新版（`Restart=on-failure` 只在崩溃时重生）。v0.2.4 已加版本落后告警；自动检测（systemd path unit 或 ExecStart 哈希比对）待评估。
- [ ] `--every` 仅支持每日 `HH:MM`——每周/每月重复未实现（用户若要「每周一 09:00」目前只能手动排多个 `--at`）。
- [ ] 并发测试 `TestConcurrentAdds` 依赖 flock，慢机可能 flaky
- [ ] status 输出文本硬编码中文（无 i18n）；README_en 已标注。若开源面向海外用户可考虑 `--lang` 或 english 文案
（v0.2.2 已修：pause/resume/run TOCTOU CAS 化；--every 首次语义改为下一个 every 时刻；--pi-args 文档标注；CI actions 升级；results 空文案）

## 三条契约（v0.2.4 起，任何改动不得违反）

1. 峰谷规则只依官方定价页，改规则 = 同改 `internal/schedule`、两个 README 与 `docs/plans` 约束段。
2. 「运行中」以持 `daemon.lock` 为准，但「无告警」还要求心跳新鲜且版本与本命令一致；等待期必须继续 pulse。
3. 队列读路径不得拿 `queue.lock`：写方 tmp+rename 原子发布，读方加锁只会把读命令排到调度器后面。

## 远程资源

无远程依赖。本地状态目录 `~/.config/belater/`（queue.json + logs/ + daemon.lock + queue.lock + daemon.heartbeat）。
GitHub: xieguaiwu/belater（gh CLI 已认证）。

## 知识图谱

- graphify-out/: 存在，2026-08-30 重建（325 节点 / 748 边 / 15 社区）

## 最后更新时间

2026-08-30 03:10
