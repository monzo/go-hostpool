package hostpool

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHostPool(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	dummyErr := errors.New("Dummy Error")

	p := New([]string{"a", "b", "c"})
	assert.Equal(t, p.Get().Host(), "a")
	assert.Equal(t, p.Get().Host(), "b")
	assert.Equal(t, p.Get().Host(), "c")
	respA := p.Get()
	assert.Equal(t, respA.Host(), "a")

	respA.Mark(dummyErr)
	respB := p.Get()
	respB.Mark(dummyErr)
	respC := p.Get()
	assert.Equal(t, respC.Host(), "c")
	respC.Mark(nil)
	// get again, and verify that it's still c
	assert.Equal(t, p.Get().Host(), "c")
	// now try to mark b as success; should fail because already marked
	respB.Mark(nil)
	assert.Equal(t, p.Get().Host(), "c") // would be b if it were not dead
	// now restore a
	respA = &standardHostPoolResponse{host: "a", pool: p}
	respA.Mark(nil)
	assert.Equal(t, p.Get().Host(), "a")
	assert.Equal(t, p.Get().Host(), "c")

	// ensure that we get *something* back when all hosts fail
	for _, host := range []string{"a", "b", "c"} {
		response := &standardHostPoolResponse{host: host, pool: p}
		response.Mark(dummyErr)
	}
	resp := p.Get()
	assert.NotEqual(t, resp, nil)
}

type mockTimer struct {
	t int // the time it will always return
}

func (t *mockTimer) between(start time.Time, end time.Time) time.Duration {
	return time.Duration(t.t) * time.Millisecond
}

func TestEpsilonGreedy(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	rand.Seed(10)

	iterations := 12000
	p := NewEpsilonGreedy([]string{"a", "b"}, 0, &LinearEpsilonValueCalculator{}).(*epsilonGreedyHostPool)

	timings := make(map[string]int64)
	timings["a"] = 200
	timings["b"] = 300

	hitCounts := make(map[string]int)
	hitCounts["a"] = 0
	hitCounts["b"] = 0

	log.Printf("starting first run (a, b)")

	for i := 0; i < iterations; i += 1 {
		if i != 0 && i%100 == 0 {
			p.performEpsilonGreedyDecay()
		}
		hostR := p.Get()
		host := hostR.Host()
		hitCounts[host]++
		timing := timings[host]
		p.timer = &mockTimer{t: int(timing)}
		hostR.Mark(nil)
	}

	for host := range hitCounts {
		log.Printf("host %s hit %d times (%0.2f percent)", host, hitCounts[host], (float64(hitCounts[host])/float64(iterations))*100.0)
	}

	assert.Equal(t, hitCounts["a"] > hitCounts["b"], true)

	hitCounts["a"] = 0
	hitCounts["b"] = 0
	log.Printf("starting second run (b, a)")
	timings["a"] = 500
	timings["b"] = 100

	for i := 0; i < iterations; i += 1 {
		if i != 0 && i%100 == 0 {
			p.performEpsilonGreedyDecay()
		}
		hostR := p.Get()
		host := hostR.Host()
		hitCounts[host]++
		timing := timings[host]
		p.timer = &mockTimer{t: int(timing)}
		hostR.Mark(nil)
	}

	for host := range hitCounts {
		log.Printf("host %s hit %d times (%0.2f percent)", host, hitCounts[host], (float64(hitCounts[host])/float64(iterations))*100.0)
	}

	assert.Equal(t, hitCounts["b"] > hitCounts["a"], true)
}

func BenchmarkEpsilonGreedy(b *testing.B) {
	b.StopTimer()

	// Make up some response times
	zipfDist := rand.NewZipf(rand.New(rand.NewSource(0)), 1.1, 5, 5000)
	timings := make([]uint64, b.N)
	for i := 0; i < b.N; i++ {
		timings[i] = zipfDist.Uint64()
	}

	// Make the hostpool with a few hosts
	p := NewEpsilonGreedy([]string{"a", "b"}, 0, &LinearEpsilonValueCalculator{}).(*epsilonGreedyHostPool)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if i != 0 && i%100 == 0 {
			p.performEpsilonGreedyDecay()
		}
		hostR := p.Get()
		p.timer = &mockTimer{t: int(timings[i])}
		hostR.Mark(nil)
	}
}

func BenchmarkEpsilonGreedyManyHosts(topB *testing.B) {
	bench := func(hostCount int, enableDecay bool) func(*testing.B) {
		return func(b *testing.B) {
			b.StopTimer()

			// Make up some response times
			zipfDist := rand.NewZipf(rand.New(rand.NewSource(0)), 1.1, 5, 5000)
			timings := make([]uint64, b.N)
			for i := 0; i < b.N; i++ {
				timings[i] = zipfDist.Uint64()
			}

			// Make the hostpool with a few hosts
			hosts := make([]string, 0, hostCount)
			for i := 0; i < hostCount; i++ {
				hosts = append(hosts, fmt.Sprintf("%d", i))
			}
			p := NewEpsilonGreedy(hosts, 0, &LinearEpsilonValueCalculator{}).(*epsilonGreedyHostPool)

			b.StartTimer()
			for i := 0; i < b.N; i++ {
				if enableDecay && i != 0 && i%100 == 0 {
					p.performEpsilonGreedyDecay()
				}
				hostR := p.Get()
				p.timer = &mockTimer{t: int(timings[i])}
				hostR.Mark(nil)
			}
		}
	}

	topB.Run("Hosts10/NoDecay", bench(10, false))
	topB.Run("Hosts25/NoDecay", bench(25, false))
	topB.Run("Hosts50/NoDecay", bench(50, false))
	topB.Run("Hosts100/NoDecay", bench(100, false))
	topB.Run("Hosts250/NoDecay", bench(250, false))

	topB.Run("Hosts10/WithDecay", bench(10, true))
	topB.Run("Hosts25/WithDecay", bench(25, true))
	topB.Run("Hosts50/WithDecay", bench(50, true))
	topB.Run("Hosts100/WithDecay", bench(100, true))
	topB.Run("Hosts250/WithDecay", bench(250, true))
}

func TestHostPoolErrorBudget(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	dummyErr := errors.New("Dummy Error")

	p := NewWithOptions([]string{"a", "b"}, StandardHostPoolOptions{
		MaxFailures:   2,
		FailureWindow: 60 * time.Second,
	})

	// Initially both hosts are available.
	assert.Equal(t, p.Get().Host(), "a")
	assert.Equal(t, p.Get().Host(), "b")

	// Mark an error against a.
	respA := p.Get()
	assert.Equal(t, respA.Host(), "a")
	respA.Mark(dummyErr)

	assert.Equal(t, p.Get().Host(), "b")
	// a should still be available (second failure)
	respA = p.Get()
	assert.Equal(t, respA.Host(), "a")
	respA.Mark(dummyErr)

	assert.Equal(t, p.Get().Host(), "b")

	respA = p.Get()
	// a should be marked as down (third failure)
	assert.Equal(t, respA.Host(), "a")
	respA.Mark(dummyErr)

	// Host a should not be available.
	assert.Equal(t, p.Get().Host(), "b")
}

func TestHostPoolErrorPercentageBudget(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	dummyErr := errors.New("Dummy Error")

	hpOptions := StandardHostPoolOptions{
		MaxFailures:       100,
		MaxFailurePercent: 50,
		FailureWindow:     60 * time.Second,
	}
	p := NewWithOptions([]string{"a", "b"}, hpOptions)

	// Initially both hosts are available.
	assert.Equal(t, p.Get().Host(), "a")
	assert.Equal(t, p.Get().Host(), "b")

	// Mark 10 successes against a & b.
	for i := 0; i < 10; i++ {
		p.Get().Mark(nil)
		p.Get().Mark(nil)
	}

	// a should still be avaliable (no failures)
	assert.Equal(t, p.Get().Host(), "a")
	assert.Equal(t, p.Get().Host(), "b")

	// Mark 10 failures against a.
	for i := 0; i < 10; i++ {
		p.Get().Mark(dummyErr)
		p.Get().Mark(nil)
	}

	// Only b should be returned
	assert.Equal(t, p.Get().Host(), "b")
	assert.Equal(t, p.Get().Host(), "b")

	// create a new hostpool with empty ringbuffers
	p = NewWithOptions([]string{"a", "b"}, hpOptions)

	// Initially both hosts are available.
	assert.Equal(t, p.Get().Host(), "a")
	assert.Equal(t, p.Get().Host(), "b")

	// Single error against a
	p.Get().Mark(dummyErr)
	p.Get().Mark(nil)

	// Both hosts still avaliable
	assert.Equal(t, p.Get().Host(), "a")
	assert.Equal(t, p.Get().Host(), "b")
}

func TestHostPoolErrorBudgetReset(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stdout)

	dummyErr := errors.New("Dummy Error")

	p := NewWithOptions([]string{"a", "b"}, StandardHostPoolOptions{
		MaxFailures:   1,
		FailureWindow: 1 * time.Second,
	})

	// Initially both hosts are available
	assert.Equal(t, p.Get().Host(), "a")
	assert.Equal(t, p.Get().Host(), "b")

	// Mark an error against a
	respA := p.Get()
	assert.Equal(t, respA.Host(), "a")
	respA.Mark(dummyErr)

	// Fetch next host
	assert.Equal(t, p.Get().Host(), "b")

	// Ensure failure window is exceeded
	time.Sleep(time.Second * 2)

	// Mark another error
	respA = p.Get()
	assert.Equal(t, respA.Host(), "a")
	respA.Mark(dummyErr)

	assert.Equal(t, p.Get().Host(), "b")

	// a should still be available
	assert.Equal(t, p.Get().Host(), "a")
}
