package btree

import "sort"

// Item是一个接口类型，含有一个Less方法，通过这个接口可以实现类似泛型的功能。
type Item interface {
	Less(than Item) bool
}

// find 查询元素位置
func (items Items) find(t Item) (index int, found bool) {
	// search方法使用二分搜索去返回元素中最小的满足[0,n)中最小的满足f(i)函数为true的索引i，如果f(i)为true，则f(i+1)也为true
	i := sort.Search(len(items), func(i int) bool {
		return t.Less(items[i])
	})
	if i > 0 && !items[i-1].Less(t) {
		// 当上一个元素刚好和t相等时，返回下标和true
		return i - 1, true
	}

	// 返回第一个大于目标元素的数组索引
	return i, false
}

// items指定位置插入一个Item
func (items *Items) insertAt(index int, item Item) {
	*items = append(*items, nil) // 扩展一个位置出来
	if index < len(*items) {     // 可能index==len，需要插入的位置就在最后一个
		copy((*items)[index+1:], (*items)[index:]) // 位置后面的往后挪一下
	}
	(*items)[index] = item // 覆盖指定位置的数据
}

type Items []Item

type IntItem int

// Less returns true if int(a) < int(b).
func (a IntItem) Less(b Item) bool {
	return a < b.(IntItem)
}

type BTree interface {
	// 向btree插入item
	// 如果item已经在tree中，则返回item
	// 如果是新增节点，则返回nil
	ReplaceOrInsert(item Item) Item

	// 如果item 存在，则从树中删除，并返回item
	// 如果要删除的item 不存在，则返回nil
	Delete(item Item) Item
}

type btree struct {
	degree uint  //树的度
	root   *node // 根节点
}

// MinCap 最小容量，也就是每个节点的元素个数大于等于最小容量
func (tree *btree) MinCap() int {
	return int(tree.degree - 1)
}

// MaxCap 最大容量,也就是每个节点的元素个数小雨等于最大容量
func (tree *btree) MaxCap() int {
	return int(2*tree.degree - 1)
}

func (tree *btree) Get(key Item) Item {
	if tree.root == nil {
		return nil
	}

	return tree.root.get(key)
}

// ReplaceOrInsert 插入
func (tree *btree) ReplaceOrInsert(item Item) Item {
	//如果是空树，则创建根后插入
	if tree.root == nil {
		tree.root = new(node)
		tree.root.items = append(tree.root.items, item)
		return nil
	}

	// 从跟节点向下寻找合适的插入节点，对于新的item，最终的插入会落到叶子节点，
	// 下降过程中，每个经过的节点如果已满，聚会进行一次分裂动作，
	// 这样保证后续插入的时候，实际插入的节点肯定会有空闲空间供插入

	// 对有可能改变根节点的情况进行单独处理
	// 如果根结点已满，对根节点进行分裂处理
	// 这里的MaxCap返回的是每个节点子树的最大树木，对于4阶树就是4
	if len(tree.root.items) >= tree.MaxCap() {
		midIndex := len(tree.root.items) / 2
		upItem, newNode := tree.root.split(midIndex)
		newRoot := new(node)
		newRoot.items = append(newRoot.items, upItem)
		newRoot.children = append(newRoot.children, tree.root, newNode)
		tree.root = newRoot
	}

	return tree.root.insert(item, tree.MaxCap())
}

func (tree *btree) Delete(item Item) Item {
	return tree.root.remove(item, tree.MinCap())
}
func (n *node) remove(item Item, minCap int) Item {
	i, found := n.items.find(item)
	// 如果node节点的子树i中items小于等于最小键个数，则先对其进行一次调整处理, 保证到叶子节点时其可以直接删除

	// 如果子树i需要调整，则调整节点后，重新进行remove操作
	if len(n.children) != 0 && len(n.children[i].items) <= minCap {
		return n.growChildAndRemove(i, item, minCap)
	}
	if found {
		if len(n.children) == 0 {
			// 如果查找到元素，且元素在叶子节点，则直接删除并返回
			// 因为在进入叶子节点之前，已经通过growChildAndRemove做了扩容处理，保证到达叶子是，一定可以直接删除而不破坏结构
			return n.items.removeAt(i)
		} else {
			// 从n.childrend[i]为根的子树依次向下，最终删除一个最大值，替换n.items[i-1]原来的值

			// 如果查找到要删除的元素不在叶子节点上， 则删除之，并从children[i]为根的子树上提取最大元素放入要删除的位置
			out := n.items[i]
			n.items[i] = n.children[i].removeMax(minCap)
			return out
		}
	} else {
		if len(n.children) == 0 {
			return nil
		} else {
			child := n.children[i]
			return child.remove(item, minCap)
		}

	}
}
func (n *node) removeMax(minCap int) Item {
	lastItemIndex := len(n.items) - 1
	if len(n.children) == 0 {
		return n.items.removeAt(lastItemIndex)
	} else if len(n.children[lastItemIndex+1].items) <= minCap {
		lastChildIndex := lastItemIndex + 1
		return n.growChildAndRemoveMax(lastChildIndex, minCap) //
	}

	lastChildIndex := lastItemIndex + 1
	lastChildNode := n.children[lastChildIndex]
	return lastChildNode.removeMax(minCap)
}

// removeAt 移除元素
func (s *Items) removeAt(index int) (out Item) {
	out = (*s)[index]
	if index+1 < len(*s) {
		copy((*s)[index:], (*s)[index+1:])
	}

	*s = (*s)[:len(*s)-1]
	return
}

// removeAt 移除子节点
func (s *children) removeAt(index int) (n *node) {
	n = (*s)[index]
	if index+1 < len(*s) {
		copy((*s)[index:], (*s)[index+1:])
	}
	*s = (*s)[:len(*s)-1]
	return
}
func (parent *node) growChildAndRemoveMax(lastChildIndex int, minCap int) Item {

	// 从左兄弟子树偷一个尾部item放入parent item[i-1],然后源parent item[i-1]下移插入到parent.children[i]的头部
	if len(parent.children[lastChildIndex-1].items) > minCap {
		lastItemIndex := lastChildIndex - 1

		stoleNode := parent.children[lastChildIndex-1]
		stoleItemIndex := len(stoleNode.items) - 1
		stoleItem := stoleNode.items.removeAt(stoleItemIndex)
		downItem := parent.items[lastItemIndex]
		parent.children[lastChildIndex].items.insertAt(0, downItem)
		parent.items[lastItemIndex] = stoleItem
		if len(stoleNode.children) != 0 {
			lastIndex := len(stoleNode.children) - 1
			nodeLastChildren := stoleNode.children.removeAt(lastIndex)
			parent.children[lastChildIndex].children.insertAt(0, nodeLastChildren)
		}
	} else {
		// combing subling

		lastItemIndex := lastChildIndex - 1
		n := &node{}
		n.items = append(n.items, parent.children[lastChildIndex-1].items...)
		n.items = append(n.items, parent.items[lastItemIndex])
		n.items = append(n.items, parent.children[lastChildIndex].items...)

		if len(parent.children[lastChildIndex-1].children) != 0 {
			n.children = append(n.children, parent.children[lastChildIndex-1].children...)
			n.children = append(n.children, parent.children[lastChildIndex].children...)
		}

		if len(parent.items) == 1 {
			*parent = *n //注意这里要对对指针指向内容重新赋值
		} else {
			parent.items.removeAt(lastItemIndex)
			parent.children.removeAt(lastChildIndex)
			parent.children[lastChildIndex-1] = n
		}

	}

	return parent.removeMax(minCap)
}
func (parent *node) growChildAndRemove(index int, t Item, minCap int) Item {
	// 从左兄弟子树偷一个尾部item放入parent item[i-1],然后源parent item[i-1]下移插入到parent.children[i]的头部
	if index > 0 && len(parent.children[index-1].items) > minCap {

		stoleNode := parent.children[index-1]
		stoleNodeIndex := len(parent.children[index-1].items) - 1
		stoleItem := stoleNode.items.removeAt(stoleNodeIndex)
		parent.children[index].items.insertAt(0, parent.items[index-1])
		parent.items[index-1] = stoleItem
		if len(stoleNode.children) != 0 {
			lastIndex := len(stoleNode.children) - 1
			nodeLastChildren := stoleNode.children.removeAt(lastIndex)
			parent.children[index].children.insertAt(0, nodeLastChildren)
		}

		return parent.remove(t, minCap)

	}

	// 从右子树偷
	if index < len(parent.children)-1 && len(parent.children[index+1].items) > minCap {

		stoleNode := parent.children[index+1]
		parent.children[index].items = append(parent.children[index].items, parent.items[index])
		stoleItem := stoleNode.items.removeAt(0)
		parent.items[index] = stoleItem

		if len(stoleNode.children) != 0 {
			nodeFirstChildren := stoleNode.children.removeAt(0)
			parent.children[index].children = append(parent.children[index].children, nodeFirstChildren)
		}

		return parent.remove(t, minCap)

	}

	//无可偷兄弟节点，需要做一次兄弟合并
	if index > 0 {
		// 非最左节点， 则合并左兄弟
		n := &node{}
		n.items = append(n.items, parent.children[index-1].items...)
		n.items = append(n.items, parent.items[index-1])
		n.items = append(n.items, parent.children[index].items...)

		if len(parent.children[index-1].children) != 0 {
			n.children = append(n.children, parent.children[index-1].children...)
			n.children = append(n.children, parent.children[index].children...)
		}

		// 处理根节点被删空的情况
		if len(parent.items) == 1 {
			*parent = *n
		} else {
			parent.items.removeAt(index - 1)
			parent.children.removeAt(index - 1)
			parent.children[index-1] = n //合并后index前移一位
		}
		return parent.remove(t, minCap)
	}

	if index+1 < len(parent.children) {
		// 最左边不可偷节点，合并右侧兄弟节点
		n := &node{}
		n.items = append(n.items, parent.children[index].items...)
		n.items = append(n.items, parent.items[index])
		n.items = append(n.items, parent.children[index+1].items...)

		if len(parent.children[index].children) != 0 {
			n.children = append(n.children, parent.children[index].children...)
			n.children = append(n.children, parent.children[index+1].children...)
		}

		if len(parent.items) == 1 {
			*parent = *n
		} else {
			parent.items.removeAt(index)
			parent.children.removeAt(index + 1)
			parent.children[index] = n
		}

		return parent.remove(t, minCap)
	}

	return nil
}

// node代表btree的一个节点
type node struct {
	items    Items    // 元素数组
	children children // 子节点的指针数组
}

func (n *node) get(key Item) Item {
	idx, found := n.items.find(key)
	if found {
		return n.items[idx]
	} else if len(n.children) > 0 {
		return n.children[idx].get(key)
	}
	return nil
}

// 修改node的items和children,使其减半
func (n *node) split(mid int) (Item, *node) {
	if len(n.items)-1 < mid {
		panic("error index")
	}

	upItem := n.items[mid]
	// 新增加一个节点
	newNode := &node{}
	// 将原节点的元素分一半给新节点
	newNode.items = append(newNode.items, n.items[mid+1:]...)
	// 原节点只保存一半
	n.items = n.items[:mid]

	// 子节点也要分半
	if n.children != nil {
		newNode.children = append(newNode.children, n.children[mid+1:]...)
		n.children = n.children[:mid+1]
	}

	// 返回当前元
	return upItem, newNode
}

// maybeSplitChild 对children进行判断
// 1.node的items和children元素个数分别+1,保证不破坏btree的属性
// 2.保证了后续插入查询地柜下沉到node的某一个子树的时候，子树items未满
func (n *node) maybeSplitChild(childIndex, maxCap int) bool {
	l := len(n.children[childIndex].items)

	// 如果items已满，则进行一次分裂
	if l >= maxCap {
		upItem, newNode := n.children[childIndex].split(l / 2)

		// 将中间的那个数据项插入到childIndex的位置
		n.items.insertAt(childIndex, upItem)
		// 将新节点插入到children里，位置为childIndex+1
		n.children.insertAt(childIndex+1, newNode)

		return true
	} else {
		return false
	}
}

// 指定位置插入节点
func (s *children) insertAt(index int, n *node) {
	(*s) = append((*s), nil)
	copy((*s)[index+1:], (*s)[index:])
	(*s)[index] = n
}

func (n *node) insert(item Item, maxCap int) Item {
	index, found := n.items.find(item)
	if found {
		return item
	}
	// 是叶子节点，直接将数据插入items
	if len(n.children) == 0 {
		n.items.insertAt(index, item)
		return nil
	}

	// 不是叶子节点，看看i处的子Node是否需要分裂， 这里的操作是为了保证后面进行插入下沉到子节点时，子节点n.children[index]一定未满
	if n.maybeSplitChild(index, maxCap) {
		// 分裂了，导致当前node的变化，需要重新定位
		// 保证插入查询能下沉到items符合范围的子树中
		inTree := n.items[index] // 获取新升级的item
		switch {
		case item.Less(inTree):
			// 要插入的item比分裂产生的item小，i没改变
		case inTree.Less(item):
			index++ // 要插入的item比分裂产生的item大，i++
		default:
			// 分裂升level的item和插入的item一致，替换
			out := n.items[index]
			n.items[index] = item
			return out
		}
	}

	// 递归插入到items符合插入范围的子树
	return n.children[index].insert(item, maxCap)
}

type children []*node

// 传入degree， 初始化一棵空树，确定btreed的节点最小度
func NewBTree(degree uint) BTree {
	return &btree{
		degree: degree,
	}
}
