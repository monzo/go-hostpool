package hostpool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetWeightedAverageResponseTime(t *testing.T) {
	t.Parallel()

	tcs := [...]struct {
		name 	 string
		entry 	 *hostEntry
		expected float64
	}{
		{
			name: "zero case",
			entry: &hostEntry{
				epsilonCounts: []int64{0},
				epsilonValues: []int64{0},
				epsilonIndex:  0,
			},

			expected: float64(0),
		},
		{
			name: "simple example",
			entry: &hostEntry{
				// this field tracks the number of observations in each bucket
				epsilonCounts: []int64{1, 1, 1, 0, 0},
				// this field tracks the sum of response times in each bucket
				epsilonValues: []int64{1000, 1000, 1000, 0, 0},
				// this field tracks how far through the buckets we are
				epsilonIndex: 2,
			},

			// 5 buckets, so weights step by 0.2 each time
			// 1000 * 0.6 + 1000 * 0.8 + 1000 * 1
			// 600 + 800 + 1000
			expected: float64(2400),
		},
		{
			name: "simple example showing iteration order",
			entry: &hostEntry{
				epsilonCounts: []int64{1, 1, 1, 0, 0},
				epsilonValues: []int64{2000, 1000, 500, 0, 0},
				epsilonIndex:  2,
			},

			// 5 buckets, so weights step by 0.2 each time
			// 2000 * 0.6 + 1000 * 0.8 + 500 * 1
			// 1200 + 800 + 500
			expected: float64(2500),
		},
		{
			name: "another example showing the count + sum are averaged first",
			entry: &hostEntry{
				epsilonCounts: []int64{2, 4, 5, 5, 5},
				epsilonValues: []int64{2000, 1000, 500, 250, 125},
				epsilonIndex:  4, // newest value in index 4
			},

			// (2000 / 2) * 0.2 + (1000 / 4) * 0.4 + (500 / 5) * 0.6 + (250 / 5) * 0.8 + (125 / 5) * 1
			// 1000 * 0.2 + 250 * 0.4 + 100 * 0.6 + 50 * 0.8 + 25 * 1
			// 200 + 100 + 60 + 40 + 25
			expected: float64(425),
		},
		{
			name: "if one bucket has no entries, we take the previous value",
			entry: &hostEntry{
				epsilonCounts: []int64{2, 4, 5, 0, 5},
				epsilonValues: []int64{2000, 1000, 500, 0, 125},
				epsilonIndex:  4, // newest value in index 4
			},

			// (2000 / 2) * 0.2 + (1000 / 4) * 0.4 + (500 / 5) * 0.6 + <empty bucket>  + (125 / 5) * 1
			//                                uses previous value 👉 + (500 / 5) * 0.8 +
			// 1000 * 0.2 + 250 * 0.4 + 100 * 0.6 + (100 * 0.8 + 25 * 1
			//                                             ☝️ weight still steps down
			// 200 + 100 + 60 + 80 + 25
			expected: float64(465),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tc.entry.calculateWeightedAverages()
			require.Equal(t, tc.expected, tc.entry.getWeightedAverageResponseTime())
		})
	}
}
