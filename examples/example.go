package hostpool

import (
	"github.com/hailocab/go-hostpool"
)

func ExampleNewEpsilonGreedy() {
	hp := hostpool.NewEpsilonGreedy([]string{"a", "b"}, 0, &hostpool.LinearEpsilonValueCalculator{})
	hostResponse := hp.Get()
	_ = hostResponse.Host()
	hostResponse.Mark(nil)
}
