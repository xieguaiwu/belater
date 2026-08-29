# Graph Report - belater  (2026-08-29)

## Corpus Check
- 28 files · ~21,355 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 319 nodes · 736 edges · 15 communities (14 shown, 1 thin omitted)
- Extraction: 81% EXTRACTED · 19% INFERRED · 0% AMBIGUOUS · INFERRED: 142 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `b470a509`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Main
- daemon_test.go
- Store
- Schedule
- belater — DeepSeek 峰谷计费延迟执行器
- statusLines
- newTestStore
- Global Constraints
- Changelog
- DefaultSchedule
- cmdAdd
- runTask
- belater
- CONTEXT_FOR_NEXT_AGENT.md
- Open

## God Nodes (most connected - your core abstractions)
1. `Store` - 29 edges
2. `Task` - 27 edges
3. `DefaultSchedule()` - 21 edges
4. `statusLines()` - 19 edges
5. `Schedule` - 19 edges
6. `newTestStore()` - 16 edges
7. `Main()` - 14 edges
8. `Run()` - 14 edges
9. `newTestStore()` - 14 edges
10. `openStore()` - 13 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `Main()`  [INFERRED]
  main.go → internal/cli/cli.go
- `Main()` --calls--> `cmdAdd()`  [INFERRED]
  internal/cli/cli.go → internal/cli/cmd_add.go
- `Main()` --calls--> `cmdConfig()`  [INFERRED]
  internal/cli/cli.go → internal/cli/cmd_config.go
- `Main()` --calls--> `cmdDaemon()`  [INFERRED]
  internal/cli/cli.go → internal/cli/cmd_daemon.go
- `Main()` --calls--> `cmdStatus()`  [INFERRED]
  internal/cli/cli.go → internal/cli/cmd_status.go

## Import Cycles
- None detected.

## Communities (15 total, 1 thin omitted)

### Community 0 - "Main"
Cohesion: 0.10
Nodes (38): resultRow, File, FlagSet, calendarDayDiff(), fmtAge(), fmtDuration(), fmtName(), fmtRemaining() (+30 more)

### Community 1 - "daemon_test.go"
Cohesion: 0.13
Nodes (33): Context, Config, acquireInstanceLock(), dispatchOnce(), finishTask(), gc(), Duration, Time (+25 more)

### Community 2 - "Store"
Cohesion: 0.22
Nodes (5): Time, Mutex, Status, Store, Task

### Community 3 - "Schedule"
Cohesion: 0.28
Nodes (4): Location, Time, Kind, Schedule

### Community 4 - "belater — DeepSeek 峰谷计费延迟执行器"
Cohesion: 0.07
Nodes (26): add 选项, belater — DeepSeek 峰谷计费延迟执行器, add options, belater — Defer big tasks to DeepSeek off-peak billing hours, Commands, Design, Development, Install (+18 more)

### Community 5 - "statusLines"
Cohesion: 0.19
Nodes (27): age(), cmdStatus(), countTasks(), Duration, Location, Time, isWeekendIn(), overdueQueued() (+19 more)

### Community 6 - "newTestStore"
Cohesion: 0.38
Nodes (16): T, newTestStore(), sampleTask(), TestAddLoadRoundtrip(), TestClaimForRun(), TestCompareAndStatus(), TestConcurrentAdds(), TestEnvRoundtrip() (+8 more)

### Community 7 - "Global Constraints"
Cohesion: 0.18
Nodes (10): belater — DeepSeek 峰谷计费延迟执行器 Implementation Plan, Global Constraints, Self-Review 记录（写完计划时执行）, Task 1: 项目骨架 + schedule 包（核心窗口逻辑）, Task 2: queue 包（任务模型 + JSON 持久化）, Task 3: runner 包（任务执行 + 日志 + 重试）, Task 4: daemon 包（调度循环 + 派发 + 崩溃恢复）, Task 5: CLI 子命令 (+2 more)

### Community 8 - "Changelog"
Cohesion: 0.12
Nodes (16): [0.1.0] - 2026-08-24, [0.1.1] - 2026-08-24, [0.2.0] - 2026-08-24, [0.2.1] - 2026-08-25, [0.2.2] - 2026-08-25, [0.2.3] - 2026-08-27, [0.2.4] - 2026-08-29, Added (+8 more)

### Community 9 - "DefaultSchedule"
Cohesion: 0.51
Nodes (9): DefaultSchedule(), T, Time, mustTime(), TestDescribe(), TestKindAt(), TestMultiplier(), TestNextOffPeakStart() (+1 more)

### Community 10 - "cmdAdd"
Cohesion: 0.13
Nodes (23): envList, defaultSchedule(), stateDir(), cmdAdd(), Location, Time, parseHHMM(), shQuote() (+15 more)

### Community 11 - "runTask"
Cohesion: 0.18
Nodes (22): Cmd, runTaskForeground(), buildEnv(), Time, Writer, inPath(), killProcessGroup(), Run() (+14 more)

### Community 13 - "CONTEXT_FOR_NEXT_AGENT.md"
Cohesion: 0.22
Nodes (7): 已验证的关键行为, 最后一次完成的工作（2026-08-29）, 最后更新时间, 知识图谱, 远程资源, 遗留问题 / 待办, 项目当前状态

### Community 14 - "Open"
Cohesion: 0.16
Nodes (19): Heartbeat, SchedulerState, ceilSeconds(), Duration, Time, HeartbeatPath(), ProbeScheduler(), ReadHeartbeat() (+11 more)

## Knowledge Gaps
- **46 isolated node(s):** `belater`, `Fixed`, `Added`, `Fixed`, `Fixed` (+41 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Task` connect `Store` to `Main`, `daemon_test.go`, `statusLines`, `newTestStore`, `runTask`?**
  _High betweenness centrality (0.180) - this node is a cross-community bridge._
- **Why does `statusLines()` connect `statusLines` to `Main`, `Store`, `Schedule`, `Open`?**
  _High betweenness centrality (0.120) - this node is a cross-community bridge._
- **Why does `Store` connect `Store` to `Main`, `daemon_test.go`, `newTestStore`, `cmdAdd`, `Open`?**
  _High betweenness centrality (0.100) - this node is a cross-community bridge._
- **Are the 19 inferred relationships involving `DefaultSchedule()` (e.g. with `defaultSchedule()` and `TestStatusLinesHeartbeatWithoutVersion()`) actually correct?**
  _`DefaultSchedule()` has 19 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `statusLines()` (e.g. with `fmtAge()` and `fmtDuration()`) actually correct?**
  _`statusLines()` has 9 INFERRED edges - model-reasoned connections that need verification._
- **What connects `belater`, `Fixed`, `Added` to the rest of the system?**
  _46 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Main` be split into smaller, more focused modules?**
  _Cohesion score 0.10188261351052048 - nodes in this community are weakly interconnected._