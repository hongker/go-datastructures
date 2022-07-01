package btree

import (
	"testing"
)

func TestItem(t *testing.T) {
	tree := NewBTree(3)
	for i := 0; i < 10; i++ {
		tree.ReplaceOrInsert(IntItem(i))
	}
}
