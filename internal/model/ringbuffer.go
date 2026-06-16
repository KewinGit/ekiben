package model

// RingBuffer is a fixed-capacity FIFO of float64 samples, oldest evicted first.
type RingBuffer struct {
	data []float64
	cap  int
}

func NewRingBuffer(capacity int) *RingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuffer{data: make([]float64, 0, capacity), cap: capacity}
}

func (r *RingBuffer) Push(v float64) {
	if len(r.data) == r.cap {
		copy(r.data, r.data[1:])
		r.data[len(r.data)-1] = v
		return
	}
	r.data = append(r.data, v)
}

// Values returns a copy of the samples, oldest first.
func (r *RingBuffer) Values() []float64 {
	out := make([]float64, len(r.data))
	copy(out, r.data)
	return out
}
