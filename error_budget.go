package hostpool

import (
	"time"
)


type ringBuffer struct {
	size  int
	index int
	items []time.Time
}

func NewRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		size: size,
		items: make([]time.Time, size),
	}
}

func (r *ringBuffer) insert(ts time.Time) {
	r.index = (r.index + 1) % r.size
	r.items[r.index] = ts
}

// Since we have time.Time values, we can make use of the zero value to filter the whole buffer.
// Callers must lock (using the mutex provided by hostpool).
func (r *ringBuffer) since(t time.Time) int {
	if r == nil {
		return 0
	}

	failures := 0
	for i := 0; i < r.size; i++ {
		if r.items[i].After(t) {
			failures += 1
		}
	}
	return failures
}