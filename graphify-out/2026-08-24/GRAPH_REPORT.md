# Graph Report - belater  (2026-08-24)

## Corpus Check
- 21 files · ~12,073 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 203 nodes · 440 edges · 14 communities (13 shown, 1 thin omitted)
- Extraction: 83% EXTRACTED · 17% INFERRED · 0% AMBIGUOUS · INFERRED: 74 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `8bae4d3f`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Main
- daemon_test.go
- Store
- Schedule
- belater — DeepSeek 峰谷计费延迟执行器
- parseHHMM
- newTestStore
- Global Constraints
- RunWithWriter
- DefaultSchedule
- cmd_daemon.go
- runTask
- belater
- CONTEXT_FOR_NEXT_AGENT.md

## God Nodes (most connected - your core abstractions)
1. `Store` - 27 edges
2. `Task` - 19 edges
3. `Schedule` - 18 edges
4. `DefaultSchedule()` - 14 edges
5. `Main()` - 13 edges
6. `newTestStore()` - 13 edges
7. `runTask()` - 13 edges
8. `openStore()` - 12 edges
9. `newTestStore()` - 12 edges
10. `usageError()` - 11 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `Main()`  [INFERRED]
  main.go → internal/cli/cli.go
- `Main()` --calls--> `cmdDaemon()`  [INFERRED]
  internal/cli/cli.go → internal/cli/cmd_daemon.go
- `openStore()` --calls--> `Open()`  [INFERRED]
  internal/cli/cli.go → internal/queue/store.go
- `cmdDaemon()` --calls--> `openStore()`  [INFERRED]
  internal/cli/cmd_daemon.go → internal/cli/cli.go
- `defaultSchedule()` --calls--> `DefaultSchedule()`  [INFERRED]
  internal/cli/cli.go → internal/schedule/window.go

## Import Cycles
- None detected.

## Communities (14 total, 1 thin omitted)

### Community 0 - "Main"
Cohesion: 0.15
Nodes (26): File, FlagSet, defaultSchedule(), fmtDuration(), fmtName(), fmtRunAt(), Duration, Time (+18 more)

### Community 1 - "daemon_test.go"
Cohesion: 0.14
Nodes (30): Context, Config, acquireInstanceLock(), dispatchOnce(), finishTask(), gc(), Duration, Time (+22 more)

### Community 2 - "Store"
Cohesion: 0.20
Nodes (7): Open(), Time, NewID(), Mutex, Status, Store, Task

### Community 3 - "Schedule"
Cohesion: 0.28
Nodes (4): Location, Time, Kind, Schedule

### Community 4 - "belater — DeepSeek 峰谷计费延迟执行器"
Cohesion: 0.13
Nodes (14): add 选项, belater — DeepSeek 峰谷计费延迟执行器, 命令, 安装, 开发, 快速开始, 方式 1：systemd 用户服务（推荐）, 方式 2：cron 轮询 (+6 more)

### Community 5 - "parseHHMM"
Cohesion: 0.19
Nodes (10): envList, T, TestFmtDuration(), TestFmtName(), TestParseHHMM(), TestShQuote(), Location, Time (+2 more)

### Community 6 - "newTestStore"
Cohesion: 0.50
Nodes (11): T, newTestStore(), sampleTask(), TestAddLoadRoundtrip(), TestConcurrentAdds(), TestEnvRoundtrip(), TestInvalidStatusRejected(), TestLoadEmpty() (+3 more)

### Community 7 - "Global Constraints"
Cohesion: 0.18
Nodes (10): belater — DeepSeek 峰谷计费延迟执行器 Implementation Plan, Global Constraints, Self-Review 记录（写完计划时执行）, Task 1: 项目骨架 + schedule 包（核心窗口逻辑）, Task 2: queue 包（任务模型 + JSON 持久化）, Task 3: runner 包（任务执行 + 日志 + 重试）, Task 4: daemon 包（调度循环 + 派发 + 崩溃恢复）, Task 5: CLI 子命令 (+2 more)

### Community 8 - "RunWithWriter"
Cohesion: 0.36
Nodes (8): Cmd, runTaskForeground(), Time, Writer, killProcessGroup(), Run(), RunWithWriter(), Result

### Community 9 - "DefaultSchedule"
Cohesion: 0.51
Nodes (9): DefaultSchedule(), T, Time, mustTime(), TestDescribe(), TestKindAt(), TestMultiplier(), TestNextOffPeakStart() (+1 more)

### Community 10 - "cmd_daemon.go"
Cohesion: 0.47
Nodes (8): cmdDaemon(), Duration, installSystemd(), mustHome(), runForever(), runOnce(), systemctl(), uninstallSystemd()

### Community 11 - "runTask"
Cohesion: 0.47
Nodes (10): T, runTask(), TestRunChildProcessKilled(), TestRunCwd(), TestRunEnv(), TestRunEscapedChildBoundedDrain(), TestRunExitCode(), TestRunNoDuplicateLastLine() (+2 more)

### Community 13 - "CONTEXT_FOR_NEXT_AGENT.md"
Cohesion: 0.25
Nodes (6): 最后一次完成的工作（2026-08-24）, 最后更新时间, 知识图谱, 远程资源, 遗留问题 / 待办, 项目当前状态

## Knowledge Gaps
- **26 isolated node(s):** `belater`, `项目当前状态`, `最后一次完成的工作（2026-08-24）`, `遗留问题 / 待办`, `远程资源` (+21 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Store` connect `Store` to `Main`, `daemon_test.go`, `cmd_daemon.go`, `newTestStore`?**
  _High betweenness centrality (0.208) - this node is a cross-community bridge._
- **Why does `Task` connect `Store` to `RunWithWriter`, `daemon_test.go`, `runTask`, `newTestStore`?**
  _High betweenness centrality (0.154) - this node is a cross-community bridge._
- **Why does `Schedule` connect `Schedule` to `Main`, `daemon_test.go`, `DefaultSchedule`?**
  _High betweenness centrality (0.129) - this node is a cross-community bridge._
- **Are the 12 inferred relationships involving `DefaultSchedule()` (e.g. with `defaultSchedule()` and `Run()`) actually correct?**
  _`DefaultSchedule()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **Are the 11 inferred relationships involving `Main()` (e.g. with `cmdAdd()` and `cmdConfig()`) actually correct?**
  _`Main()` has 11 INFERRED edges - model-reasoned connections that need verification._
- **What connects `belater`, `项目当前状态`, `最后一次完成的工作（2026-08-24）` to the rest of the system?**
  _26 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Main` be split into smaller, more focused modules?**
  _Cohesion score 0.14623655913978495 - nodes in this community are weakly interconnected._