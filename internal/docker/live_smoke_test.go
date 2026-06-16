package docker

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// Live smoke test against the real daemon. Runs only with EKIBEN_LIVE=1.
func TestLiveSmoke(t *testing.T) {
	if os.Getenv("EKIBEN_LIVE") != "1" {
		t.Skip("set EKIBEN_LIVE=1 to run against the real docker daemon")
	}
	cli, err := NewSDK()
	if err != nil {
		t.Fatalf("NewSDK: %v", err)
	}
	defer cli.Close()
	cs, err := cli.List(context.Background(), true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	fmt.Printf("LIVE: %d containers\n", len(cs))
	proj := map[string]int{}
	var firstRunning string
	for _, c := range cs {
		proj[c.Project]++
		if c.Running() && firstRunning == "" {
			firstRunning = c.ID
		}
		fmt.Printf("  %-26s proj=%-16s status=%-10s health=%-9s ports=%v\n",
			c.Name, c.Project, c.Status, c.Health, c.Ports)
	}
	fmt.Printf("LIVE: groups=%v\n", proj)
	if firstRunning != "" {
		s, err := cli.Stats(context.Background(), firstRunning)
		if err != nil {
			t.Fatalf("Stats: %v", err)
		}
		fmt.Printf("LIVE stats[%s]: cpu=%.2f%% mem=%d net=↓%d↑%d pids=%d\n",
			firstRunning[:12], s.CPUPerc, s.MemUsage, s.NetRx, s.NetTx, s.PIDs)
	}
}
