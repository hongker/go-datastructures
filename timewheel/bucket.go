package timewheel

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// bucket 存储任务列表的桶
type bucket struct {
	expiration int64 // 过期时间,需要用atomic保证并发安全

	mu     sync.Mutex
	timers *list.List
}

func (b *bucket) Expiration() int64 {
	return atomic.LoadInt64(&b.expiration)
}

func (b *bucket) SetExpiration(expiration int64) bool {
	return atomic.SwapInt64(&b.expiration, expiration) != expiration
}

// Flush 清空
func (b *bucket) Flush(reinsert func(t *timer)) {
	b.mu.Lock()

	// 遍历链表
	for e := b.timers.Front(); e != nil; {
		next := e.Next()

		t := e.Value.(*timer)
		b.remove(t)

		// 满足条件的直接执行，否则继续插入到其他bucket，等待到期再执行
		reinsert(t)

		e = next
	}

	b.SetExpiration(-1)
	b.mu.Unlock()
}

// remove 移除队列里的元素
func (b *bucket) remove(t *timer) bool {
	if t.getBucket() != b {
		return false
	}
	b.timers.Remove(t.element)
	t.setBucket(nil)
	t.element = nil
	return true
}

// Add 添加任务列表
func (b *bucket) Add(t *timer) {
	b.mu.Lock()
	elem := b.timers.PushBack(t)
	t.element = elem
	t.setBucket(b)
	b.mu.Unlock()

}

func newBucket() *bucket {
	return &bucket{
		expiration: -1,
		timers:     list.New(),
	}
}
