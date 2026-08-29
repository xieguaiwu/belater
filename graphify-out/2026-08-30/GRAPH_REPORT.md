# Graph Report - belater  (2026-08-29)

## Corpus Check
- 28 files · ~21,474 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 320 nodes · 737 edges · 15 communities (14 shown, 1 thin omitted)
- Extraction: 81% EXTRACTED · 19% INFERRED · 0% AMBIGUOUS · INFERRED: 142 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d09aa74c`
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
- collectResults
- runTask
- belater
- CONTEXT_FOR_NEXT_AGENT.md
- SchedulerState

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
- `Main()` --calls--> `cmdStatus()`  [INFERRED]
  internal/cli/cli.go → internal/cli/cmd_status.go
- `cmdStatus()` --calls--> `stateDir()`  [INFERRED]
  internal/cli/cmd_status.go → internal/cli/cli.go
- `openStore()` --calls--> `Open()`  [INFERRED]
  internal/cli/cli.go → internal/queue/store.go
- `cmdStatus()` --calls--> `openStore()`  [INFERRED]
  internal/cli/cmd_status.go → internal/cli/cli.go

## Import Cycles
- None detected.

## Communities (15 total, 1 thin omitted)

### Community 0 - "Main"
Cohesion: 0.11
Nodes (40): File, FlagSet, defaultSchedule(), fmtDuration(), fmtName(), fmtRemaining(), fmtRunAt(), Duration (+32 more)

### Community 1 - "daemon_test.go"
Cohesion: 0.13
Nodes (33): Context, Config, acquireInstanceLock(), dispatchOnce(), finishTask(), gc(), Duration, Time (+25 more)

### Community 2 - "Store"
Cohesion: 0.20
Nodes (6): Open(), Time, Mutex, Status, Store, Task

### Community 3 - "Schedule"
Cohesion: 0.28
Nodes (4): Location, Time, Kind, Schedule

### Community 4 - "belater — DeepSeek 峰谷计费延迟执行器"
Cohesion: 0.07
Nodes (26): add 选项, belater — DeepSeek 峰谷计费延迟执行器, add options, belater — Defer big tasks to DeepSeek off-peak billing hours, Commands, Design, Development, Install (+18 more)

### Community 5 - "statusLines"
Cohesion: 0.16
Nodes (32): calendarDayDiff(), fmtAge(), fmtSwitchTime(), Location, Time, age(), cmdStatus(), countTasks() (+24 more)

### Community 6 - "newTestStore"
Cohesion: 0.38
Nodes (16): T, newTestStore(), sampleTask(), TestAddLoadRoundtrip(), TestClaimForRun(), TestCompareAndStatus(), TestConcurrentAdds(), TestEnvRoundtrip() (+8 more)

### Community 7 - "Global Constraints"
Cohesion: 0.17
Nodes (11): belater — DeepSeek 峰谷计费延迟执行器 Implementation Plan, Global Constraints, Self-Review 记录（写完计划时执行）, Task 1: 项目骨架 + schedule 包（核心窗口逻辑）, Task 2: queue 包（任务模型 + JSON 持久化）, Task 3: runner 包（任务执行 + 日志 + 重试）, Task 4: daemon 包（调度循环 + 派发 + 崩溃恢复）, Task 5: CLI 子命令 (+3 more)

### Community 8 - "Changelog"
Cohesion: 0.12
Nodes (16): [0.1.0] - 2026-08-24, [0.1.1] - 2026-08-24, [0.2.0] - 2026-08-24, [0.2.1] - 2026-08-25, [0.2.2] - 2026-08-25, [0.2.3] - 2026-08-27, [0.2.4] - 2026-08-29, Added (+8 more)

### Community 9 - "DefaultSchedule"
Cohesion: 0.51
Nodes (9): DefaultSchedule(), T, Time, mustTime(), TestDescribe(), TestKindAt(), TestMultiplier(), TestNextOffPeakStart() (+1 more)

### Community 10 - "collectResults"
Cohesion: 0.14
Nodes (16): envList, resultRow, T, TestCollectResults(), TestCollectResultsEmpty(), TestFmtDuration(), TestFmtName(), TestParseHHMM() (+8 more)

### Community 11 - "runTask"
Cohesion: 0.18
Nodes (22): Cmd, runTaskForeground(), buildEnv(), Time, Writer, inPath(), killProcessGroup(), Run() (+14 more)

### Community 13 - "CONTEXT_FOR_NEXT_AGENT.md"
Cohesion: 0.22
Nodes (7): 已验证的关键行为, 最后一次完成的工作（2026-08-29）, 最后更新时间, 知识图谱, 远程资源, 遗留问题 / 待办, 项目当前状态

### Community 14 - "SchedulerState"
Cohesion: 0.18
Nodes (18): Heartbeat, SchedulerState, ceilSeconds(), Duration, Time, HeartbeatPath(), ProbeScheduler(), ReadHeartbeat() (+10 more)

## Knowledge Gaps
- **47 isolated node(s):** `belater`, `Fixed`, `Added`, `Fixed`, `Fixed` (+42 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Task` connect `Store` to `daemon_test.go`, `statusLines`, `newTestStore`, `collectResults`, `runTask`?**
  _High betweenness centrality (0.179) - this node is a cross-community bridge._
- **Why does `statusLines()` connect `statusLines` to `Main`, `Store`, `Schedule`, `SchedulerState`?**
  _High betweenness centrality (0.119) - this node is a cross-community bridge._
- **Why does `Store` connect `Store` to `Main`, `daemon_test.go`, `newTestStore`?**
  _High betweenness centrality (0.099) - this node is a cross-community bridge._
- **Are the 19 inferred relationships involving `DefaultSchedule()` (e.g. with `defaultSchedule()` and `TestStatusLinesHeartbeatWithoutVersion()`) actually correct?**
  _`DefaultSchedule()` has 19 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `statusLines()` (e.g. with `fmtAge()` and `fmtDuration()`) actually correct?**
  _`statusLines()` has 9 INFERRED edges - model-reasoned connections that need verification._
- **What connects `belater`, `Fixed`, `Added` to the rest of the system?**
  _47 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Main` be split into smaller, more focused modules?**
  _Cohesion score 0.10628019323671498 - nodes in this community are weakly interconnected._