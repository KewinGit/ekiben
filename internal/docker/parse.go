package docker

import (
	"regexp"
	"strconv"
	"strings"
)

var exitRe = regexp.MustCompile(`\((\d+)\)`)

// ParseState maps docker's State + Status strings to our enums + exit code.
func ParseState(state, status string) (Status, Health, int) {
	exit := 0
	if m := exitRe.FindStringSubmatch(status); m != nil {
		exit, _ = strconv.Atoi(m[1])
	}
	health := HealthNone
	switch {
	case strings.Contains(status, "(healthy)"):
		health = HealthHealthy
	case strings.Contains(status, "(unhealthy)"):
		health = HealthUnhealthy
	case strings.Contains(status, "health: starting"):
		health = HealthStarting
	}
	switch state {
	case "running":
		if strings.Contains(status, "(Paused)") {
			return StatusPaused, health, exit
		}
		return StatusUp, health, exit
	case "paused":
		return StatusPaused, health, exit
	case "restarting":
		return StatusRestarting, HealthNone, exit
	case "exited":
		return StatusExited, HealthNone, exit
	case "dead":
		return StatusDead, HealthNone, exit
	case "created":
		return StatusCreated, HealthNone, exit
	}
	return StatusExited, HealthNone, exit
}

var hostPortRe = regexp.MustCompile(`:(\d+)->`)

// ParsePorts extracts host-published ports (deduped, in order) as ":<port>".
func ParsePorts(s string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range hostPortRe.FindAllStringSubmatch(s, -1) {
		p := ":" + m[1]
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
