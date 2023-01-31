package hostpool

import (
	"sync"
	"time"
)

const standardFailureBuckets = 120

// standardHostPoolResponse implements HostPoolResponse
type standardHostPoolResponse struct {
	host string
	sync.Once
	pool HostPool
}

func (r *standardHostPoolResponse) Host() string {
	return r.host
}

func (r *standardHostPoolResponse) hostPool() HostPool {
	return r.pool
}

func (r *standardHostPoolResponse) Mark(err error) {
	r.Do(func() {
		doMark(err, r)
	})
}

func doMark(err error, r HostPoolResponse) {
	if err == nil {
		r.hostPool().markSuccess(r)
	} else {
		r.hostPool().markFailed(r)
	}
}

// standardHostPool implements HostPool
type standardHostPool struct {
	sync.RWMutex
	hosts           map[string]*hostEntry
	hostList        []*hostEntry
	returnUnhealthy bool
	nextHostIndex   int
	logger          Logger
	// Host retry parameters
	initialRetryDelay time.Duration
	maxRetryInterval  time.Duration
	// Error budget config
	maxFailures       int
	maxFailurePercent float64
	failureWindow     time.Duration
	quit              chan bool
}

type StandardHostPoolOptions struct {
	Logger Logger
	// Host retry parameters
	InitialRetryDelay time.Duration
	MaxRetryInterval  time.Duration
	// Error budget config
	MaxFailures       int
	MaxFailurePercent float64
	FailureWindow     time.Duration
}

// ------ constants -------------------

const initialRetryDelay = time.Duration(30) * time.Second
const maxRetryInterval = time.Duration(900) * time.Second
const defaultFailureWindow = time.Duration(60) * time.Second
const defaultMaxFailurePercent = 101.0

// Construct a basic HostPool using the hostnames provided
func New(hosts []string) HostPool {
	p := &standardHostPool{
		returnUnhealthy:   true,
		hosts:             make(map[string]*hostEntry, len(hosts)),
		hostList:          make([]*hostEntry, len(hosts)),
		logger:            DefaultLogger{},
		initialRetryDelay: initialRetryDelay,
		maxRetryInterval:  maxRetryInterval,
		quit:              make(chan bool, 1),
	}

	for i, h := range hosts {
		e := &hostEntry{
			host:            h,
			retryDelay:      p.initialRetryDelay,
			bucketedSuccess: make([]int64, standardFailureBuckets),
			bucketedFailure: make([]int64, standardFailureBuckets),
		}
		p.hosts[h] = e
		p.hostList[i] = e
	}

	return p
}

func NewWithOptions(hosts []string, options StandardHostPoolOptions) HostPool {
	// Initialise with defaults, override from options
	p := &standardHostPool{
		returnUnhealthy:   true,
		hosts:             make(map[string]*hostEntry, len(hosts)),
		hostList:          make([]*hostEntry, len(hosts)),
		logger:            DefaultLogger{},
		initialRetryDelay: initialRetryDelay,
		maxRetryInterval:  maxRetryInterval,
		maxFailurePercent: defaultMaxFailurePercent,
		failureWindow:     defaultFailureWindow,
		quit:              make(chan bool, 1),
	}

	if options.Logger != nil {
		p.logger = options.Logger
	}
	if options.InitialRetryDelay > 0 {
		p.initialRetryDelay = options.InitialRetryDelay
	}
	if options.MaxRetryInterval > 0 {
		p.maxRetryInterval = options.MaxRetryInterval
	}
	if options.MaxFailures > 0 {
		p.maxFailures = options.MaxFailures
	}
	if options.MaxFailurePercent > 0 {
		p.maxFailurePercent = options.MaxFailurePercent
	}
	if options.FailureWindow > 0 {
		p.failureWindow = options.FailureWindow
		go p.rotateBuckets()
	}

	for i, h := range hosts {
		e := &hostEntry{
			host:            h,
			retryDelay:      p.initialRetryDelay,
			bucketedSuccess: make([]int64, standardFailureBuckets),
			bucketedFailure: make([]int64, standardFailureBuckets),
		}
		if p.maxFailures > 0 {
			// We test for failures > maxFailures, so need an extra slot in the buffer.
			e.failures = NewRingBuffer(p.maxFailures + 1)
		}
		p.hosts[h] = e
		p.hostList[i] = e
	}

	return p
}

// return an entry from the HostPool
func (p *standardHostPool) Get() HostPoolResponse {
	p.Lock()
	defer p.Unlock()
	host := p.getRoundRobin()
	if host == "" {
		return nil
	}

	return &standardHostPoolResponse{host: host, pool: p}
}

func (p *standardHostPool) getRoundRobin() string {
	now := time.Now()
	hostCount := len(p.hostList)
	for i := range p.hostList {
		// iterate via sequenece from where we last iterated
		currentIndex := (i + p.nextHostIndex) % hostCount

		h := p.hostList[currentIndex]
		if !h.dead {
			p.nextHostIndex = currentIndex + 1
			return h.host
		}
		if h.nextRetry.Before(now) {
			h.willRetryHost(p.maxRetryInterval)
			p.nextHostIndex = currentIndex + 1
			return h.host
		}
	}

	// all hosts are down and returnUnhealhy is false then return no host
	if !p.returnUnhealthy {
		return ""
	}

	// all hosts are down. re-add them
	p.doResetAll()
	p.nextHostIndex = 0
	return p.hostList[0].host
}

func (p *standardHostPool) rotateBuckets() {
	durationPerBucket := p.failureWindow / standardFailureBuckets
	ticker := time.NewTicker(durationPerBucket)
	for {
		select {
		case <-p.quit:
			ticker.Stop()
			return
		case <-ticker.C:
			p.performBucketRotation()
		}
	}
}

func (p *standardHostPool) performBucketRotation() {
	p.Lock()
	for _, h := range p.hostList {
		h.bucketedRotate()
	}
	p.Unlock()
}

func (p *standardHostPool) ResetAll() {
	p.Lock()
	defer p.Unlock()
	p.doResetAll()
}

func (p *standardHostPool) SetHosts(hosts []string) {
	p.Lock()
	defer p.Unlock()
	p.setHosts(hosts)
}

func (p *standardHostPool) ReturnUnhealthy(v bool) {
	p.Lock()
	defer p.Unlock()
	p.returnUnhealthy = v
}

func (p *standardHostPool) setHosts(hosts []string) {
	p.hosts = make(map[string]*hostEntry, len(hosts))
	p.hostList = make([]*hostEntry, len(hosts))

	for i, h := range hosts {
		e := &hostEntry{
			host:       h,
			retryDelay: p.initialRetryDelay,
		}
		if p.maxFailures > 0 {
			// We test for failures > maxFailures, so need an extra slot in the buffer.
			e.failures = NewRingBuffer(p.maxFailures + 1)
		}
		p.hosts[h] = e
		p.hostList[i] = e
	}
}

// this actually performs the logic to reset,
// and should only be called when the lock has
// already been acquired
func (p *standardHostPool) doResetAll() {
	for _, h := range p.hosts {
		h.dead = false
	}
}

func (p *standardHostPool) Close() {
	for _, h := range p.hosts {
		h.dead = true
	}
	close(p.quit)
}

func (p *standardHostPool) markSuccess(hostR HostPoolResponse) {
	host := hostR.Host()
	p.Lock()
	defer p.Unlock()

	h, ok := p.hosts[host]
	if !ok {
		p.logger.Fatalf("host %s not in HostPool %v", host, p.Hosts())
	}
	h.dead = false
	h.bucketedSuccess[h.bucketedIndex] += 1
}

func (p *standardHostPool) markFailed(hostR HostPoolResponse) {
	host := hostR.Host()
	p.Lock()
	defer p.Unlock()

	h, ok := p.hosts[host]
	if !ok {
		p.logger.Fatalf("host %s not in HostPool %v", host, p.Hosts())
	}

	h.bucketedFailure[h.bucketedIndex] += 1
	if !h.dead {
		if p.maxFailurePercent > 0 {
			successCount, _ := h.requestBucketStatus()
			errorPercentage := (h.calculateErrorRatio() * 100)
			if successCount > 0 && errorPercentage >= p.maxFailurePercent {
				p.logger.Printf("host %s exceeded %d%% failure percentage in %s (%0.2f%%)", h.host, p.maxFailures, p.failureWindow, errorPercentage)
				h.markDead(p.initialRetryDelay)
				return
			}
		}

		if h.failures != nil {
			ts := time.Now()
			h.failures.insert(ts)
			failureLookback := -p.failureWindow
			failuresInWindow := h.failures.since(ts.Add(failureLookback))
			if failuresInWindow > p.maxFailures {
				p.logger.Printf("host %s exceeded %d failures in %s", h.host, p.maxFailures, p.failureWindow)
				h.markDead(p.initialRetryDelay)
				return
			}
		} else {
			h.markDead(p.initialRetryDelay)
		}
	}

}

func (p *standardHostPool) Hosts() []string {
	hosts := make([]string, 0, len(p.hosts))
	for host := range p.hosts {
		hosts = append(hosts, host)
	}
	return hosts
}

func (p *standardHostPool) Statistics() HostPoolStatistics {
	p.RLock()
	defer p.RUnlock()

	var alive int64 = 0
	for _, host := range p.hosts {
		if !host.dead {
			alive += 1
		}
	}

	return HostPoolStatistics{
		Gauges: []Gauge{
			{
				Name:  HostPoolGaugeNumberOfHosts,
				Value: int64(len(p.hosts)),
			},
			{
				Name:  HostPoolGaugeNumberOfLiveHosts,
				Value: alive,
			},
		},
	}
}
