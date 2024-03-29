// Copyright (c) 2019-2023, Oracle and/or its affiliates. All rights reserved.

// Package rate A leaky bucket rate limiter for streaming data
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

	// Take removes some number of elements from the limiter in order to
	// attempt to get as close to the limit as it can.  It returns the
	// number of elements that can be processed at this time.  If it returns
	// 0 elements it may return the duration the caller should wait before
	// trying to take more.  If both the number of elements and the duration
	// return 0 then the limiter is empty
	Take() (uint64, time.Duration)

	// GetLimit Returns the limit this limiter will attempt to achieve in
	// elements per second
	GetLimit() uint64

	// SetLimit Sets a new limit in elements per second and returns the old limit.
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
	takeByBlock  bool

	currentBucketSize uint64
	lastEvent         *time.Time
	replies           *repliesList
	blocks            []block
	nextBlockTime     *time.Time
}

// Option is an option for New that allows the limiter to
// be configured
type Option func(l *limiterData)

// TakeByBlock if this is set then the elements MUST be returned
// in the exact sizes as given to the Add function.  This is useful
// for cases where the incoming stream is a block-based protocol and
// the packets must not be split up.  In this mode the Take method will
// always return the exact sequence of bytes given by Add in the same order
// and may return large blocking times to ensure that the total number of elements
// does not exceed the limit over time.  However, in this mode it is
// possible for there to be spikes over the limit since the block itself
// could be larger than the limit (e.g., if my limit is 10 and the next block
// is 20 then when the system gives out 20 it will be over the limit for that
// time period but the next operation will return an appropriate wait duration
// to keep the total throughput at the limit)
func TakeByBlock() Option {
	return func(l *limiterData) {
		l.takeByBlock = true
	}
}

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
		blocks:       make([]block, 0),
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

	if ld.takeByBlock {
		ld.blocks = append(ld.blocks, block{chunkSize})
		return
	}

	currentSize := ld.currentBucketSize
	ld.currentBucketSize = currentSize + chunkSize
}

func (ld *limiterData) Take() (uint64, time.Duration) {
	if ld.takeByBlock {
		return ld.doTakeByBlock()
	}

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

func (ld *limiterData) doTakeByBlock() (uint64, time.Duration) {
	ld.lock.Lock()
	defer ld.lock.Unlock()

	now := ld.clock.Now()

	if ld.nextBlockTime != nil {
		nbt := *ld.nextBlockTime

		if now.Before(nbt) {
			// user did not in fact wait long enough, return the difference
			return 0, nbt.Sub(now)
		}

		// user waited enough time, just go on with the protocol
		ld.nextBlockTime = nil
	}

	previousSecondSize := ld.replies.calculateAndCut(now)
	if previousSecondSize <= ld.limit {
		if len(ld.blocks) == 0 {
			return 0, 0
		}

		block := ld.blocks[0]
		ld.blocks = ld.blocks[1:]

		blockSize := block.size

		// not at limit for this second yet
		ld.replies.add(blockSize, now)

		totalThisSecond := blockSize + previousSecondSize
		if totalThisSecond > ld.limit {
			// we JUST pushed it over, need to set the future wait time
			ld.setNextBlockTime(now, float64(totalThisSecond))
		}

		return blockSize, 0
	}

	overTime := ld.setNextBlockTime(now, float64(previousSecondSize))

	return 0, overTime
}

func (ld *limiterData) setNextBlockTime(now time.Time, knownSize float64) time.Duration {
	overTime := time.Duration(knownSize / (ld.limitAsFloat / float64(time.Second)))

	oldestRecordTime := ld.replies.lastTime()
	if oldestRecordTime == nil {
		// does not seem possible to get into here
		nextBlockTime := now.Add(overTime)
		ld.nextBlockTime = &nextBlockTime

		return overTime
	}

	durationServed := now.Sub(*oldestRecordTime)

	overTime = overTime - durationServed
	if overTime < 1 {
		overTime = 1
	}

	nextBlockTime := now.Add(overTime)
	ld.nextBlockTime = &nextBlockTime

	return overTime
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

// calculateAndCut returns total amount sent in last second
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

type block struct {
	size uint64
}

type defaultClock struct {
}

func (dc *defaultClock) Now() time.Time {
	return time.Now()
}
