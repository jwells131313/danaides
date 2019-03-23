// Copyright (c) 2019, Oracle and/or its affiliates. All rights reserved.

package rate

import (
	"math"
	"sync"
	"time"
)

const (
	boostFactor float64 = 1.1
)

// Limiter is used to limit a stream of items to a specific rate
type Limiter interface {
	// Add Adds a chunk of data to the stream managed by this limiter
	Add(chunkSize uint64)

	// Take removes bytes from the limiter in order to attempt to get
	// as close to the limit as it can.  It returns the number of
	// elements that can be processed at this time.  Will return 0 if
	// the bucket is empty.  This method will block until it is safe to
	// process at least one element unless there is an event on the doneChan
	Take(doneChan chan interface{}) uint64

	// Returns the limit this limiter will attempt to achieve in
	// items per second
	GetLimit() float64
}

// Clock is an interface to be used for testing
type Clock interface {
	Now() time.Time
}

type limiterData struct {
	lock         sync.Mutex
	clock        Clock
	limit        float64
	boostedLimit float64

	currentBucketSize uint64
	lastEvent         time.Time
}

// New creates a new Limiter with the given limit
func New(limit float64) Limiter {
	return NewWithClock(limit, &defaultClock{})
}

// NewWithClock is used for testing with a specialized clock
func NewWithClock(limit float64, clock Clock) Limiter {
	return &limiterData{
		clock:        clock,
		limit:        limit,
		boostedLimit: limit * boostFactor,
	}
}

func (ld *limiterData) Add(chunkSize uint64) {
	ld.lock.Lock()
	defer ld.lock.Unlock()

	currentSize := ld.currentBucketSize
	ld.currentBucketSize = currentSize + chunkSize

	if currentSize == 0 {
		ld.lastEvent = ld.clock.Now()
	}
}

func (ld *limiterData) Take(doneChan chan interface{}) uint64 {
	ld.lock.Lock()
	defer ld.lock.Unlock()

	now := ld.clock.Now()
	defer func() {
		ld.lastEvent = now
	}()

	if ld.currentBucketSize == 0 {
		return 0
	}

	sinceLastEvent := now.Sub(ld.lastEvent)

	var maxOutput float64
	if sinceLastEvent < time.Second {
		maxOutput = ld.boostedLimit * (float64(sinceLastEvent) / float64(time.Second))
	} else {
		maxOutput = ld.limit
	}

	output := uint64(math.Round(maxOutput))
	if output > ld.currentBucketSize {
		output = ld.currentBucketSize
	}
	ld.currentBucketSize = ld.currentBucketSize - output

	return output
}

func (ld *limiterData) GetLimit() float64 {
	return ld.limit
}

type defaultClock struct {
}

func (dc *defaultClock) Now() time.Time {
	return time.Now()
}
