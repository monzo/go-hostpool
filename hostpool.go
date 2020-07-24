// A Go package to intelligently and flexibly pool among multiple hosts from your Go application.
// Host selection can operate in round robin or epsilon greedy mode, and unresponsive hosts are
// avoided. A good overview of Epsilon Greedy is here http://stevehanov.ca/blog/index.php?id=132
package hostpool

// Returns current version
func Version() string {
	return "0.1"
}

// --- Response interfaces and structs ----

// This interface represents the response from HostPool. You can retrieve the
// hostname by calling Host(), and after making a request to the host you should
// call Mark with any error encountered, which will inform the HostPool issuing
// the HostPoolResponse of what happened to the request and allow it to update.
type HostPoolResponse interface {
	Host() string
	Mark(error)

	hostPool() HostPool
}

// --- HostPool structs and interfaces ----

// This is the main HostPool interface. Structs implementing this interface
// allow you to Get a HostPoolResponse (which includes a hostname to use),
// get the list of all Hosts, and use ResetAll to reset state.
type HostPool interface {
	// Get selects a host from the pool. The returned response should then be marked as
	// either successful or failed when the host is used (by calling Mark() on the response
	// with the error value either nil for success, or set for failure).
	Get() HostPoolResponse

	// ResetAll marks all hosts in the pool as alive
	ResetAll()

	// ReturnUnhealthy when called with true will prevent an unhealthy node from
	// being returned and will instead return a nil HostPoolResponse. If using
	// this feature then you should check the result of Get for nil
	ReturnUnhealthy(v bool)

	// Hosts returns the current set of hosts known to the host pool
	Hosts() []string

	// SetHosts sets the current set of hosts known to the host pool. The set hosts default
	// to being alive
	SetHosts([]string)

	// Statistics returns a sample of properties of the hostpool that would be useful
	// to monitor, for example, the number of hosts, and the number of alive hosts
	Statistics() HostPoolStatistics

	// Close the hostpool and release all resources.
	Close()

	// keep the marks separate so we can override independently
	markSuccess(HostPoolResponse)
	markFailed(HostPoolResponse)
}

// HostPoolStatistics represents a single sample of possible statistics associated
// with a hostpool
type HostPoolStatistics struct {
	Gauges []Gauge
}
