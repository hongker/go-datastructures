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

package bitarray

import (
	"fmt"
	"unsafe"
)

// block defines how we split apart the bit array. This also determines the size
// of s. This can be changed to any unsigned integer type: uint8, uint16,
// uint32, and so on.
// block 定义每个分块的长度，也可以是uint8
type block uint64

// s denotes the size of any element in the block array.
// For a block of uint64, s will be equal to 64
// For a block of uint32, s will be equal to 32
// and so on...
// 根据block读取最大存储长度
const s = uint64(unsafe.Sizeof(block(0)) * 8)

// maximumBlock represents a block of all 1s and is used in the constructors.
// 根据block的长度得到二进制为1的最大数
const maximumBlock = block(0) | ^block(0)

// toNums 读取数据
func (b block) toNums(offset uint64, nums *[]uint64) {
	for i := uint64(0); i < s; i++ {
		// 遍历之后，通过按位与运算，将位数不位0的数据取出
		if b&block(1<<i) > 0 {
			*nums = append(*nums, i+offset)
		}
	}
}

// findLeftPosition 找到最左侧为1的位置
func (b block) findLeftPosition() uint64 {
	for i := s - 1; i < s; i-- {
		test := block(1 << i)
		if b&test == test {
			return i
		}
	}

	return s
}

// findRightPosition 找到最右侧位1的位置
func (b block) findRightPosition() uint64 {
	for i := uint64(0); i < s; i++ {
		test := block(1 << i)
		if b&test == test {
			return i
		}
	}

	return s
}

// insert 将指定位置设为1
func (b block) insert(position uint64) block {
	return b | block(1<<position)
}

func (b block) remove(position uint64) block {
	return b & ^block(1<<position)
}

// or 将两个bitmap进行异或运算
func (b block) or(other block) block {
	return b | other
}

func (b block) and(other block) block {
	return b & other
}

func (b block) nand(other block) block {
	return b &^ other
}

func (b block) get(position uint64) bool {
	return b&block(1<<position) != 0
}

func (b block) equals(other block) bool {
	return b == other
}

// intersects 判断是否相交
func (b block) intersects(other block) bool {
	return b&other == other
}

func (b block) String() string {
	return fmt.Sprintf(fmt.Sprintf("%%0%db", s), uint64(b))
}
