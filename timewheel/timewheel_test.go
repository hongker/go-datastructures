package timewheel

import (
	"fmt"
	"testing"
	"time"
)

func TestTimeWheel(t *testing.T) {
	tw := New(5*time.Millisecond, 12)
	tw.Start()
	defer tw.Stop()
	durations := []time.Duration{
		//10 * time.Millisecond,
		//50 * time.Millisecond,
		//100 * time.Millisecond,
		//500 * time.Millisecond,
		1 * time.Second,
		//2 * time.Second,
		//3 * time.Second,
	}

	for _, duration := range durations {
		d := duration
		tw.AfterFunc(d, func() {
			fmt.Println("task run", d, time.Now().UnixMilli())
		})

	}

	time.Sleep(time.Second * 5)

}
