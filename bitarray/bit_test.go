package bitarray

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"runtime"
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

func GetMem() uint64 {
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)
	return memStat.Sys
}

func TestBitmapArray(t *testing.T) {
	before := GetMem()
	items := make([]*BitmapArray, 20)
	for j := 0; j < 20; j++ {
		n := uint64(1000000)
		ba := NewBitmapArray(n)
		assert.True(t, ba.Empty())

		for i := uint64(0); i < n; i++ {
			ba.Set(i)
		}
		items[j] = ba
	}

	after := GetMem()
	fmt.Printf("before:%d, after:%d, memory usage: %.3f K\n", before, after, float64(after-before)/1024)

}
