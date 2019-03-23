// Copyright (c) 2019, Oracle and/or its affiliates. All rights reserved.

package rate

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestFundedTakes(t *testing.T) {
	mc := &mockClock{}
	mc.nextNow = time.Now()

	limiter := NewWithClock(100, mc)

	// should add 1000 to the bucket
	limiter.Add(1000)

	// wait a minute
	mc.nextNow = mc.nextNow.Add(time.Minute)

	took := limiter.Take(nil)

	assert.Equal(t, uint64(100), took)

	// wait one tenth of a second
	mc.nextNow = mc.nextNow.Add(100 * time.Millisecond)

	took = limiter.Take(nil)

	assert.Equal(t, uint64(11), took)
}

type mockClock struct {
	nextNow time.Time
}

func (mc *mockClock) Now() time.Time {
	return mc.nextNow
}
