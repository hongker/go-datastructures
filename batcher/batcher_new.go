package batcher

import "time"

type BatcherNew struct {
	items     []interface{}      // 元素数组
	batchChan chan []interface{} // 缓冲区
	lock      *mutex             // 锁
	disposed  bool               // 弃用
	arrayLen  uint               // 数组长度
	option    Option             // 选项
}

type Option struct {
	maxTime        time.Duration  // 按时间
	maxItems       uint           // 按数量
	maxBytes       uint           // 按长度
	availableBytes uint           // 可用长度
	calculateBytes CalculateBytes // 计算长度的函数
}

func (b *BatcherNew) Put(item interface{}) error {
	b.lock.Lock()
	if b.disposed { // 判断队列是否可用
		b.lock.Unlock()
		return ErrDisposed
	}

	// 添加元素
	b.items = append(b.items, item)
	if b.option.calculateBytes != nil { // 计算长度
		b.option.availableBytes += b.option.calculateBytes(item)
	}
	if b.ready() { // 如果满足条件，将数据刷入缓冲区
		b.flush()
	}
	b.lock.Unlock()
	return nil
}
func (b *BatcherNew) ready() bool {
	// 按数量判断
	if b.option.maxItems != 0 && uint(len(b.items)) >= b.option.maxItems {
		return true
	}
	// 按字节数判断
	if b.option.maxBytes != 0 && b.option.availableBytes >= b.option.maxBytes {
		return true
	}
	return false
}

func (b *BatcherNew) Get() ([]interface{}, error) {
	// 定时器
	var timeout <-chan time.Time
	if b.option.maxTime > 0 {
		timeout = time.After(b.option.maxTime)
	}

	select {
	case items, ok := <-b.batchChan:
		if !ok {
			return nil, ErrDisposed
		}
		return items, nil
	case <-timeout:
		for {
			if b.lock.TryLock() {
				select {
				case items, ok := <-b.batchChan:
					b.lock.Unlock()
					if !ok {
						return nil, ErrDisposed
					}
					return items, nil
				default:
				}
				// 直接取当前数据项
				items := b.items
				b.items = make([]interface{}, 0, b.arrayLen)
				b.option.availableBytes = 0
				b.lock.Unlock()
				return items, nil
			} else {
				select {
				case items, ok := <-b.batchChan:
					if !ok {
						return nil, ErrDisposed
					}
					return items, nil
				}
			}
		}

	}
}

func (b *BatcherNew) Flush() error {
	// This is the same pattern as a Put
	b.lock.Lock()
	if b.disposed {
		b.lock.Unlock()
		return ErrDisposed
	}
	b.flush()
	b.lock.Unlock()
	return nil
}

// flush 将数组输出到缓冲区
func (b *BatcherNew) flush() {
	b.batchChan <- b.items
	// 重新初始化
	b.items = make([]interface{}, 0, b.arrayLen)
	b.option.availableBytes = 0
}

func (b *BatcherNew) Dispose() {
	for {
		if b.lock.TryLock() {
			if b.disposed {
				b.lock.Unlock()
				return
			}
			b.disposed = true
			b.items = nil
			b.drainBatchChan()
			close(b.batchChan)
			b.lock.Unlock()
		} else {
			b.drainBatchChan()
		}
	}
}

func (b *BatcherNew) IsDisposed() bool {
	b.lock.Lock()
	disposed := b.disposed
	b.lock.Unlock()
	return disposed
}

func (b *BatcherNew) drainBatchChan() {
	for {
		select {
		case <-b.batchChan:
		default:
			return
		}
	}
}

func WithMaxTime(maxTime time.Duration) Option {
	return Option{maxTime: maxTime}
}
func WithMaxItems(maxItems uint) Option {
	return Option{maxItems: maxItems}
}
func WithMaxBytes(maxBytes uint, calculateBytes CalculateBytes) Option {
	return Option{maxBytes: maxBytes, calculateBytes: calculateBytes}
}

// NewBatcher 初始化
func NewBatcher(queueLen uint, option Option) Batcher {
	var arrayLen uint = 1024
	if option.maxItems > 0 {
		arrayLen = option.maxItems
	}
	return &BatcherNew{
		option:    option,
		items:     make([]interface{}, 0, arrayLen),
		batchChan: make(chan []interface{}, queueLen),
		lock:      newMutex(),
		disposed:  false,
		arrayLen:  arrayLen,
	}
}
