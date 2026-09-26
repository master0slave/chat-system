package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"supportchat/pkg/clock"
)

func TestNowIsUTCMilliseconds(t *testing.T) {
	now := clock.System{}.Now()

	assert.Equal(t, time.UTC, now.Location())
	assert.Zero(t, now.Nanosecond()%int(time.Millisecond))
}
