// Copyright (c) 2019, Oracle and/or its affiliates. All rights reserved.

package rate

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestFundedTakesLongInitialWait(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := New(100, WithClock(mc))

	// should add 1000 to the bucket
	limiter.Add(1000)

	// wait a minute
	mc.nextNow = mc.nextNow.Add(time.Minute)

	took, delay := limiter.Take()

	assert.Equal(t, uint64(100), took)
	assert.Equal(t, uint64(900), limiter.GetBucketSize())
	assert.Equal(t, time.Duration(0), delay)

	// wait one tenth of a second
	mc.nextNow = mc.nextNow.Add(100 * time.Millisecond)

	// should not be able to take as we've already parcelled out the 100 for this second
	took, delay = limiter.Take()

	assert.Equal(t, uint64(0), took)
	assert.Equal(t, uint64(900), limiter.GetBucketSize())
	assert.Equal(t, 900*time.Millisecond, delay)
}

func TestTenTakes(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := New(100, WithClock(mc))

	// should add 1000 to the bucket
	limiter.Add(15)
	took, delay := limiter.Take()

	// primes the pump by immediately taking the values back, does not increment clock
	assert.Equal(t, 15, int(took))
	assert.Equal(t, 0, int(limiter.GetBucketSize()))

	limiter.Add(100)

	// 7 runs gets rid of 77 bytes
	for lcv := 0; lcv < 7; lcv++ {
		mc.nextNow = mc.nextNow.Add(100 * time.Millisecond)

		took, delay = limiter.Take()

		assert.Equal(t, 11, int(took))
		assert.Equal(t, 100-(11*(lcv+1)), int(limiter.GetBucketSize()))
		assert.Equal(t, time.Duration(0), delay)
	}

	// wait one tenth of a second
	mc.nextNow = mc.nextNow.Add(100 * time.Millisecond)

	took, delay = limiter.Take()

	assert.Equal(t, 8, int(took))
	assert.Equal(t, 15, int(limiter.GetBucketSize()))
	assert.Equal(t, time.Duration(0), delay)

	// wait one tenth of a second
	mc.nextNow = mc.nextNow.Add(100 * time.Millisecond)

	took, delay = limiter.Take()

	assert.Equal(t, 0, int(took))
	assert.Equal(t, 15, int(limiter.GetBucketSize()))
	assert.Equal(t, 100*time.Millisecond, delay)

	// now waiting 2 * 1/10 of a second to clear enough space to get the rest
	mc.nextNow = mc.nextNow.Add(200 * time.Millisecond)

	// should now be able to take the last few bytes
	took, delay = limiter.Take()

	assert.Equal(t, 15, int(took))
	assert.Equal(t, 0, int(limiter.GetBucketSize()))
	assert.Equal(t, time.Duration(0), delay)

	// one more 1/10 and take again but there should be nothing left
	mc.nextNow = mc.nextNow.Add(100 * time.Millisecond)

	took, delay = limiter.Take()

	assert.Equal(t, 0, int(took))
	assert.Equal(t, 0, int(limiter.GetBucketSize()))
	assert.Equal(t, time.Duration(0), delay)
}

func TestTwoAndAHalfSeconds(t *testing.T) {
	limiter := New(100)

	now := time.Now()
	limiter.Add(250)

	count := 0
	totalTook := 0
	for {
		count++

		took, delay := limiter.Take()

		if took == 0 && delay == 0 {
			break
		}

		if took == 0 {
			time.Sleep(delay)
		} else {
			totalTook += int(took)
		}
	}

	elapsed := time.Now().Sub(now)

	assert.Equal(t, 250, totalTook)
	assert.True(t, elapsed >= (2*time.Second))
}

func TestRateOfOne(t *testing.T) {
	testLowRate(t, 1)
}

func TestRateOfZero(t *testing.T) {
	// zero should be the same as 1
	testLowRate(t, 0)
}

func testLowRate(t *testing.T, rate uint64) {
	mc := &mockClock{}
	now := time.Now()
	mc.nextNow = now

	limiter := New(rate, WithClock(mc))

	limiter.Add(10)

	count := 0
	for {
		count++

		took, delay := limiter.Take()
		if delay != 0 {
			// faux sleep
			mc.nextNow = mc.nextNow.Add(delay)
			continue
		}

		// delay is zero
		if took == 0 {
			// All ten are gone
			break
		}

		assert.Equal(t, 1, int(took))
	}

	assert.Equal(t, 20, count)

	// should have slept nine seconds
	expectedWaitTime := now.Add(9 * time.Second)

	assert.Equal(t, expectedWaitTime, mc.nextNow)
}

type mockClock struct {
	nextNow time.Time
}

func (mc *mockClock) Now() time.Time {
	return mc.nextNow
}
