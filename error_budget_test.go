package hostpool

import (
	"github.com/bmizerany/assert"

	"testing"
	"time"
)

func TestErrorBudgetFailuresSince(t *testing.T) {
	rb := NewRingBuffer(10)
	assert.Equal(t, 0, rb.since(time.Now().Add(-time.Minute)))

	rb.insert(time.Now())
	rb.insert(time.Now())

	assert.Equal(t, 2, rb.since(time.Now().Add(-time.Minute)))
	// Avoid boundary flakiness by adding an hour to current time.
	assert.Equal(t, 0, rb.since(time.Now().Add(time.Minute)))
}

func TestErrorBudgetInsert(t *testing.T) {
	rb := NewRingBuffer(5)

	lastInsert := time.Time{}
	for i := 0; i < 99; i++ {
		lastInsert = time.Now()
		rb.insert(lastInsert)
	}

	assert.Equal(t, lastInsert, rb.items[rb.index-1])
}
