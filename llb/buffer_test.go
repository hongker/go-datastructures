package llb

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

func TestBuffer(t *testing.T) {
	const maxBlocks = 100

	var (
		llb Buffer
		cum int
		buf bytes.Buffer
	)

	rand.Seed(time.Now().Unix())
	for i := 0; i < maxBlocks; i++ {
		n := rand.Intn(1024) + 128
		cum += n
		data := make([]byte, n)
		rand.Read(data)
		llb.PushBack(data)
		buf.Write(data)
	}

	require.EqualValues(t, maxBlocks, llb.Len())
	require.EqualValues(t, cum, llb.Buffered())
}
