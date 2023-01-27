package hostpool

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testFakeLogger struct {
	callCount int
}

func (l *testFakeLogger) Printf(msg string, args ...interface{}) {
	l.callCount += 1
}

func (l *testFakeLogger) Println(msg string) {
	l.callCount += 1
}

func (l *testFakeLogger) Fatalf(msg string, args ...interface{}) {
	l.callCount += 1
}

func TestLogger(t *testing.T) {
	dummyErr := errors.New("Dummy Error")
	lgr := testFakeLogger{}

	p := NewWithOptions([]string{"a"}, StandardHostPoolOptions{
		MaxFailures:   1,
		FailureWindow: 60 * time.Second,
		Logger:        &lgr,
	})

	// Mark two errors against a.
	p.Get().Mark(dummyErr)
	p.Get().Mark(dummyErr)

	// we should have logged an error
	assert.Equal(t, lgr.callCount, 1)
}
