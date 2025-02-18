/*
Copyright 2014 Workiva, LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plus

func split(tree *btree, parent, child node) node {
	if !child.needsSplit(tree.nodeSize) {
		return parent
	}

	// 分裂节点，得到中间元素和左右子树
	key, left, right := child.split()
	if parent == nil { // 对于根节点，直接分裂成一个二叉树
		in := newInternalNode(tree.nodeSize)
		in.keys = append(in.keys, key) // 设置父节点的索引值
		in.nodes = append(in.nodes, left)
		in.nodes = append(in.nodes, right)
		return in
	}

	// 非根节点就可以根据parent节点来处理
	p := parent.(*inode)
	i := p.search(key)
	// we want to ensure if the children are leaves we set
	// the left node's left sibling to point to left
	if cr, ok := left.(*lnode); ok {
		if i > 0 {
			p.nodes[i-1].(*lnode).pointer = cr
		}
	}
	p.keys.insertAt(i, key)
	p.nodes[i] = left
	p.nodes.insertAt(i+1, right)

	return parent
}

type node interface {
	insert(tree *btree, key Key) bool
	needsSplit(nodeSize uint64) bool
	// key is the median key while left and right nodes
	// represent the left and right nodes respectively
	split() (Key, node, node)
	search(key Key) int
	find(key Key) *iterator
}

type nodes []node

func (nodes *nodes) insertAt(i int, node node) {
	if i == len(*nodes) {
		*nodes = append(*nodes, node)
		return
	}

	*nodes = append(*nodes, nil)
	copy((*nodes)[i+1:], (*nodes)[i:])
	(*nodes)[i] = node
}

func (ns nodes) splitAt(i int) (nodes, nodes) {
	left := make(nodes, i, cap(ns))
	right := make(nodes, len(ns)-i, cap(ns))
	copy(left, ns[:i])
	copy(right, ns[i:])
	return left, right
}

type inode struct {
	keys  keys  // 元素
	nodes nodes // 子树
}

func (node *inode) search(key Key) int {
	return node.keys.search(key)
}

func (node *inode) find(key Key) *iterator {
	// 非叶子节点的遍历，得先通过递归算法找到对应的叶子节点
	i := node.search(key)
	if i == len(node.keys) {
		return node.nodes[len(node.nodes)-1].find(key)
	}

	found := node.keys[i]
	switch found.Compare(key) {
	case 0, 1:
		return node.nodes[i+1].find(key)
	default:
		return node.nodes[i].find(key)
	}
}

func (n *inode) insert(tree *btree, key Key) bool {
	// 由于非叶子节点的元素=子节点的最大元素，故找到元素的位置=找到目标子节点的位置
	i := n.search(key)
	var child node
	if i == len(n.keys) { // we want the last child node in this case
		child = n.nodes[len(n.nodes)-1] // 当目标元素大于节点的最大元素，此刻就需要将数据插入到最后一个子节点
	} else {
		match := n.keys[i]
		switch match.Compare(key) {
		case 1, 0:
			child = n.nodes[i+1]
		default:
			child = n.nodes[i]
		}
	}

	// 如果child依然是非叶子节点，则继续地柜，直到为叶子节点
	result := child.insert(tree, key)
	if !result { // no change of state occurred
		return result
	}

	// 插入数据后，需要检查是否需要分裂叶子节点
	if child.needsSplit(tree.nodeSize) {
		split(tree, n, child)
	}

	return result
}

func (n *inode) needsSplit(nodeSize uint64) bool {
	return uint64(len(n.keys)) >= nodeSize
}

func (n *inode) split() (Key, node, node) {
	if len(n.keys) < 3 {
		return nil, nil, nil
	}

	i := len(n.keys) / 2
	key := n.keys[i]

	ourKeys := make(keys, len(n.keys)-i-1, cap(n.keys))
	otherKeys := make(keys, i, cap(n.keys))
	copy(ourKeys, n.keys[i+1:])
	copy(otherKeys, n.keys[:i])
	left, right := n.nodes.splitAt(i + 1)
	otherNode := &inode{
		keys:  otherKeys,
		nodes: left,
	}
	n.keys = ourKeys
	n.nodes = right
	return key, otherNode, n
}

func newInternalNode(size uint64) *inode {
	return &inode{
		keys:  make(keys, 0, size),
		nodes: make(nodes, 0, size+1),
	}
}

// lnode 叶子节点
type lnode struct {
	// points to the left leaf node is there is one
	pointer *lnode // 链表式指针，指向下一条数据，用于快速范围查找
	keys    keys   // 元素数组
}

func (node *lnode) search(key Key) int {
	return node.keys.search(key)
}

// insert 给叶子节点插入元素
func (lnode *lnode) insert(tree *btree, key Key) bool {
	// 查找合适的插入位置
	i := keySearch(lnode.keys, key)
	var inserted bool
	if i == len(lnode.keys) { // 数组为空或者目标元素最大
		lnode.keys = append(lnode.keys, key)
		inserted = true
	} else {
		// 判断元素是否已存在，存在就替换，否则就插入
		if lnode.keys[i].Compare(key) == 0 {
			lnode.keys[i] = key
		} else { // insert key at position
			lnode.keys.insertAt(i, key)
			inserted = true
		}
	}

	if !inserted {
		return false
	}

	return true
}

func (node *lnode) find(key Key) *iterator {
	// 根据key查找到对应的位置
	i := node.search(key)
	if i == len(node.keys) { // 如果元素不存在
		if node.pointer == nil { // 且没有更后的节点来查询
			return nilIterator()
		}

		// 后面还有节点可以遍历
		return &iterator{
			node:  node.pointer,
			index: -1,
		}
	}

	// 元素就在当前节点里
	iter := &iterator{
		node:  node,
		index: i - 1,
	}
	return iter
}

func (node *lnode) split() (Key, node, node) {
	if len(node.keys) < 2 {
		return nil, nil, nil
	}
	i := len(node.keys) / 2 // 取中间一个元素
	key := node.keys[i]
	otherKeys := make(keys, i, cap(node.keys))
	ourKeys := make(keys, len(node.keys)-i, cap(node.keys))
	// we perform these copies so these slices don't all end up
	// pointing to the same underlying array which may make
	// for some very difficult to debug situations later.
	copy(otherKeys, node.keys[:i])
	copy(ourKeys, node.keys[i:])

	// this should release the original array for GC
	node.keys = ourKeys
	// 左子树
	otherNode := &lnode{
		keys:    otherKeys,
		pointer: node, // 形成链表
	}
	return key, otherNode, node
}

func (lnode *lnode) needsSplit(nodeSize uint64) bool {
	return uint64(len(lnode.keys)) >= nodeSize
}

func newLeafNode(size uint64) *lnode {
	return &lnode{
		keys: make(keys, 0, size),
	}
}

type keys []Key

func (keys keys) search(key Key) int {
	return keySearch(keys, key)
}

func (keys *keys) insertAt(i int, key Key) {
	if i == len(*keys) {
		*keys = append(*keys, key)
		return
	}

	*keys = append(*keys, nil)
	copy((*keys)[i+1:], (*keys)[i:])
	(*keys)[i] = key
}

func (keys keys) reverse() {
	for i := 0; i < len(keys)/2; i++ {
		keys[i], keys[len(keys)-i-1] = keys[len(keys)-i-1], keys[i]
	}
}
