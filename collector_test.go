package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestCollectorSmoke(t *testing.T) {
	initPaths()
	events := scanProjectLogs(time.Now().AddDate(0, 0, -7))
	t.Logf("loaded %d events from %s", len(events), projectsDir)

	refreshUsage()
	snap := getUsageSnapshot()
	b, _ := json.MarshalIndent(snap, "", "  ")
	fmt.Println("=== UsageSnapshot ===")
	fmt.Println(string(b))

	if snap.FiveHour.LimitTokens == 0 && snap.SevenDay.LimitTokens == 0 {
		t.Log("warning: no limits computed (auto-estimation may have returned 0)")
	}
}
