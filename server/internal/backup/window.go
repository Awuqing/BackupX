package backup

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MaintenanceWindow 描述一个允许执行备份的时段。
// 格式语义：
//   - Days 为 "0..6" 的字符串集合（0=周日，6=周六）；空 = 每天
//   - StartMinutes / EndMinutes 为"午夜起计算的分钟数"，0 ≤ v < 1440
//   - 跨午夜窗口：Start > End 表示跨夜（如 22:00-06:00）
//
// 多个窗口是 OR 语义：只要 now 落入任一窗口即允许执行。
type MaintenanceWindow struct {
	Days         map[int]bool
	StartMinutes int
	EndMinutes   int
}

// ParseMaintenanceWindows 解析用户配置（CSV 每项形如 "days=mon,tue|time=22:00-06:00"）。
// 简化语法：多个窗口以 ';' 分隔，每个窗口按 "[days=xxx;]time=HH:MM-HH:MM" 格式。
// Days 缺省 = 全周；若不合法，跳过该段而非抛错（让调用方尽力工作）。
// 示例：
//   "time=01:00-05:00"                        每天 1 点到 5 点
//   "days=sat,sun;time=00:00-23:59"           仅周末全天
//   "time=22:00-06:00"                        每天跨夜
//   "days=mon,tue,wed,thu,fri;time=22:00-06:00" 工作日跨夜
func ParseMaintenanceWindows(value string) []MaintenanceWindow {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	segments := strings.Split(v, ";")
	var windows []MaintenanceWindow
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		window, ok := parseSingleWindow(segment)
		if !ok {
			continue
		}
		windows = append(windows, window)
	}
	return windows
}

func parseSingleWindow(segment string) (MaintenanceWindow, bool) {
	// "days=xxx,time=HH:MM-HH:MM" 或 "time=..."
	fields := strings.Split(segment, ",")
	days := map[int]bool{}
	var timeExpr string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if strings.HasPrefix(field, "days=") {
			daysPart := strings.TrimPrefix(field, "days=")
			for _, day := range strings.Split(daysPart, "|") {
				if idx := parseDayToken(strings.TrimSpace(day)); idx >= 0 {
					days[idx] = true
				}
			}
		} else if strings.HasPrefix(field, "time=") {
			timeExpr = strings.TrimPrefix(field, "time=")
		}
	}
	start, end, ok := parseTimeRange(strings.TrimSpace(timeExpr))
	if !ok {
		return MaintenanceWindow{}, false
	}
	return MaintenanceWindow{Days: days, StartMinutes: start, EndMinutes: end}, true
}

var dayTokens = map[string]int{
	"sun": 0, "sunday": 0, "0": 0,
	"mon": 1, "monday": 1, "1": 1,
	"tue": 2, "tuesday": 2, "2": 2,
	"wed": 3, "wednesday": 3, "3": 3,
	"thu": 4, "thursday": 4, "4": 4,
	"fri": 5, "friday": 5, "5": 5,
	"sat": 6, "saturday": 6, "6": 6,
}

func parseDayToken(value string) int {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return -1
	}
	if idx, ok := dayTokens[v]; ok {
		return idx
	}
	return -1
}

// parseTimeRange 解析 "HH:MM-HH:MM"，返回起止分钟数。
func parseTimeRange(value string) (int, int, bool) {
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, ok := parseHHMM(parts[0])
	if !ok {
		return 0, 0, false
	}
	end, ok := parseHHMM(parts[1])
	if !ok {
		return 0, 0, false
	}
	return start, end, true
}

func parseHHMM(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || h < 0 || h > 23 {
		return 0, false
	}
	m, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// IsWithinWindow 判断 t 是否落入任一窗口。windows 为空或 nil 时总是返回 true（不限制）。
func IsWithinWindow(t time.Time, windows []MaintenanceWindow) bool {
	if len(windows) == 0 {
		return true
	}
	minutes := t.Hour()*60 + t.Minute()
	weekday := int(t.Weekday())
	for _, w := range windows {
		if len(w.Days) > 0 && !w.Days[weekday] {
			continue
		}
		if w.StartMinutes == w.EndMinutes {
			continue
		}
		if w.StartMinutes < w.EndMinutes {
			// 同日窗口
			if minutes >= w.StartMinutes && minutes < w.EndMinutes {
				return true
			}
		} else {
			// 跨午夜：[start, 1440) ∪ [0, end)
			if minutes >= w.StartMinutes || minutes < w.EndMinutes {
				return true
			}
		}
	}
	return false
}

// ValidateMaintenanceWindows 用户输入合法性校验（返回人可读的错误）。
func ValidateMaintenanceWindows(value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	segments := strings.Split(v, ";")
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if _, ok := parseSingleWindow(segment); !ok {
			return fmt.Errorf("无效的维护窗口配置: %q（期望格式如 time=22:00-06:00 或 days=sat,sun,time=00:00-23:59）", segment)
		}
	}
	return nil
}
