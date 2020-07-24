package hostpool

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetricsBasics(t *testing.T) {
	pool := New([]string{"a", "b", "c"})

	stats := pool.Statistics()

	gaugeWithName := func(name string) Gauge {
		for _, gauge := range stats.Gauges {
			if gauge.Name == name {
				return gauge
			}
		}

		require.FailNow(t, "Couldn't find named gauge", "Wanted %s but had %v", name, stats.Gauges)
		return Gauge{} //unreachable
	}

	require.Equal(t, int64(3), gaugeWithName(HostPoolGaugeNumberOfHosts).Value)
	require.Equal(t, int64(3), gaugeWithName(HostPoolGaugeNumberOfLiveHosts).Value)

	poolMember := pool.Get()
	poolMember.Mark(errors.New("any old error"))

	stats = pool.Statistics()
	require.Equal(t, int64(3), gaugeWithName(HostPoolGaugeNumberOfHosts).Value)
	require.Equal(t, int64(2), gaugeWithName(HostPoolGaugeNumberOfLiveHosts).Value)
}
