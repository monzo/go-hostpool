package hostpool

import (
	"math/rand"
	"time"
)

type powerOfTwoHostPool struct {
	epsilonGreedyHostPool
}

type PowerOfTwoOptions struct {
	Hosts          []string
	DecayDuration  time.Duration
	Calc           EpsilonValueCalculator
	InitialEpsilon float64
	EpsilonBuckets int
	EpsilonDecay   float32
	MinEpsilon     float32
}

func NewPowerOfTwo(options PowerOfTwoOptions) HostPool {
	epsilonGreedy := NewEpsilonGreedyWithOptions(EpsilonGreedyOptions{
		Hosts:                         options.Hosts,
		DecayDuration:                 options.DecayDuration,
		Calc:                          options.Calc,
		InitialEpsilon:                options.InitialEpsilon,
		EpsilonBuckets:                options.EpsilonBuckets,
		EpsilonDecay:                  options.EpsilonDecay,
		MinEpsilon:                    options.MinEpsilon,
		IncludeHostsWithoutDatapoints: true,
	}).(*epsilonGreedyHostPool)

	return &powerOfTwoHostPool{
		epsilonGreedyHostPool: *epsilonGreedy,
	}
}

func (p *powerOfTwoHostPool) Get() HostPoolResponse {
	p.Lock()
	defer p.Unlock()

	host := p.getPowerOfTwoHost()
	if host == "" {
		return nil
	}

	started := time.Now()
	return &epsilonHostPoolResponse{
		standardHostPoolResponse: standardHostPoolResponse{host: host, pool: p},
		started:                  started,
	}
}

func (p *powerOfTwoHostPool) getPowerOfTwoHost() string {
	possibleHosts := p.scoreHosts()

	n := len(possibleHosts)
	switch n {
	case 0:
		// all hosts are marked dead - revert to round robin choice
		// note when `returnUnhealthy` is true in the standard host
		// pool (it is by default), this also resets all host marks
		return p.getRoundRobin()
	case 1:
		return possibleHosts[0].host
	default:
		firstPosition := rand.Intn(n)
		firstHost := possibleHosts[firstPosition]

		// move our selected host to the end of the slice
		possibleHosts[firstPosition], possibleHosts[n-1] = possibleHosts[n-1], possibleHosts[firstPosition]
		// then truncate the slice header, i.e. remove the selected host
		n -= 1
		possibleHosts = possibleHosts[:n]

		secondPosition := rand.Intn(n)
		secondHost := possibleHosts[secondPosition]

		var hostToUse *hostEntry
		// now compare the two random choices and return the best one
		if firstHost.epsilonValue > secondHost.epsilonValue {
			hostToUse = firstHost
		} else {
			hostToUse = secondHost
		}

		if hostToUse.dead {
			hostToUse.willRetryHost(p.maxRetryInterval)
		}

		return hostToUse.host
	}
}
