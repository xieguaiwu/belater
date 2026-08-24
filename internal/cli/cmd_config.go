package cli

import (
	"encoding/json"
	"flag"
	"fmt"

	"belater/internal/schedule"
)

// cmdConfig shows the effective scheduling configuration.
func cmdConfig(args []string) int {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "JSON 输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		return usageError(fs, "不接受位置参数")
	}

	sch := defaultSchedule()

	if *jsonOut {
		peaks := make([][2]int, 0, len(sch.WeekdayPeaks))
		for _, p := range sch.WeekdayPeaks {
			peaks = append(peaks, p)
		}
		out := map[string]any{
			"version":              Version,
			"state_dir":            stateDir(),
			"timezone":             sch.Location.String(),
			"weekday_peak_windows": peaks,
			"weekend_all_offpeak":  sch.WeekendAllOffPeak,
			"offpeak_multiplier":   sch.Multiplier(schedule.KindOffPeak),
			"peak_multiplier":      sch.Multiplier(schedule.KindPeak),
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	fmt.Println("belater 调度配置")
	fmt.Printf("  版本:        %s\n", Version)
	fmt.Printf("  状态目录:    %s\n", stateDir())
	fmt.Printf("  时区:        %s (DeepSeek 计费时区)\n", sch.Location)
	fmt.Println("  高峰窗口:    周一至周五 09:00-12:00、14:00-18:00")
	fmt.Println("  周末:        全天空闲 (2026-08-23 起规则)")
	fmt.Println("  价格:        空闲 = 高峰的 50%")
	return 0
}
