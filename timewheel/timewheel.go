package timewheel

import (
	"errors"
	"sync/atomic"
	"time"
	"unsafe"
)

// TimeWheel 实现一个时间轮定时器，在指定时间运行指定的任务
type TimeWheel struct {
	tick int64 // 时间跨度，单位是ms
	size int64 // 时间轮格数

	interval    int64 // 总体时间跨度，interval=tick × size
	currentTime int64 // 当前运行的时间,是 tickMs 的整数倍

	buckets   []*bucket // 存储任务的桶
	stop      chan struct{}
	waitGroup waitGroupWrapper
	queue     *DelayQueue

	// 层级时间轮，类似于链表结构，每一层时间轮都会指向下一层时间轮
	overflowWheel unsafe.Pointer
}

// New 实例化一个时间轮定时器
func New(tick time.Duration, wheelSize int64) *TimeWheel {
	// 根据tick计算定时器的执行间隔，单位是毫秒
	tickMs := int64(tick / time.Millisecond)
	if tickMs <= 0 {
		panic(errors.New("tick must be greater than or equal to 1ms"))
	}

	// 初始化一个延迟队列，任何层的时间轮都是共用一个队列
	queue := newQueue(int(wheelSize))
	return newTimeWheel(tickMs, wheelSize, time.Now().UnixMilli(), queue)
}

func newTimeWheel(tickMs int64, wheelSize int64, startMs int64, queue *DelayQueue) *TimeWheel {
	// 初始化任务桶
	buckets := make([]*bucket, wheelSize)
	for i := 0; i < int(wheelSize); i++ {
		buckets[i] = newBucket()
	}

	return &TimeWheel{
		tick:        tickMs,
		size:        wheelSize,
		interval:    tickMs * wheelSize,
		currentTime: truncate(startMs, tickMs),
		buckets:     buckets,
		stop:        make(chan struct{}),
		queue:       queue,
	}
}

// advanceClock 移动指针
func (tw *TimeWheel) advanceClock(expiration int64) {
	// 加载当前时间
	currentTime := atomic.LoadInt64(&tw.currentTime)
	if expiration >= currentTime+tw.tick {
		// 将时间前进一个时间刻度
		currentTime = truncate(expiration, tw.tick)
		// 更新currentTime
		atomic.StoreInt64(&tw.currentTime, currentTime)

		// 同时将更高层级的时间轮的currentTime也前拨
		overflowWheel := atomic.LoadPointer(&tw.overflowWheel)
		if overflowWheel != nil {
			(*TimeWheel)(overflowWheel).advanceClock(currentTime)
		}
	}
}

func (tw *TimeWheel) Start() {
	// 开启协程，执行拉取任务的逻辑
	tw.waitGroup.Wrap(func() {
		tw.queue.Poll(tw.stop, func() int64 {
			return time.Now().UnixMilli()
		})
	})

	// 开启协程，执行执行任务的逻辑
	tw.waitGroup.Wrap(func() {
		for {
			select {
			case b := <-tw.queue.C:
				// 调整指针
				tw.advanceClock(b.expiration)
				// 对任务列表进行遍历，将到期的任务执行，未到期的继续插入
				b.Flush(tw.addOrRun)
			case <-tw.stop:
				return

			}
		}
	})
}

// Stop 关闭
func (tw *TimeWheel) Stop() {
	close(tw.stop)
	tw.waitGroup.Wait()
}

// AfterFunc
func (tw *TimeWheel) AfterFunc(d time.Duration, f func()) *timer {
	t := &timer{
		expiration: time.Now().Add(d).UnixMilli(),
		task:       f,
	}

	tw.addOrRun(t)

	return t
}

// add 添加任务，如果任务已过期，则返回false
func (tw *TimeWheel) add(t *timer) bool {
	// 加载当前时间
	currentTime := atomic.LoadInt64(&tw.currentTime)
	// 判断任务的执行时间是否在当前时间轮的执行期内
	if t.expiration < currentTime+tw.tick {
		// 返回false代表让任务直接执行
		return false
	} else if t.expiration < currentTime+tw.interval {
		// 没到执行时间且在本层时间轮里执行
		// 找到对应的bucket
		virtualID := t.expiration / tw.tick
		b := tw.buckets[virtualID%tw.size]
		b.Add(t)

		// 更新执行时间成功后，需要调整队列的优先级,如果时间相同，则不需要调整
		if b.SetExpiration(virtualID * tw.tick) {
			tw.queue.Offer(b, b.expiration)
		}

		return true
	} else { // 当执行时间在此周期外
		overflowWheel := atomic.LoadPointer(&tw.overflowWheel)
		if overflowWheel == nil {
			// 初始化下一层的时间轮
			atomic.CompareAndSwapPointer(
				&tw.overflowWheel,
				nil,
				unsafe.Pointer(newTimeWheel(tw.interval, tw.size, currentTime, tw.queue)),
			)
			// 再加载一次
			overflowWheel = atomic.LoadPointer(&tw.overflowWheel)
		}
		// 利用递归的思想，将任务插入到对应层的bucket里
		return (*TimeWheel)(overflowWheel).add(t)
	}

}
func (tw *TimeWheel) addOrRun(t *timer) {
	if !tw.add(t) {
		// 任务已过期，直接执行任务
		go t.task()
	}
}

// truncate returns the result of rounding x toward zero to a multiple of m.
// If m <= 0, Truncate returns x unchanged.
func truncate(x, m int64) int64 {
	if m <= 0 {
		return x
	}
	return x - x%m
}
