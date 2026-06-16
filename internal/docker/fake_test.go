package docker

import (
	"context"
	"testing"
)

func TestFakeListAndAction(t *testing.T) {
	f := NewFake([]Container{{ID: "1", Name: "a", Status: StatusUp}})
	got, err := f.List(context.Background(), true)
	if err != nil || len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("list = %v err %v", got, err)
	}
	if err := f.Stop(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	got, _ = f.List(context.Background(), true)
	if got[0].Status != StatusExited {
		t.Fatalf("after stop status = %v", got[0].Status)
	}
}
