/*
Copyright (c) 2016, Theodore Butler
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// Package bitmap contains bitmaps of length 32 and 64 for tracking bool
// values without the need for arrays or hashing.
package bitarray

// Bitmap32 tracks 32 bool values within a uint32
type Bitmap32 uint32

// SetBit returns a Bitmap32 with the bit at the given position set to 1
// 将指定位置设为1
func (b Bitmap32) SetBit(pos uint) Bitmap32 {
	// 精妙
	// 1.根据位移运算，得到一个指定位置为1的数字，比如pos为4,则得到的是 0001 0000
	// 2.通过按位异或运算将值设置上去
	return b | (1 << pos)
}

// ClearBit returns a Bitmap32 with the bit at the given position set to 0
// 将指定位置设为0
func (b Bitmap32) ClearBit(pos uint) Bitmap32 {
	// 1.位移运算得到指定位置为1的数字，如：0001 0000
	// 2.取反,如：1110 1111
	// 3.按位与,如b为 0011 0000,最终结果为: 0010 0000
	return b & ^(1 << pos)
}

// GetBit returns true if the bit at the given position in the Bitmap32 is 1
// 查看指定位置的值是否为1
func (b Bitmap32) GetBit(pos uint) bool {
	// 1.位移运算得到指定位置为1的数字，如：0001 0000
	// 2.按位与,如b为 0011 0000,得到结果为：0001 0000
	// 3.只要结果不为0,则证明该位上的值为1
	return (b & (1 << pos)) != 0
}

// PopCount returns the amount of bits set to 1 in the Bitmap32
// 查看值位1的数量
func (b Bitmap32) PopCount() int {
	// http://graphics.stanford.edu/~seander/bithacks.html#CountBitsSetParallel
	// 将b的二进制分为16等份，计算每个部分含1的个数。
	// 等同于：b = (b & 0x55555555) + ((b >> 1) & 0x55555555);
	// 示例：10 11 10 01 00 11 10 11 00 01 10 01 10 00 01 00
	// 经过运算：01 10 01 01 00 10 01 10 00 01 01 01 01 00 01 00
	b -= (b >> 1) & 0x55555555

	// 后面的计算就是一步步地将每部分的值加起来
	// 第1部分+第2部分，第3部分+第4部分，...第15部分+第16部分
	// 经过运算：0011  0010  0010  0011  0001  0010  0001  0001
	b = (b>>2)&0x33333333 + b&0x33333333
	// 以下是精简后的运算，等同于上述操作
	b = ((b + (b>>4)&0xF0F0F0F) * 0x1010101)
	return int(byte(b >> 24))
}

// Bitmap64 tracks 64 bool values within a uint64
type Bitmap64 uint64

// SetBit returns a Bitmap64 with the bit at the given position set to 1
func (b Bitmap64) SetBit(pos uint) Bitmap64 {
	return b | (1 << pos)
}

// ClearBit returns a Bitmap64 with the bit at the given position set to 0
func (b Bitmap64) ClearBit(pos uint) Bitmap64 {
	return b & ^(1 << pos)
}

// GetBit returns true if the bit at the given position in the Bitmap64 is 1
func (b Bitmap64) GetBit(pos uint) bool {
	return (b & (1 << pos)) != 0
}

// PopCount returns the amount of bits set to 1 in the Bitmap64
func (b Bitmap64) PopCount() int {
	// http://graphics.stanford.edu/~seander/bithacks.html#CountBitsSetParallel
	b -= (b >> 1) & 0x5555555555555555
	b = (b>>2)&0x3333333333333333 + b&0x3333333333333333
	b += b >> 4
	b &= 0x0f0f0f0f0f0f0f0f
	b *= 0x0101010101010101
	return int(byte(b >> 56))
}
