package llb

import "math"

var (
	bsPool = Pool{}
)

// node 节点
type node struct {
	buf  []byte
	next *node
}

// len 获取节点的bytes长度
func (n *node) len() int {
	return len(n.buf)
}

type Buffer struct {
	bs    [][]byte // 缓存近一次Peek的数据
	head  *node    // 头节点
	tail  *node    // 尾节点
	size  int      // 节点数量
	bytes int      // 总长度
}

func (llb *Buffer) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	for b := llb.pop(); b != nil; b = llb.pop() { // 循环取头部节点
		m := copy(p[n:], b.buf) // 取出节点的数组进行复制到目标数组里
		n += m
		if m < b.len() { // 目标数组已满，且未读取完buf，则将剩余数据还回链表里
			b.buf = b.buf[m:]
			llb.pushFront(b)
		} else { // 读取完的数据，将其还回对象池
			bsPool.Put(b.buf)
		}

		if n == len(p) { // 满足数据的读取条件，直接返回
			return
		}
	}

	return
}

// PushFront
func (llb *Buffer) PushFront(p []byte) {
	n := len(p)
	if n == 0 {
		return
	}

	b := bsPool.Get(n)
	copy(b, p)
	llb.pushFront(&node{buf: b})
}

func (llb *Buffer) PushBack(p []byte) {
	n := len(p)
	if n == 0 {
		return
	}
	b := bsPool.Get(n)
	copy(b, p)
	llb.pushBack(&node{buf: b})
}

func (llb *Buffer) Len() int {
	return llb.size
}

func (llb *Buffer) Buffered() int {
	return llb.bytes
}

// Peek
func (llb *Buffer) Peek(maxBytes int) [][]byte {
	if maxBytes <= 0 {
		maxBytes = math.MaxInt32
	}

	llb.bs = llb.bs[:0]
	var cum int
	for iter := llb.head; iter != nil; iter = iter.next {
		llb.bs = append(llb.bs, iter.buf)
		if cum += iter.len(); cum >= maxBytes {
			break
		}
	}
	return llb.bs
}

// Discard 抛弃指定长度的数据
func (llb *Buffer) Discard(n int) (discarded int, err error) {
	if n <= 0 {
		return
	}
	for n != 0 {
		b := llb.pop()
		if b == nil {
			break
		}

		if n < b.len() {
			b.buf = b.buf[n:]
			discarded += n
			llb.pushFront(b)
			break
		}
		n -= b.len()
		discarded += b.len()
		bsPool.Put(b.buf)
	}
	return
}

func (llb *Buffer) IsEmpty() bool {
	return llb.head == nil
}

// Reset 重置所有节点
func (llb *Buffer) Reset() {
	for b := llb.pop(); b != nil; b = llb.pop() {
		bsPool.Put(b.buf) // 回收
	}
	llb.head = nil
	llb.tail = nil
	llb.size = 0
	llb.bytes = 0
	llb.bs = llb.bs[:0]
}

// pop 返回一个节点
func (llb *Buffer) pop() *node {
	if llb.head == nil { // 如果头部信息
		return nil
	}

	b := llb.head        // 取头部节点
	llb.head = b.next    // 指向next
	if llb.head == nil { // 如果head为nil,则tail也应该为nil
		llb.tail = nil
	}
	b.next = nil
	llb.size--
	llb.bytes -= b.len()
	return b
}

// pushFront 在链表头部插入一个节点
func (llb *Buffer) pushFront(n *node) {
	if n == nil {
		return
	}

	if llb.head == nil { // 当链表为空链表
		n.next = nil
		llb.tail = n
	} else { // 链表不为空，直接指向头部节点
		n.next = llb.head
	}

	llb.head = n
	llb.size++
	llb.bytes += n.len()
}

// pushBack 在链表尾部插入一个节点
func (llb *Buffer) pushBack(n *node) {
	if n == nil {
		return
	}

	if llb.tail == nil { // 空链表
		llb.head = n
	} else {
		llb.tail.next = n
	}
	n.next = nil
	llb.tail = n
	llb.size++
	llb.bytes += n.len()
}
