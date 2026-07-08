package main

import (
	"math"
	"testing"
	"time"
)

func TestTrayPaceThresholds(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	resetAfter := func(d time.Duration) *time.Time {
		// 経過 d ⇔ リセットまで残り (7日 - d)
		ts := now.Add(7*24*time.Hour - d)
		return &ts
	}
	tests := []struct {
		name       string
		resetsAt   *time.Time
		wantYellow float64
		wantRed    float64
	}{
		{"リセット時刻不明は固定閾値", nil, 60, 80},
		{"2日経過", resetAfter(48 * time.Hour), 60 * 2 / 7.0, 80 * 2 / 7.0},
		{"7日経過(終端)", resetAfter(7 * 24 * time.Hour), 60, 80},
		{"6時間経過は12時間の床を適用", resetAfter(6 * time.Hour), 60 * 0.5 / 7, 80 * 0.5 / 7},
		{"経過ゼロも床を適用", resetAfter(0), 60 * 0.5 / 7, 80 * 0.5 / 7},
		{"時計ずれで8日経過扱いでも上限でクランプ", resetAfter(8 * 24 * time.Hour), 60, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yellow, red := trayPaceThresholds(tt.resetsAt, now, 7)
			if math.Abs(yellow-tt.wantYellow) > 1e-9 || math.Abs(red-tt.wantRed) > 1e-9 {
				t.Errorf("trayPaceThresholds() = (%.4f, %.4f), want (%.4f, %.4f)",
					yellow, red, tt.wantYellow, tt.wantRed)
			}
		})
	}
}

func TestTrayPaceThresholdsSplitDays(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	resetAfter := func(d time.Duration) *time.Time {
		// 経過 d ⇔ リセットまで残り (7日 - d)。windowTotal は splitDays に関係なく常に 7 日固定。
		ts := now.Add(7*24*time.Hour - d)
		return &ts
	}
	tests := []struct {
		name       string
		resetsAt   *time.Time
		splitDays  int
		wantYellow float64
		wantRed    float64
	}{
		{"5日分割: 1日経過", resetAfter(24 * time.Hour), 5, 60 * 24 / 120.0, 80 * 24 / 120.0},
		{"5日分割: 5日超過は上限でクランプ", resetAfter(6 * 24 * time.Hour), 5, 60, 80},
		{"分割なし: resetsAtがあっても固定閾値", resetAfter(24 * time.Hour), 0, 60, 80},
		{"分割なし: resetsAtがnilでも固定閾値", nil, 0, 60, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yellow, red := trayPaceThresholds(tt.resetsAt, now, tt.splitDays)
			if math.Abs(yellow-tt.wantYellow) > 1e-9 || math.Abs(red-tt.wantRed) > 1e-9 {
				t.Errorf("trayPaceThresholds() = (%.4f, %.4f), want (%.4f, %.4f)",
					yellow, red, tt.wantYellow, tt.wantRed)
			}
		})
	}
}

func TestTrayBandFor(t *testing.T) {
	const yellow, red = 17.14, 22.86
	tests := []struct {
		name        string
		utilization float64
		want        trayBand
	}{
		{"閾値未満は緑", 10, trayBandGreen},
		{"黄閾値ちょうどは緑", yellow, trayBandGreen},
		{"黄閾値超は黄", yellow + 0.01, trayBandAmber},
		{"赤閾値ちょうどは黄", red, trayBandAmber},
		{"赤閾値超は赤", red + 0.01, trayBandRed},
		{"100%は赤", 100, trayBandRed},
		{"0%は緑", 0, trayBandGreen},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trayBandFor(tt.utilization, yellow, red); got != tt.want {
				t.Errorf("trayBandFor(%v) = %v, want %v", tt.utilization, got, tt.want)
			}
		})
	}
}
