package hostpool

import (
	"time"
)

// --- hostEntry - this is due to get upgraded

type hostEntry struct {
	host       string
	nextRetry  time.Time
	retryCount int16
	retryDelay time.Duration
	dead       bool
	failures   *ringBuffer

	epsilonCounts          []int64 // ring of counts observed
	epsilonValues          []int64 // ring of total time observations observed
	epsilonIndex           int     // current index in the ring
	epsilonWeightedTotal   float64 // The total not including the active bucket
	epsilonWeightedLastVal float64 // The last non-zero count average

	bucketedSuccess []int64 // successes observed by bucket
	bucketedFailure []int64 // errors observed by time bucket
	bucketedIndex   int     // current index in the ring
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
	currentBucketCount := h.epsilonCounts[h.epsilonIndex]

	// If we've not seen any observations yet, use the last value from the
	// previous buckets
	if currentBucketCount == 0 {
		return h.epsilonWeightedTotal + h.epsilonWeightedLastVal
	}

	// Take our weighted total and add on the average of our current index
	// which has a 100% weighting
	currentAvg := float64(h.epsilonValues[h.epsilonIndex]) / float64(currentBucketCount)
	return h.epsilonWeightedTotal + currentAvg
}

func (h *hostEntry) epsilonDecay() {
	// Move to the next position in the ring
	h.epsilonIndex = (h.epsilonIndex + 1) % len(h.epsilonCounts)
	h.epsilonCounts[h.epsilonIndex] = 0
	h.epsilonValues[h.epsilonIndex] = 0
	h.calculateWeightedAverages()
}

func (h *hostEntry) bucketedRotate() {
	// Move to the next position in the ring
	h.bucketedIndex = (h.bucketedIndex + 1) % len(h.bucketedSuccess)
	h.bucketedSuccess[h.bucketedIndex] = 0
	h.bucketedFailure[h.bucketedIndex] = 0
}

func (h *hostEntry) calculateWeightedAverages() {
	// We start with the oldest entry in the ring and move forward, coming up to
	// the most recent entry (but not the current one which is when i = 0
	// resulting in pos pointing to the current bucket index)
	buckets := len(h.epsilonCounts)
	var total, lastValue float64
	for i := 1; i < buckets; i++ {
		pos := (h.epsilonIndex + i) % buckets
		bucketCount := h.epsilonCounts[pos]
		weight := float64(i) / float64(buckets)
		if h.epsilonCounts[pos] > 0 {
			// We have observed values in this bucket, so let's tally them up
			avg := float64(h.epsilonValues[pos]) / float64(bucketCount)
			total += avg * weight
			lastValue = avg
		} else {
			// We had no values observed in this bucket, so we just use the
			// previous bucket and carry over the weight
			total += lastValue * weight
		}
	}

	h.epsilonWeightedTotal = total
	h.epsilonWeightedLastVal = lastValue
}

func (h *hostEntry) requestBucketStatus() (int64, int64) {
	var successCount int64
	var failureCount int64
	for i := range h.bucketedSuccess {
		successCount += h.bucketedSuccess[i]
		failureCount += h.bucketedFailure[i]
	}
	return successCount, failureCount
}

func (h *hostEntry) calculateErrorRatio() float64 {
	successCount, failureCount := h.requestBucketStatus()
	return float64(failureCount) / float64(failureCount+successCount)
}

func (h *hostEntry) markDead(initialRetryDelay time.Duration) {
	h.dead = true
	h.retryCount = 0
	h.retryDelay = initialRetryDelay
	h.nextRetry = time.Now().Add(h.retryDelay)
}
