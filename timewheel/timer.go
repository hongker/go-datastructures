package timewheel

import (
	"container/list"
	"sync/atomic"
	"unsafe"
)

type timer struct {
	expiration int64 // in milliseconds

	// 当出现并发读写的时候，对于指针都可以用unsafe.Pointer处理
	bucket  unsafe.Pointer // bucket,需要用atomic保证并发安全
	element *list.Element
	task    func() // 具体任务
}

func (t *timer) getBucket() *bucket {
	return (*bucket)(atomic.LoadPointer(&t.bucket))
}

func (t *timer) setBucket(bucket *bucket) {
	atomic.StorePointer(&t.bucket, unsafe.Pointer(bucket))
}

// Stop 停止任务,当任务已执行时返回false
func (t *timer) Stop() (stopped bool) {
	for b := t.getBucket(); b != nil; b = t.getBucket() {
		stopped = b.remove(t)
	}
	return
}
