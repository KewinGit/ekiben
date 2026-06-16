package model

import (
	"sort"

	"github.com/KewinGit/ekiben/internal/docker"
)

// SortContainers sorts in place by the given key:
// "name" (asc), "cpu" (desc), "mem" (desc), "status" (problems first, then name).
func SortContainers(cs []docker.Container, stats map[string]docker.Stats, by string) {
	switch by {
	case "cpu":
		sort.SliceStable(cs, func(i, j int) bool {
			return stats[cs[i].ID].CPUPerc > stats[cs[j].ID].CPUPerc
		})
	case "mem":
		sort.SliceStable(cs, func(i, j int) bool {
			return stats[cs[i].ID].MemUsage > stats[cs[j].ID].MemUsage
		})
	case "status":
		sort.SliceStable(cs, func(i, j int) bool {
			if cs[i].Problem() != cs[j].Problem() {
				return cs[i].Problem()
			}
			return cs[i].Name < cs[j].Name
		})
	default: // "name"
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name })
	}
}
