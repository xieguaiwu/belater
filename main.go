// belater — DeepSeek 峰谷计费延迟执行器。
//
// 把大型任务（如 pi-agent 多 agent 工作）排到空闲时段自动执行，
// 避开高峰计费（空闲价格 = 高峰的 50%）。
package main

import (
	"os"

	"belater/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
