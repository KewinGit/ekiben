package docker

import (
	"context"
	"testing"
	"time"
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

func TestFakeImagesRoundTrip(t *testing.T) {
	f := NewFake(nil)
	f.ImagesList = []Image{
		{ID: "sha256:abc123", Repo: "nginx", Tag: "latest", Size: 1024 * 1024 * 50, Created: time.Now()},
		{ID: "sha256:def456", Repo: "alpine", Tag: "3.18", Size: 1024 * 1024 * 5, Created: time.Now()},
	}
	imgs, err := f.Images(context.Background())
	if err != nil {
		t.Fatalf("Images() error: %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("want 2 images, got %d", len(imgs))
	}
	if imgs[0].Repo != "nginx" || imgs[0].Tag != "latest" {
		t.Fatalf("unexpected first image: %+v", imgs[0])
	}
	if imgs[1].Repo != "alpine" || imgs[1].Tag != "3.18" {
		t.Fatalf("unexpected second image: %+v", imgs[1])
	}
}

func TestFakeVolumesRoundTrip(t *testing.T) {
	f := NewFake(nil)
	f.VolumesList = []Volume{
		{Name: "mydata", Driver: "local", Mountpoint: "/var/lib/docker/volumes/mydata/_data"},
		{Name: "pgvol", Driver: "local", Mountpoint: "/var/lib/docker/volumes/pgvol/_data"},
	}
	vols, err := f.Volumes(context.Background())
	if err != nil {
		t.Fatalf("Volumes() error: %v", err)
	}
	if len(vols) != 2 {
		t.Fatalf("want 2 volumes, got %d", len(vols))
	}
	if vols[0].Name != "mydata" || vols[0].Driver != "local" {
		t.Fatalf("unexpected first volume: %+v", vols[0])
	}
	if vols[1].Name != "pgvol" {
		t.Fatalf("unexpected second volume: %+v", vols[1])
	}
}
