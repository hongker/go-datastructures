package hashmap

const loadFactor = 0.65 // 负载因子，控制扩容的触发

// roundUp 返回邻近的2的N次方的数
func roundUp(v uint64) uint64 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}

type HashMap struct {
	count   uint64  //总数量
	buckets buckets //桶个数
}

func New(hint uint64) *HashMap {
	if hint == 0 {
		hint = 16
	}
	return &HashMap{
		count:   0,
		buckets: make(buckets, roundUp(hint)),
	}
}

// Get 查询，返回值以及是否存在
func (hm *HashMap) Get(key uint64) (interface{}, bool) {
	idx := hm.buckets.find(key)
	if hm.buckets[idx] == nil {
		return nil, false
	}
	return hm.buckets[idx].val, true
}

// Set 赋值
func (hm *HashMap) Set(key uint64, val interface{}) {
	// 判断到负载因子大于指定阈值时，就需要扩容并重新分配
	if float64(hm.count+1)/float64(len(hm.buckets)) > loadFactor {
		hm.rebuild()
	}
	hm.buckets.set(&item{key: key, val: val})
	hm.count++
}

// rebuild 扩容并重新分配
func (hm *HashMap) rebuild() {
	// 利用一个扩容的临时buckets，重新赋值
	temp := make(buckets, roundUp(uint64(len(hm.buckets)+1)))
	for _, item := range hm.buckets {
		if item == nil {
			continue
		}
		temp.set(item)
	}
	hm.buckets = temp
}

// find 查询key所在位置
func (buckets buckets) find(key uint64) uint64 {
	idx := buckets.hashFor(hashcode(key))
	// 利用开放地址法处理hash冲突后的寻址
	for buckets[idx] != nil && buckets[idx].key != key {
		// 当冲突后，通过线性探测，依次遍历找到空闲位置
		idx = (idx + 1) & (uint64(len(buckets)) - 1)
	}

	return idx
}

func (buckets buckets) set(item *item) {
	idx := buckets.find(item.key)
	if buckets[idx] == nil { // 如果为空闲位置，则直接插入
		buckets[idx] = item
		return
	}

	// 如果key已存在，则覆盖val
	buckets[idx].val = item.val
}

// hashFor 通过位运算确定hashcode对应的位置
func (buckets buckets) hashFor(hashcode uint64) uint64 {
	return hashcode & (uint64(len(buckets)) - 1)
}

type item struct {
	key uint64
	val interface{}
}

type buckets []*item

func hashcode(key uint64) uint64 {
	key ^= key >> 33
	key *= 0xff51afd7ed558ccd
	key ^= key >> 33
	key *= 0xc4ceb9fe1a85ec53
	key ^= key >> 33
	return key
}
