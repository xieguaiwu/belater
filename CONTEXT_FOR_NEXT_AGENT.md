# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态

belater v0.2.4 — DeepSeek 峰谷计费延迟执行器（Go 1.25 纯标准库，零第三方依赖）。
**公开仓库**: https://github.com/xieguaiwu/belater（PUBLIC, master, 8 topics, MIT, CI 全绿）。
已部署 `~/.local/bin/belater`（v0.2.4，2026-08-29 16:05），systemd 用户服务已重启到新二进制。

**已实战验证**：2026-08-24/25 完成首个真实任务闭环——rebirth 漏洞检查任务（pi 36 分钟完成：复现挂起 → 定位根因 → v0.8.0 修复熔断器 → 测试 → 推送 → 部署），全链路排队→调度→执行→日志→结果可用。当天唯一失败（18:00 exit 127）为 PATH bug，已修复并验证。

## 最后一次完成的工作（2026-08-29）

用户报「status 高峰时段显示明显落后」。先证伪两件事，再修四项（v0.2.4）：

- **规则没落后**：抓官方定价页（api-docs.deepseek.com/zh-cn/quick_start/pricing）——高峰=北京时间周一至周五 09:00-12:00、14:00-18:00，其余（含周末全天）空闲，与 `internal/schedule` 完全一致；另跑逐分钟扫一周（10080 采样）对比独立参考实现，KindAt/NextSwitch **0 处不符**。故 schedule 包**一行未改**。
- **数值没算错**：用户那次运行时刻从 fish history `when:1787989791` 定为 08-29 15:49:51（周六），下次切换周一 09:00 = 1d 17h 10m，与显示一致；`status` 实测 3–5ms、strace 无慢 syscall，10s 未复现。

修的四项：

1. **下次切换缺日期**（周六的「09:00」实为周一）→ `fmtSwitchTime` + `calendarDayDiff`（`math.Round` 处理 DST 23/25 小时），今天裸时钟 / 明天前缀 / 更远用星期。
2. **「立即排队执行」不实**：调度器按 tick 轮询 → 文案改「下次调度检查（约 15s 内）执行」，tick 取运行中调度器心跳的真实值。高峰分支同时改用 `NextOffPeakStart`（旧代码把「下次边界」当「下次空闲」，靠窗口交替才侥幸正确）。
3. **判活只看 flock** → 新增 `internal/daemon/heartbeat.go`：调度器每次派发后原子写 `daemon.heartbeat`（pid/version/tick/last_tick_at，0600，写失败只记一次日志）。status 据此报心跳年龄，并对三种情况各出一行警告：循环卡住（锁在、心跳超 3×tick+30s）、**调度器版本低于本命令**（服务不会自动换二进制）、有 Queued 任务过期 >60s 未派发。实况：当时跑的 daemon 是 08-26 23:43 启动的旧版（磁盘二进制 08-27 17:25 已升 v0.2.3）→ 现已重启到 v0.2.4 并确认心跳在走。
4. **读路径排在校验锁后面**：`Store.Load` 原本拿阻塞 `flock(LOCK_EX)`（唯一能解释多秒卡顿的代码路径）→ 改为不加锁（写方 tmp+rename 原子发布，读方必见完整文件），`readLocked`→`read`。写路径 `mutate`/`Save` 持锁不变。附带：`belater list` 过期任务原显示正数「剩余」，现显示「已到期 Xm」（`fmtRemaining`）。

测试：新增 `internal/daemon/heartbeat_test.go`（7）、`internal/cli/status_test.go`（6 组，含周末/高峰/深夜/三种告警/无心跳）、`store_test.go` 加 `TestLoadDoesNotWaitForTheWriteLock`（外部持锁时 Load 必不阻塞）+ `TestLoadSeesCommittedWrites`。`go vet ./... && go test ./...` 全绿，`gofmt -l .` 空。

## 已验证的关键行为

- 峰谷窗口：周二 11:52 add 默认排 12:00 午间空隙 ✓；周末全天空闲 ✓
- 单实例锁：手动 `daemon` 在已有调度器时打印提示并退出 0（不再报错）✓
- 服务环境执行 pi：73595fb5 "路径修复成功" exit 0（9s）✓
- 大任务：7f1d18c8 rebirth 检查修复 exit 0（36m）✓
- **v0.2.4 端到端**（08-29 16:05，真服务）：restart 后 status 显示 `v0.2.4 · tick 15s · 心跳 1s 前` 无告警；`add --now echo` 在同一 tick 内派发（16:05:37 done exit 0）并验证 `last_tick_at` 前进；临时状态目录造旧版+旧心跳+过期任务，三行告警同时命中 ✓
- **读不加锁回归**：daemon 持 `queue.lock` 时 `Load` 立即返回（旧代码会等到释放）✓

## 遗留问题 / 待办

- [ ] 计费规则硬编码 `internal/schedule`（DefaultSchedule）——DeepSeek 改规则需改代码；config.json 外部化（v0.3.0 候选）。本轮已人工复核官方页一致（2026-08-29），但未做自动校对。
- [ ] `daemon.gc()` 与 `recoverInterrupted()` 是 `Load` 后整体 `Save`：读→写之间入队的新任务会被覆盖丢失。该窗口早于 v0.2.4 就存在（v0.2.4 去掉读锁没有改变其性质）。最小修法：导出 `Store.Mutate(fn)` 戒 `PruneFinished(cutoff)`，把过滤放进单次 flock 里。
- [ ] 升级二进制后服务不会自动换新版（`Restart=on-failure` 只在崩溃时重生）。v0.2.4 已加版本落后告警；自动检测（systemd path unit 戒 ExecStart 哈希比对）待评估。
- [ ] 并发测试 `TestConcurrentAdds` 依赖 flock，慢机可能 flaky
- [ ] status 输出文本硬编码中文（无 i18n）；README_en 已标注。若开源面向海外用户可考虑 `--lang` 或 english 文案
（v0.2.2 已修：pause/resume/run TOCTOU CAS 化；--every 首次语义改为下一个 every 时刻（原本清单里的「待办」属记录陈旧，已删）；--pi-args 文档标注；CI actions 升级；results 空文案）

## 远程资源

无远程依赖。本地状态目录 `~/.config/belater/`（queue.json + logs/ + daemon.lock + queue.lock）。
GitHub: xieguaiwu/belater（gh CLI 已认证）。

## 知识图谱

- graphify-out/: 存在（随代码变更重建）

## 最后更新时间

2026-08-29 16:20
