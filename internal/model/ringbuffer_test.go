package model

import (
	"reflect"
	"testing"
)

func TestRingBufferKeepsLastN(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(1)
	rb.Push(2)
	rb.Push(3)
	rb.Push(4) // evicts 1
	got := rb.Values()
	want := []float64{2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRingBufferPartial(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Push(7)
	rb.Push(8)
	got := rb.Values()
	want := []float64{7, 8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
