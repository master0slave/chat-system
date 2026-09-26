package ids_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"supportchat/pkg/ids"
)

func TestIDsSortInCreationOrder(t *testing.T) {
	gen := ids.UUIDv7{}
	prev := gen.NewID()
	for range 10_000 { // many IDs land in the same millisecond
		next := gen.NewID()
		assert.Less(t, prev, next)
		prev = next
	}
}
