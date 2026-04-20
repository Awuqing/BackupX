package backup

import (
	"testing"
	"time"
)

func TestParseAndCheck_SingleSameDayWindow(t *testing.T) {
	windows := ParseMaintenanceWindows("time=01:00-05:00")
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	// 周一 03:00 UTC（天数不限制）
	at := time.Date(2026, 4, 20, 3, 0, 0, 0, time.UTC)
	if !IsWithinWindow(at, windows) {
		t.Fatalf("expected 03:00 to be inside 01:00-05:00")
	}
	at = time.Date(2026, 4, 20, 6, 0, 0, 0, time.UTC)
	if IsWithinWindow(at, windows) {
		t.Fatalf("expected 06:00 to be outside 01:00-05:00")
	}
}

func TestParseAndCheck_CrossMidnight(t *testing.T) {
	windows := ParseMaintenanceWindows("time=22:00-06:00")
	if len(windows) != 1 {
		t.Fatalf("expected 1 window")
	}
	tests := []struct {
		hour, minute int
		inside       bool
	}{
		{22, 30, true},
		{23, 59, true},
		{0, 0, true},
		{3, 0, true},
		{5, 59, true},
		{6, 0, false},
		{7, 0, false},
		{21, 59, false},
	}
	base := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	for _, tc := range tests {
		at := base.Add(time.Duration(tc.hour)*time.Hour + time.Duration(tc.minute)*time.Minute)
		if got := IsWithinWindow(at, windows); got != tc.inside {
			t.Errorf("%02d:%02d expected inside=%v, got %v", tc.hour, tc.minute, tc.inside, got)
		}
	}
}

func TestParseAndCheck_DaysFilter(t *testing.T) {
	// 周末全天
	windows := ParseMaintenanceWindows("days=sat|sun,time=00:00-23:59")
	if len(windows) != 1 {
		t.Fatalf("expected 1 window")
	}
	sat := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC) // Saturday
	sun := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC) // Sunday
	mon := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC) // Monday
	if !IsWithinWindow(sat, windows) {
		t.Fatalf("saturday should be inside")
	}
	if !IsWithinWindow(sun, windows) {
		t.Fatalf("sunday should be inside")
	}
	if IsWithinWindow(mon, windows) {
		t.Fatalf("monday should be outside")
	}
}

func TestParseAndCheck_Multiple(t *testing.T) {
	// 两段：工作日跨夜 + 周末全天
	windows := ParseMaintenanceWindows("days=mon|tue|wed|thu|fri,time=22:00-06:00;days=sat|sun,time=00:00-23:59")
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}
	monAfternoon := time.Date(2026, 4, 20, 15, 0, 0, 0, time.UTC)
	if IsWithinWindow(monAfternoon, windows) {
		t.Fatalf("mon 15:00 should be outside both windows")
	}
	monNight := time.Date(2026, 4, 20, 23, 0, 0, 0, time.UTC)
	if !IsWithinWindow(monNight, windows) {
		t.Fatalf("mon 23:00 should be inside weekday-night window")
	}
	sunNoon := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	if !IsWithinWindow(sunNoon, windows) {
		t.Fatalf("sun 12:00 should be inside weekend window")
	}
}

func TestValidateMaintenanceWindows(t *testing.T) {
	if err := ValidateMaintenanceWindows(""); err != nil {
		t.Fatalf("empty should be valid, got %v", err)
	}
	if err := ValidateMaintenanceWindows("time=01:00-05:00"); err != nil {
		t.Fatalf("valid format rejected: %v", err)
	}
	if err := ValidateMaintenanceWindows("bad-input"); err == nil {
		t.Fatalf("invalid format should return error")
	}
	if err := ValidateMaintenanceWindows("time=25:00-30:00"); err == nil {
		t.Fatalf("invalid hour should return error")
	}
}

func TestIsWithinWindow_NoWindows(t *testing.T) {
	if !IsWithinWindow(time.Now(), nil) {
		t.Fatalf("no windows should always be inside")
	}
}
