package bitarray

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBitmap(t *testing.T) {
	b := Bitmap(0)
	b = b.Set(0)
	assert.True(t, b.Has(0))
	//assert.Equal(t, 1, b.Count())
	//b = b.Clear(4)
	//assert.False(t, b.Has(4))
	//assert.Equal(t, 0, b.Count())
	//
	//b = b.Set(63)
	//assert.True(t, b.Has(63))
	//assert.Equal(t, 1, b.Count())
}

func TestPrintln(t *testing.T) {
	fmt.Println(1 << 1)
	fmt.Println(getIndexAndRemainder(1000))
}

func TestBitmapArray(t *testing.T) {
	ba := NewBitmapArray(1000)
	assert.True(t, ba.Empty())

	for i := 0; i < 100; i++ {
		if err := ba.Set(uint64(i)); err != nil {
			t.Fatal(err)
		}
	}

	assert.False(t, ba.Empty())
	fmt.Println(ba.ToNums())
	assert.Equal(t, 100, int(ba.Count()))
}
