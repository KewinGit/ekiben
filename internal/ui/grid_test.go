package ui

import (
	"testing"
)

func TestCardAt_Inside(t *testing.T) {
	rects := []cardRect{
		{id: "a", x: 0, y: 0, w: 20, h: 5},
		{id: "b", x: 21, y: 0, w: 20, h: 5},
	}
	id, ok := cardAt(rects, 5, 2)
	if !ok || id != "a" {
		t.Fatalf("expected (a, true), got (%q, %v)", id, ok)
	}
	id, ok = cardAt(rects, 25, 3)
	if !ok || id != "b" {
		t.Fatalf("expected (b, true), got (%q, %v)", id, ok)
	}
}

func TestCardAt_InGap(t *testing.T) {
	rects := []cardRect{
		{id: "a", x: 0, y: 0, w: 20, h: 5},
		{id: "b", x: 21, y: 0, w: 20, h: 5},
	}
	// x=20 is the gap between the two cards
	_, ok := cardAt(rects, 20, 2)
	if ok {
		t.Fatal("expected miss in gap, got hit")
	}
}

func TestCardAt_ExclusiveRightEdge(t *testing.T) {
	rects := []cardRect{
		{id: "a", x: 0, y: 0, w: 20, h: 5},
	}
	// x=19 is last column of card (0..19), x=20 is outside
	id, ok := cardAt(rects, 19, 0)
	if !ok || id != "a" {
		t.Fatalf("expected (a, true) at right edge, got (%q, %v)", id, ok)
	}
	_, ok = cardAt(rects, 20, 0)
	if ok {
		t.Fatal("expected miss one past right edge")
	}
}

func TestCardAt_ExclusiveBottomEdge(t *testing.T) {
	rects := []cardRect{
		{id: "a", x: 0, y: 0, w: 20, h: 5},
	}
	// y=4 is last row, y=5 is outside
	id, ok := cardAt(rects, 0, 4)
	if !ok || id != "a" {
		t.Fatalf("expected (a, true) at bottom edge, got (%q, %v)", id, ok)
	}
	_, ok = cardAt(rects, 0, 5)
	if ok {
		t.Fatal("expected miss one past bottom edge")
	}
}

func TestCardAt_SecondRow(t *testing.T) {
	rects := []cardRect{
		{id: "a", x: 0, y: 0, w: 20, h: 5},
		{id: "b", x: 0, y: 5, w: 20, h: 5},
	}
	id, ok := cardAt(rects, 10, 5)
	if !ok || id != "b" {
		t.Fatalf("expected (b, true) in second row, got (%q, %v)", id, ok)
	}
}

func TestCardAt_Empty(t *testing.T) {
	_, ok := cardAt(nil, 0, 0)
	if ok {
		t.Fatal("expected miss on empty rects")
	}
}

func TestCardAt_NegativeCoords(t *testing.T) {
	rects := []cardRect{
		{id: "a", x: 0, y: 0, w: 20, h: 5},
	}
	_, ok := cardAt(rects, -1, 0)
	if ok {
		t.Fatal("expected miss for negative x")
	}
	_, ok = cardAt(rects, 0, -1)
	if ok {
		t.Fatal("expected miss for negative y")
	}
}
