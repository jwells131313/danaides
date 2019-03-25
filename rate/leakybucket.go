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
	// Add adds the given number of elements to the stream managed by this limiter
	Add(chunkSize uint64)

	// Take removes bytes from the limiter in order to attempt to get
	// as close to the limit as it can.  It returns the number of
	// elements that can be processed at this time.  If it returns 0
	// it will return the time the caller should wait before trying to
	// take more again.  If both the number of elements and the duration
	// return 0 then the limiter is empty
	Take() (uint64, time.Duration)

	// Returns the limit this limiter will attempt to achieve in
	// elements per second
	GetLimit() uint64

	// Sets a new limit in elements per second and returns the old limit.
	// Note that a limit of 0 is converted to a limit of 1
	SetLimit(uint64) uint64

	// GetBucketSize returns the current size of the bucket
	GetBucketSize() uint64
}

// Clock is an interface to be used for testing
type Clock interface {
	Now() time.Time
}

type limiterData struct {
	lock         sync.Mutex
	clock        Clock
	limit        uint64
	limitAsFloat float64
	boostedLimit float64

	currentBucketSize uint64
	lastEvent         *time.Time
	replies           *repliesList
}

// Option is an option for New that allows the limiter to
// be configured
type Option func(l *limiterData)

// New creates a new Limiter with the limit in elements
// per second.  If limit is 0 it will be set to 1
func New(limit uint64, opts ...Option) Limiter {
	if limit == 0 {
		limit = 1
	}

	limitAsFloat := float64(limit)

	retVal := &limiterData{
		clock:        &defaultClock{},
		limit:        limit,
		limitAsFloat: limitAsFloat,
		boostedLimit: limitAsFloat * boostFactor,
		replies:      &repliesList{},
	}

	for _, opt := range opts {
		opt(retVal)
	}

	return retVal
}

// WithClock is an option to new that allows a clock to
// be used with the limiter, used for testing
func WithClock(clock Clock) Option {
	return func(l *limiterData) {
		l.clock = clock
	}
}

func (ld *limiterData) Add(chunkSize uint64) {
	ld.lock.Lock()
	defer ld.lock.Unlock()

	currentSize := ld.currentBucketSize
	ld.currentBucketSize = currentSize + chunkSize
}

func (ld *limiterData) Take() (uint64, time.Duration) {
	ld.lock.Lock()
	defer ld.lock.Unlock()

	if ld.currentBucketSize == 0 {
		// no need to put on replies list and does not count as an event
		return 0, 0
	}

	now := ld.clock.Now()
	defer func() {
		ld.lastEvent = &now
	}()

	var sinceLastEvent time.Duration
	if ld.lastEvent == nil {
		sinceLastEvent = time.Second
	} else {
		sinceLastEvent = now.Sub(*ld.lastEvent)
	}

	var maxOutput float64
	if sinceLastEvent < time.Second {
		maxOutput = ld.boostedLimit * (float64(sinceLastEvent) / float64(time.Second))
	} else {
		maxOutput = ld.limitAsFloat
	}

	if maxOutput > ld.limitAsFloat {
		maxOutput = ld.limitAsFloat
	}

	output := uint64(math.Round(maxOutput))
	if output > ld.currentBucketSize {
		output = ld.currentBucketSize
	} else if output == 0 {
		// always try to send at least one element
		output = 1
	}

	historicalOutput := ld.replies.calculateAndCut(now)
	if historicalOutput > ld.limit {
		historicalOutput = ld.limit
	}

	totalOverOneSecond := historicalOutput + output
	if totalOverOneSecond > ld.limit {
		output = ld.limit - historicalOutput
	}

	// finished calculating the size, safe to remove from
	ld.currentBucketSize = ld.currentBucketSize - output
	delay := time.Duration(0)
	if output > 0 {
		ld.replies.add(output, now)
	} else {
		lastTime := ld.replies.lastTime()
		if lastTime == nil {
			delay = time.Second
		} else {
			delay = time.Second - now.Sub(*lastTime)
		}

	}

	return output, delay
}

func (ld *limiterData) GetLimit() uint64 {
	ld.lock.Lock()
	defer ld.lock.Unlock()

	return ld.limit
}

func (ld *limiterData) SetLimit(nLimit uint64) uint64 {
	ld.lock.Lock()
	defer ld.lock.Unlock()

	if nLimit == 0 {
		nLimit = 1
	}

	retVal := ld.limit
	ld.limit = nLimit
	ld.limitAsFloat = float64(nLimit)
	ld.boostedLimit = float64(nLimit) * boostFactor

	return retVal
}

func (ld *limiterData) GetBucketSize() uint64 {
	ld.lock.Lock()
	defer ld.lock.Unlock()

	return ld.currentBucketSize
}

type repliesList struct {
	head *repliesEntry
	tail *repliesEntry
}

func (rl *repliesList) add(output uint64, now time.Time) {
	entry := &repliesEntry{
		reply:   output,
		replied: now,
	}

	if rl.head == nil {
		rl.head = entry
		rl.tail = entry
		return
	}

	entry.next = rl.head
	rl.head = entry
}

func (rl *repliesList) lastTime() *time.Time {
	if rl.tail == nil {
		return nil
	}

	return &rl.tail.replied
}

func (rl *repliesList) calculateAndCut(now time.Time) uint64 {
	if rl.head == nil {
		return 0
	}

	// First, delete a second from now
	dropTime := now.Add(-time.Second)

	var retVal uint64
	var previous *repliesEntry
	for entry := rl.head; entry != nil; entry = entry.next {
		if entry.replied.After(dropTime) {
			retVal = retVal + entry.reply
		} else {
			// doing the cut, and returning
			if previous == nil {
				rl.head = nil
				rl.tail = nil
				return retVal
			}

			previous.next = nil
			rl.tail = previous
			return retVal
		}

		previous = entry
	}

	// nothing was cut, just return the value
	return retVal
}

type repliesEntry struct {
	reply   uint64
	replied time.Time

	next *repliesEntry
}

type defaultClock struct {
}

func (dc *defaultClock) Now() time.Time {
	return time.Now()
}
