package model

import (
	"sort"

	"github.com/KewinGit/ekiben/internal/docker"
)

const StandaloneGroup = "standalone"

type Group struct {
	Name       string
	Containers []docker.Container
}

// GroupContainers buckets containers by compose project. Order: groups present
// in `order` first (in that order), then remaining named groups alphabetically,
// then the standalone group last.
func GroupContainers(cs []docker.Container, order []string) []Group {
	buckets := map[string][]docker.Container{}
	for _, c := range cs {
		key := c.Project
		if key == "" {
			key = StandaloneGroup
		}
		buckets[key] = append(buckets[key], c)
	}

	seen := map[string]bool{}
	out := []Group{}
	add := func(name string) {
		if cl, ok := buckets[name]; ok && !seen[name] {
			seen[name] = true
			out = append(out, Group{Name: name, Containers: cl})
		}
	}

	for _, name := range order {
		add(name)
	}

	rest := []string{}
	for name := range buckets {
		if !seen[name] && name != StandaloneGroup {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		add(name)
	}
	add(StandaloneGroup)
	return out
}

func (g Group) GroupName() string                 { return g.Name }
func (g Group) GetContainers() []docker.Container { return g.Containers }
