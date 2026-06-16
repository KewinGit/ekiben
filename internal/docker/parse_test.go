package docker

import (
	"reflect"
	"testing"
)

func TestParseStatus(t *testing.T) {
	cases := []struct {
		state  string
		status string
		want   Status
		health Health
		exit   int
	}{
		{"running", "Up 50 minutes (healthy)", StatusUp, HealthHealthy, 0},
		{"running", "Up 50 minutes", StatusUp, HealthNone, 0},
		{"running", "Up 2 seconds (health: starting)", StatusUp, HealthStarting, 0},
		{"running", "Up 5 minutes (unhealthy)", StatusUp, HealthUnhealthy, 0},
		{"exited", "Exited (137) 3 minutes ago", StatusExited, HealthNone, 137},
		{"paused", "Up 10 minutes (Paused)", StatusPaused, HealthNone, 0},
		{"restarting", "Restarting (1) 5 seconds ago", StatusRestarting, HealthNone, 1},
	}
	for _, c := range cases {
		st, h, ex := ParseState(c.state, c.status)
		if st != c.want || h != c.health || ex != c.exit {
			t.Errorf("%q/%q -> (%v,%v,%d) want (%v,%v,%d)",
				c.state, c.status, st, h, ex, c.want, c.health, c.exit)
		}
	}
}

func TestParsePorts(t *testing.T) {
	in := "8081-8082/tcp, 0.0.0.0:8201->8080/tcp, 127.0.0.1:5432->5432/tcp"
	got := ParsePorts(in)
	want := []string{":8201", ":5432"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
