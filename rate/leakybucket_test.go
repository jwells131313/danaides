// Copyright (c) 2019, Oracle and/or its affiliates. All rights reserved.

package rate

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestFundedTakesLongInitialWait(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := NewWithClock(100, mc)

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

	limiter := NewWithClock(100, mc)

	// should add 1000 to the bucket
	limiter.Add(100)

	// 9 runs gets rid of 99 bytes
	for lcv := 0; lcv < 9; lcv++ {
		mc.nextNow = mc.nextNow.Add(100 * time.Millisecond)

		took, delay := limiter.Take()

		assert.Equal(t, uint64(11), took)
		assert.Equal(t, 100-(11*(lcv+1)), int(limiter.GetBucketSize()))
		assert.Equal(t, time.Duration(0), delay)
	}

	// wait one tenth of a second
	mc.nextNow = mc.nextNow.Add(100 * time.Millisecond)

	took, delay := limiter.Take()

	assert.Equal(t, uint64(1), took)
	assert.Equal(t, uint64(0), limiter.GetBucketSize())
	assert.Equal(t, time.Duration(0), delay)

	// should not be able to take as we've already parcelled out the 100 for this second
	took, delay = limiter.Take()

	assert.Equal(t, uint64(0), took)
	assert.Equal(t, uint64(0), limiter.GetBucketSize())
	assert.Equal(t, time.Duration(0), delay)
}

func TestTwoAndAHalfSeconds(t *testing.T) {
	limiter := New(100)

	now := time.Now()
	limiter.Add(250)

	totalTook := 0
	for {
		took, delay := limiter.Take()

		if took == 0 && delay == 0 {
			break
		}

		if took == 0 {
			fmt.Printf("JRW(10) sleeping %d milli seconds\n", delay/time.Millisecond)
			time.Sleep(delay)
		} else {
			fmt.Printf("JRW(20) got %d bytes\n", took)
			totalTook += int(took)
		}
	}

	elapsed := time.Now().Sub(now)

	assert.Equal(t, 250, totalTook)
	assert.True(t, elapsed >= ((2*time.Second)+(500*time.Millisecond)))
}

type mockClock struct {
	nextNow time.Time
}

func (mc *mockClock) Now() time.Time {
	return mc.nextNow
}
