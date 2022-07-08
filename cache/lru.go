package cache

import (
	"container/list"
	"sync"
)

type LRUCache struct {
	sync.Mutex                    // 锁，保证并发安全
	cap        uint64             // 最大容量
	size       uint64             // 缓存size
	items      map[string]*cached // 缓存数据
	keyList    *list.List         // 链表，用于存储最近使用的key
}

func NewLRUCache(capacity uint64) *LRUCache {
	c := &LRUCache{
		cap:     capacity,
		keyList: list.New(),
		items:   map[string]*cached{},
	}
	return c
}

func (cache *LRUCache) Get(key string) Item {
	cache.Lock()
	defer cache.Unlock()
	return cache.get(key)
}

func (cache *LRUCache) get(key string) Item {
	// 判断数据是否存在
	cached, exist := cache.items[key]
	if !exist {
		return nil
	}

	// 触发最近使用策略
	cache.record(key)
	return cached.item
}

func (cache *LRUCache) BatchGet(keys ...string) []Item {
	cache.Lock()
	defer cache.Unlock()
	items := make([]Item, len(keys))
	for i, key := range keys {
		items[i] = cache.get(key)
	}
	return items
}

// ensureCapacity 根据新增的数据的长度，判断总容量是否溢出，如果满足，则需要删除一部分数据，直到容量正常
func (cache *LRUCache) ensureCapacity(toAdd uint64) {
	mustRemove := int64(cache.size+toAdd) - int64(cache.cap)
	for mustRemove > 0 {
		key := cache.keyList.Back().Value.(string)
		mustRemove -= int64(cache.items[key].item.Size())
		cache.remove(key)
	}
}
func (cache *LRUCache) Put(key string, item Item) {
	cache.Lock()
	defer cache.Unlock()
	cache.Remove(key)

	cache.ensureCapacity(item.Size())
	cached := &cached{item: item}
	cached.setElementIfNotNil(cache.record(key))
	cache.items[key] = cached
	cache.size += item.Size()
}

func (cache *LRUCache) Remove(keys ...string) {
	cache.Lock()
	defer cache.Unlock()
	for _, key := range keys {
		cache.remove(key)
	}
}

func (cache *LRUCache) remove(key string) {
	if cached, ok := cache.items[key]; ok {
		// 删除数组里的元素
		delete(cache.items, key)
		// 减去大小
		cache.size -= cached.item.Size()
		// 移除链表中的元素
		cache.keyList.Remove(cached.element)
	}
}

func (cache *LRUCache) Size() uint64 {
	return cache.size
}

// A function to record the given key and mark it as last to be evicted
func (cache *LRUCache) record(key string) *list.Element {
	// 如果数据已存在，将元素key移动到链表头部
	if item, ok := cache.items[key]; ok {
		cache.keyList.MoveToFront(item.element)
		return item.element
	}
	// 数据不存在，直接插入到头部
	return cache.keyList.PushFront(key)
}
