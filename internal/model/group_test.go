package model

import (
	"testing"

	"github.com/KewinGit/ekiben/internal/docker"
)

func names(g []Group) []string {
	out := []string{}
	for _, x := range g {
		out = append(out, x.Name)
	}
	return out
}

func TestGroupOrdering(t *testing.T) {
	cs := []docker.Container{
		{Name: "a1", Project: "arya"},
		{Name: "h1", Project: "hydra"},
		{Name: "z1", Project: "zeta"},
		{Name: "s1", Project: ""}, // standalone
	}
	g := GroupContainers(cs, []string{"hydra", "arya"})
	// configured order first, then unknown alpha, then standalone last
	if got := names(g); len(got) != 4 ||
		got[0] != "hydra" || got[1] != "arya" || got[2] != "zeta" || got[3] != StandaloneGroup {
		t.Fatalf("order = %v", got)
	}
}

func TestStandaloneCollectsUnlabeled(t *testing.T) {
	cs := []docker.Container{{Name: "x", Project: ""}, {Name: "y", Project: ""}}
	g := GroupContainers(cs, nil)
	if len(g) != 1 || g[0].Name != StandaloneGroup || len(g[0].Containers) != 2 {
		t.Fatalf("standalone grouping wrong: %+v", g)
	}
}
