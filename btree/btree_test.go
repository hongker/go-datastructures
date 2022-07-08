package btree

import (
	"fmt"
	"testing"
)

func TestItem(t *testing.T) {
	tree := NewBTree(2)

	items := []IntItem{38, 21, 40, 96, 20, 39, 41, 42, 46, 43, 44}
	for _, item := range items {
		tree.ReplaceOrInsert(item)
	}
	fmt.Println(tree)
}
