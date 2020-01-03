package hostpool

import (
	"time"
)

// --- hostEntry - this is due to get upgraded

type hostEntry struct {
	host            string
	nextRetry       time.Time
	retryCount      int16
	retryDelay      time.Duration
	dead            bool
	historicBuckets []*epsilonBucket
	activeBucket    *epsilonBucket

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
	h.retryCount++
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
	var lastBucket *epsilonBucket

	for i, bucket := range h.historicBuckets {
		if bucket.Count() > 0 {
			value += bucket.WeightedAverage(i)
			lastBucket = bucket
		} else if lastBucket != nil {
			value += lastBucket.WeightedAverage(i)
		}
	}
	return value
}
