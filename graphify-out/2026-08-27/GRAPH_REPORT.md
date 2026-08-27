# Graph Report - belater  (2026-08-25)

## Corpus Check
- 24 files · ~15,474 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 249 nodes · 538 edges · 13 communities (12 shown, 1 thin omitted)
- Extraction: 82% EXTRACTED · 18% INFERRED · 0% AMBIGUOUS · INFERRED: 97 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `a335bb4d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Main
- daemon_test.go
- Store
- Schedule
- belater — DeepSeek 峰谷计费延迟执行器
- cmdResults
- newTestStore
- Global Constraints
- Changelog
- DefaultSchedule
- runTask
- belater
- CONTEXT_FOR_NEXT_AGENT.md

## God Nodes (most connected - your core abstractions)
1. `Store` - 29 edges
2. `Task` - 24 edges
3. `Schedule` - 18 edges
4. `DefaultSchedule()` - 15 edges
5. `Main()` - 14 edges
6. `newTestStore()` - 14 edges
7. `newTestStore()` - 14 edges
8. `openStore()` - 13 edges
9. `finishTask()` - 13 edges
10. `runTask()` - 13 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `Main()`  [INFERRED]
  main.go → internal/cli/cli.go
- `Main()` --calls--> `cmdResults()`  [INFERRED]
  internal/cli/cli.go → internal/cli/cmd_results.go
- `openStore()` --calls--> `Open()`  [INFERRED]
  internal/cli/cli.go → internal/queue/store.go
- `cmdResults()` --calls--> `openStore()`  [INFERRED]
  internal/cli/cmd_results.go → internal/cli/cli.go
- `defaultSchedule()` --calls--> `DefaultSchedule()`  [INFERRED]
  internal/cli/cli.go → internal/schedule/window.go

## Import Cycles
- None detected.

## Communities (13 total, 1 thin omitted)

### Community 0 - "Main"
Cohesion: 0.12
Nodes (35): File, FlagSet, defaultSchedule(), fmtDuration(), fmtRunAt(), Duration, Time, Main() (+27 more)

### Community 1 - "daemon_test.go"
Cohesion: 0.13
Nodes (33): Context, Config, acquireInstanceLock(), dispatchOnce(), finishTask(), gc(), Duration, Time (+25 more)

### Community 2 - "Store"
Cohesion: 0.20
Nodes (7): Open(), Time, NewID(), Mutex, Status, Store, Task

### Community 3 - "Schedule"
Cohesion: 0.28
Nodes (4): Location, Time, Kind, Schedule

### Community 4 - "belater — DeepSeek 峰谷计费延迟执行器"
Cohesion: 0.07
Nodes (25): add 选项, belater — DeepSeek 峰谷计费延迟执行器, add options, belater — Defer big tasks to DeepSeek off-peak billing hours, Commands, Design, Install, License (+17 more)

### Community 5 - "cmdResults"
Cohesion: 0.13
Nodes (18): envList, resultRow, fmtName(), T, TestCollectResults(), TestCollectResultsEmpty(), TestFmtDuration(), TestFmtName() (+10 more)

### Community 6 - "newTestStore"
Cohesion: 0.45
Nodes (13): T, newTestStore(), sampleTask(), TestAddLoadRoundtrip(), TestClaimForRun(), TestCompareAndStatus(), TestConcurrentAdds(), TestEnvRoundtrip() (+5 more)

### Community 7 - "Global Constraints"
Cohesion: 0.18
Nodes (10): belater — DeepSeek 峰谷计费延迟执行器 Implementation Plan, Global Constraints, Self-Review 记录（写完计划时执行）, Task 1: 项目骨架 + schedule 包（核心窗口逻辑）, Task 2: queue 包（任务模型 + JSON 持久化）, Task 3: runner 包（任务执行 + 日志 + 重试）, Task 4: daemon 包（调度循环 + 派发 + 崩溃恢复）, Task 5: CLI 子命令 (+2 more)

### Community 8 - "Changelog"
Cohesion: 0.17
Nodes (11): [0.1.0] - 2026-08-24, [0.1.1] - 2026-08-24, [0.2.0] - 2026-08-24, [0.2.1] - 2026-08-25, [0.2.2] - 2026-08-25, Added, Added, Changelog (+3 more)

### Community 9 - "DefaultSchedule"
Cohesion: 0.51
Nodes (9): DefaultSchedule(), T, Time, mustTime(), TestDescribe(), TestKindAt(), TestMultiplier(), TestNextOffPeakStart() (+1 more)

### Community 11 - "runTask"
Cohesion: 0.18
Nodes (22): Cmd, runTaskForeground(), buildEnv(), Time, Writer, inPath(), killProcessGroup(), Run() (+14 more)

### Community 13 - "CONTEXT_FOR_NEXT_AGENT.md"
Cohesion: 0.22
Nodes (7): 已验证的关键行为, 最后一次完成的工作（2026-08-25）, 最后更新时间, 知识图谱, 远程资源, 遗留问题 / 待办, 项目当前状态

## Knowledge Gaps
- **42 isolated node(s):** `belater`, `Fixed`, `Fixed`, `Added`, `Added` (+37 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Task` connect `Store` to `daemon_test.go`, `runTask`, `cmdResults`, `newTestStore`?**
  _High betweenness centrality (0.169) - this node is a cross-community bridge._
- **Why does `Store` connect `Store` to `Main`, `daemon_test.go`, `newTestStore`?**
  _High betweenness centrality (0.146) - this node is a cross-community bridge._
- **Why does `Schedule` connect `Schedule` to `Main`, `daemon_test.go`, `DefaultSchedule`?**
  _High betweenness centrality (0.093) - this node is a cross-community bridge._
- **Are the 13 inferred relationships involving `DefaultSchedule()` (e.g. with `defaultSchedule()` and `Run()`) actually correct?**
  _`DefaultSchedule()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `Main()` (e.g. with `cmdAdd()` and `cmdConfig()`) actually correct?**
  _`Main()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **What connects `belater`, `Fixed`, `Fixed` to the rest of the system?**
  _42 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Main` be split into smaller, more focused modules?**
  _Cohesion score 0.12195121951219512 - nodes in this community are weakly interconnected._