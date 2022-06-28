package batcher

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBatcherNew(t *testing.T) {
	assert := assert.New(t)
	b := NewBatcher(10, WithMaxItems(200))
	for i := 0; i < 1000; i++ {
		assert.Nil(b.Put("foo bar baz"))
	}

	batch, err := b.Get()
	assert.Len(batch, 200)
	assert.Nil(err)
}
