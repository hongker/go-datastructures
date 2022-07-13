package timewheel

import (
	"container/list"
	"fmt"
	"testing"
)

func TestList(t *testing.T) {
	l := list.New()
	l.PushBack(1)
	l.PushBack(2)
	l.PushBack(3)
	for e := l.Front(); e != nil; {
		fmt.Println(e.Value)
		e = e.Next()
	}

	for e := l.Back(); e != nil; {
		fmt.Println(e.Value)
		e = e.Prev()
	}
}
