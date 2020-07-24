package hostpool

// A gauge is a metric which represents a single value, whose value
// may increase or decrease. A hostpool specific example could be
// the number of hosts in the pool.
type Gauge struct {
	Name  string
	Value int64
}

const (
	HostPoolGaugeNumberOfHosts     = "hostpool_number_of_hosts"
	HostPoolGaugeNumberOfLiveHosts = "hostpool_number_of_live_hosts"
)
