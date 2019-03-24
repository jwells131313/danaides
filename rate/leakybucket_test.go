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

type mockClock struct {
	nextNow time.Time
}

func (mc *mockClock) Now() time.Time {
	return mc.nextNow
}
