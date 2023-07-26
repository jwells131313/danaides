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
	assert.Equal(t, 0, int(delay))
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
			// due to fake clock, we known exactly how much time we took
			assert.Equal(t, 1*time.Second, delay)

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

func TestChangeLimit(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := New(100, WithClock(mc))

	assert.Equal(t, int(100), int(limiter.GetLimit()))

	limiter.Add(1000)

	mc.nextNow = mc.nextNow.Add(time.Second)

	took, delay := limiter.Take()
	assert.Equal(t, 100, int(took))
	assert.Equal(t, 0, int(delay))

	old := limiter.SetLimit(50)
	assert.Equal(t, 100, int(old))
	assert.Equal(t, 50, int(limiter.GetLimit()))

	mc.nextNow = mc.nextNow.Add(time.Second)

	// should only be able to get 50 now
	took, delay = limiter.Take()
	assert.Equal(t, 50, int(took))
	assert.Equal(t, 0, int(delay))

	// now move limit up and be sure we can get it in next took call
	old = limiter.SetLimit(200)
	assert.Equal(t, 50, int(old))
	assert.Equal(t, 200, int(limiter.GetLimit()))

	// The 50 from before will still be in the window
	mc.nextNow = mc.nextNow.Add(time.Second - 1)

	took, delay = limiter.Take()
	assert.Equal(t, 150, int(took))
	assert.Equal(t, 0, int(delay))
}

func TestBlockLimitBasic(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := New(100, WithClock(mc), TakeByBlock())

	limiter.Add(200)
	limiter.Add(10)

	numTaken, waitDuration := limiter.Take()
	assert.Equal(t, uint64(200), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(time.Second / 2) // add a half second and call again, should not be enough time

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, 1*time.Second+time.Second/2, waitDuration)

	mc.nextNow = mc.nextNow.Add(waitDuration) // this should get us over the expected wait time

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(10), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(time.Second / 4) // another quarter second for veracity

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)
}

func TestBlockLimitMultipleTakesInOneSecond(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := New(100, WithClock(mc), TakeByBlock())

	limiter.Add(10)
	limiter.Add(10)
	limiter.Add(10)
	limiter.Add(71) // a total of 101
	limiter.Add(10) // one past

	numTaken, waitDuration := limiter.Take()
	assert.Equal(t, uint64(10), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(1)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(10), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(1)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(10), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(1)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(71), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(1)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(1009999996), waitDuration) // 1.01 seconds minus 4 nanoseconds

	mc.nextNow = mc.nextNow.Add(waitDuration)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(10), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(time.Second)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)
}

func TestBlockLimitMultipleTakesInterlevedAdds(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := New(100, WithClock(mc), TakeByBlock())

	limiter.Add(10)

	numTaken, waitDuration := limiter.Take()
	assert.Equal(t, uint64(10), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(1)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	limiter.Add(10)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(10), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(1)

	limiter.Add(81)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(81), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(100)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(1009999898), waitDuration)

	limiter.Add(100)

	mc.nextNow = mc.nextNow.Add(time.Second)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(9999898), waitDuration)

	mc.nextNow = mc.nextNow.Add(waitDuration)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(100), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(1)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)
}

func TestBlockLimitMultipleLargeInitialAdd(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := New(100, WithClock(mc), TakeByBlock())

	limiter.Add(1000)

	numTaken, waitDuration := limiter.Take()
	assert.Equal(t, uint64(1000), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(time.Second)

	limiter.Add(1)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, 9*time.Second, waitDuration)

	mc.nextNow = mc.nextNow.Add(waitDuration)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(1), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)

	mc.nextNow = mc.nextNow.Add(1)

	numTaken, waitDuration = limiter.Take()
	assert.Equal(t, uint64(0), numTaken)
	assert.Equal(t, time.Duration(0), waitDuration)
}

type mockClock struct {
	nextNow time.Time
}

func (mc *mockClock) Now() time.Time {
	return mc.nextNow
}
