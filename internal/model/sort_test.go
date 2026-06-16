package model

import (
	"testing"

	"github.com/KewinGit/ekiben/internal/docker"
)

func TestSortByName(t *testing.T) {
	cs := []docker.Container{{Name: "b"}, {Name: "a"}, {Name: "c"}}
	SortContainers(cs, nil, "name")
	if cs[0].Name != "a" || cs[1].Name != "b" || cs[2].Name != "c" {
		t.Fatalf("name sort wrong: %v", cs)
	}
}

func TestSortByCPUDesc(t *testing.T) {
	cs := []docker.Container{{ID: "1", Name: "a"}, {ID: "2", Name: "b"}}
	st := map[string]docker.Stats{"1": {CPUPerc: 1}, "2": {CPUPerc: 9}}
	SortContainers(cs, st, "cpu")
	if cs[0].ID != "2" {
		t.Fatalf("cpu sort should put highest first: %v", cs)
	}
}
