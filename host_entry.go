package hostpool

import (
	"time"
)

// --- hostEntry - this is due to get upgraded

type hostEntry struct {
	host              string
	nextRetry         time.Time
	retryCount        int16
	retryDelay        time.Duration
	dead              bool
	epsilonCounts     []int64
	epsilonValues     []int64
	epsilonIndex      int
	epsilonValue      float64
	epsilonPercentage float64
}

func (h *hostEntry) canTryHost(now time.Time) bool {
	if !h.dead {
		return true
	}
	if h.nextRetry.Before(now) {
		return true
	}
	return false
}

func (h *hostEntry) willRetryHost(maxRetryInterval time.Duration) {
	h.retryCount += 1
	newDelay := h.retryDelay * 2
	if newDelay < maxRetryInterval {
		h.retryDelay = newDelay
	} else {
		h.retryDelay = maxRetryInterval
	}
	h.nextRetry = time.Now().Add(h.retryDelay)
}

func (h *hostEntry) getWeightedAverageResponseTime() float64 {
	var value float64
	var lastValue float64

	buckets := len(h.epsilonCounts)

	// start at 1 so we start with the oldest entry
	for i := 1; i <= buckets; i += 1 {
		pos := (h.epsilonIndex + i) % buckets
		bucketCount := h.epsilonCounts[pos]
		// Changing the line below to what I think it should be to get the weights right
		weight := float64(i) / float64(buckets)
		if bucketCount > 0 {
			currentValue := float64(h.epsilonValues[pos]) / float64(bucketCount)
			value += currentValue * weight
			lastValue = currentValue
		} else {
			value += lastValue * weight
		}
	}
	return value
}
