package hashmap

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHashMap(t *testing.T) {
	hmap := New(8)
	for i := uint64(0); i < 100; i++ {
		hmap.Set(i, fmt.Sprintf("val:%d", i))
	}

	val, exist := hmap.Get(88)
	assert.True(t, exist)
	assert.Equal(t, "val:88", val)
}

func TestRoundUp(t *testing.T) {
	assert.Equal(t, uint64(1), roundUp(1))
	assert.Equal(t, uint64(2), roundUp(2))
	assert.Equal(t, uint64(4), roundUp(3))
	assert.Equal(t, uint64(8), roundUp(5))
	assert.Equal(t, uint64(16), roundUp(9))
	assert.Equal(t, uint64(32), roundUp(31))
	assert.Equal(t, uint64(64), roundUp(63))

}
