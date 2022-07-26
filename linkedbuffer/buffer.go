package linkedbuffer

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/bytedance/gopkg/lang/mcache"
)

const (
	block1k = 1 * 1024
	block2k = 2 * 1024
	block4k = 4 * 1024
	block8k = 8 * 1024
)

const pagesize = block8k

// BinaryInplaceThreshold marks the minimum value of the nocopy slice length,
// which is the threshold to use copy to minimize overhead.
const BinaryInplaceThreshold = block4k

// LinkBufferCap that can be modified marks the minimum value of each node of LinkBuffer.
var LinkBufferCap = block4k

// NewLinkBuffer size defines the initial capacity, but there is no readable data.
func NewLinkBuffer(size ...int) *LinkBuffer {
	var buf = &LinkBuffer{}
	var l int
	if len(size) > 0 {
		l = size[0]
	}
	var node = newLinkBufferNode(l)
	buf.head, buf.read, buf.flush, buf.write = node, node, node, node
	return buf
}

// LinkBuffer implements ReadWriter.
type LinkBuffer struct {
	length     int32
	mallocSize int

	head  *linkBufferNode // release head
	read  *linkBufferNode // read head
	flush *linkBufferNode // malloc head
	write *linkBufferNode // malloc tail

	caches [][]byte // buf allocated by Next when cross-package, which should be freed when release
}

// Reader is a collection of operations for nocopy reads.
//
// For ease of use, it is recommended to implement Reader as a blocking interface,
// rather than simply fetching the buffer.
// For example, the return of calling Next(n) should be blocked if there are fewer than n bytes, unless timeout.
// The return value is guaranteed to meet the requirements or an error will be returned.
type Reader interface {
	// Next returns a slice containing the next n bytes from the buffer,
	// advancing the buffer as if the bytes had been returned by Read.
	//
	// If there are fewer than n bytes in the buffer, Next returns will be blocked
	// until data enough or an error occurs (such as a wait timeout).
	//
	// The slice p is only valid until the next call to the Release method.
	// Next is not globally optimal, and Skip, ReadString, ReadBinary methods
	// are recommended for specific scenarios.
	//
	// Return: len(p) must be n or 0, and p and error cannot be nil at the same time.
	Next(n int) (p []byte, err error)

	// Peek returns the next n bytes without advancing the reader.
	// Other behavior is the same as Next.
	Peek(n int) (buf []byte, err error)

	// Skip the next n bytes and advance the reader, which is
	// a faster implementation of Next when the next data is not used.
	Skip(n int) (err error)

	// Until reads until the first occurrence of delim in the input,
	// returning a slice stops with delim in the input buffer.
	// If Until encounters an error before finding a delimiter,
	// it returns all the data in the buffer and the error itself (often ErrEOF or ErrConnClosed).
	// Until returns err != nil only if line does not end in delim.
	Until(delim byte) (line []byte, err error)

	// ReadString is a faster implementation of Next when a string needs to be returned.
	// It replaces:
	//
	//  var p, err = Next(n)
	//  return string(p), err
	//
	ReadString(n int) (s string, err error)

	// ReadBinary is a faster implementation of Next when it needs to
	// return a copy of the slice that is not shared with the underlying layer.
	// It replaces:
	//
	//  var p, err = Next(n)
	//  var b = make([]byte, n)
	//  copy(b, p)
	//  return b, err
	//
	ReadBinary(n int) (p []byte, err error)

	// ReadByte is a faster implementation of Next when a byte needs to be returned.
	// It replaces:
	//
	//  var p, err = Next(1)
	//  return p[0], err
	//
	ReadByte() (b byte, err error)

	// Slice returns a new Reader containing the Next n bytes from this Reader.
	//
	// If you want to make a new Reader using the []byte returned by Next, Slice already does that,
	// and the operation is zero-copy. Besides, Slice would also Release this Reader.
	// The logic pseudocode is similar:
	//
	//  var p, err = this.Next(n)
	//  var reader = new Reader(p) // pseudocode
	//  this.Release()
	//  return reader, err
	//
	Slice(n int) (r Reader, err error)

	// Release the memory space occupied by all read slices. This method needs to be executed actively to
	// recycle the memory after confirming that the previously read data is no longer in use.
	// After invoking Release, the slices obtained by the method such as Next, Peek, Skip will
	// become an invalid address and cannot be used anymore.
	Release() (err error)

	// Len returns the total length of the readable data in the reader.
	Len() (length int)
}

// Writer is a collection of operations for nocopy writes.
//
// The usage of the design is a two-step operation, first apply for a section of memory,
// fill it and then submit. E.g:
//
//  var buf, _ = Malloc(n)
//  buf = append(buf[:0], ...)
//  Flush()
//
// Note that it is not recommended to submit self-managed buffers to Writer.
// Since the writer is processed asynchronously, if the self-managed buffer is used and recycled after submission,
// it may cause inconsistent life cycle problems. Of course this is not within the scope of the design.
type Writer interface {
	// Malloc returns a slice containing the next n bytes from the buffer,
	// which will be written after submission(e.g. Flush).
	//
	// The slice p is only valid until the next submit(e.g. Flush).
	// Therefore, please make sure that all data has been written into the slice before submission.
	Malloc(n int) (buf []byte, err error)

	// WriteString is a faster implementation of Malloc when a string needs to be written.
	// It replaces:
	//
	//  var buf, err = Malloc(len(s))
	//  n = copy(buf, s)
	//  return n, err
	//
	// The argument string s will be referenced based on the original address and will not be copied,
	// so make sure that the string s will not be changed.
	WriteString(s string) (n int, err error)

	// WriteBinary is a faster implementation of Malloc when a slice needs to be written.
	// It replaces:
	//
	//  var buf, err = Malloc(len(b))
	//  n = copy(buf, b)
	//  return n, err
	//
	// The argument slice b will be referenced based on the original address and will not be copied,
	// so make sure that the slice b will not be changed.
	WriteBinary(b []byte) (n int, err error)

	// WriteByte is a faster implementation of Malloc when a byte needs to be written.
	// It replaces:
	//
	//  var buf, _ = Malloc(1)
	//  buf[0] = b
	//
	WriteByte(b byte) (err error)

	// WriteDirect is used to insert an additional slice of data on the current write stream.
	// For example, if you plan to execute:
	//
	//  var bufA, _ = Malloc(nA)
	//  WriteBinary(b)
	//  var bufB, _ = Malloc(nB)
	//
	// It can be replaced by:
	//
	//  var buf, _ = Malloc(nA+nB)
	//  WriteDirect(b, nB)
	//
	// where buf[:nA] = bufA, buf[nA:nA+nB] = bufB.
	WriteDirect(p []byte, remainCap int) error

	// MallocAck will keep the first n malloc bytes and discard the rest.
	// The following behavior:
	//
	//  var buf, _ = Malloc(8)
	//  buf = buf[:5]
	//  MallocAck(5)
	//
	// equivalent as
	//  var buf, _ = Malloc(5)
	//
	MallocAck(n int) (err error)

	// Append the argument writer to the tail of this writer and set the argument writer to nil,
	// the operation is zero-copy, similar to p = append(p, w.p).
	Append(w Writer) (err error)

	// Flush will submit all malloc data and must confirm that the allocated bytes have been correctly assigned.
	// Its behavior is equivalent to the io.Writer hat already has parameters(slice b).
	Flush() (err error)

	// MallocLen returns the total length of the writable data that has not yet been submitted in the writer.
	MallocLen() (length int)
}

var _ Reader = &LinkBuffer{}
var _ Writer = &LinkBuffer{}

// Len implements Reader.
func (b *LinkBuffer) Len() int {
	l := atomic.LoadInt32(&b.length)
	return int(l)
}

// IsEmpty check if this LinkBuffer is empty.
func (b *LinkBuffer) IsEmpty() (ok bool) {
	return b.Len() == 0
}

// ------------------------------------------ implement zero-copy reader ------------------------------------------

// Next implements Reader.
func (b *LinkBuffer) Next(n int) (p []byte, err error) {
	if n <= 0 {
		return
	}
	// check whether enough or not.
	if b.Len() < n {
		return p, fmt.Errorf("link buffer next[%d] not enough", n)
	}
	b.recalLen(-n) // re-cal length

	// single node
	if b.isSingleNode(n) {
		return b.read.Next(n), nil
	}
	// multiple nodes
	var pIdx int
	if block1k < n && n <= mallocMax {
		p = malloc(n, n)
		b.caches = append(b.caches, p)
	} else {
		p = make([]byte, n)
	}
	var l int
	for ack := n; ack > 0; ack = ack - l {
		l = b.read.Len()
		if l >= ack {
			pIdx += copy(p[pIdx:], b.read.Next(ack))
			break
		} else if l > 0 {
			pIdx += copy(p[pIdx:], b.read.Next(l))
		}
		b.read = b.read.next
	}
	_ = pIdx
	return p, nil
}

// Peek does not have an independent lifecycle, and there is no signal to
// indicate that Peek content can be released, so Peek will not introduce mcache for now.
func (b *LinkBuffer) Peek(n int) (p []byte, err error) {
	if n <= 0 {
		return
	}
	// check whether enough or not.
	if b.Len() < n {
		return p, fmt.Errorf("link buffer peek[%d] not enough", n)
	}
	// single node
	if b.isSingleNode(n) {
		return b.read.Peek(n), nil
	}
	// multiple nodes
	var pIdx int
	if block1k < n && n <= mallocMax {
		p = malloc(n, n)
		b.caches = append(b.caches, p)
	} else {
		p = make([]byte, n)
	}
	var node = b.read
	var l int
	for ack := n; ack > 0; ack = ack - l {
		l = node.Len()
		if l >= ack {
			pIdx += copy(p[pIdx:], node.Peek(ack))
			break
		} else if l > 0 {
			pIdx += copy(p[pIdx:], node.Peek(l))
		}
		node = node.next
	}
	_ = pIdx
	return p, nil
}

// Skip implements Reader.
func (b *LinkBuffer) Skip(n int) (err error) {
	if n <= 0 {
		return
	}
	// check whether enough or not.
	if b.Len() < n {
		return fmt.Errorf("link buffer skip[%d] not enough", n)
	}
	b.recalLen(-n) // re-cal length

	var l int
	for ack := n; ack > 0; ack = ack - l {
		l = b.read.Len()
		if l >= ack {
			b.read.off += ack
			break
		}
		b.read = b.read.next
	}
	return nil
}

// Release the node that has been read.
// b.flush == nil indicates that this LinkBuffer is created by LinkBuffer.Slice
func (b *LinkBuffer) Release() (err error) {
	for b.read != b.flush && b.read.Len() == 0 {
		b.read = b.read.next
	}
	for b.head != b.read {
		node := b.head
		b.head = b.head.next
		node.Release()
	}
	for i := range b.caches {
		free(b.caches[i])
		b.caches[i] = nil
	}
	b.caches = b.caches[:0]
	return nil
}

// ReadString implements Reader.
func (b *LinkBuffer) ReadString(n int) (s string, err error) {
	if n <= 0 {
		return
	}
	// check whether enough or not.
	if b.Len() < n {
		return s, fmt.Errorf("link buffer read string[%d] not enough", n)
	}
	return unsafeSliceToString(b.readBinary(n)), nil
}

// ReadBinary implements Reader.
func (b *LinkBuffer) ReadBinary(n int) (p []byte, err error) {
	if n <= 0 {
		return
	}
	// check whether enough or not.
	if b.Len() < n {
		return p, fmt.Errorf("link buffer read binary[%d] not enough", n)
	}
	return b.readBinary(n), nil
}

// readBinary cannot use mcache, because the memory allocated by readBinary will not be recycled.
func (b *LinkBuffer) readBinary(n int) (p []byte) {
	b.recalLen(-n) // re-cal length

	// single node
	p = make([]byte, n)
	if b.isSingleNode(n) {
		copy(p, b.read.Next(n))
		return p
	}
	// multiple nodes
	var pIdx int
	var l int
	for ack := n; ack > 0; ack = ack - l {
		l = b.read.Len()
		if l >= ack {
			pIdx += copy(p[pIdx:], b.read.Next(ack))
			break
		} else if l > 0 {
			pIdx += copy(p[pIdx:], b.read.Next(l))
		}
		b.read = b.read.next
	}
	_ = pIdx
	return p
}

// ReadByte implements Reader.
func (b *LinkBuffer) ReadByte() (p byte, err error) {
	// check whether enough or not.
	if b.Len() < 1 {
		return p, errors.New("link buffer read byte is empty")
	}
	b.recalLen(-1) // re-cal length
	for {
		if b.read.Len() >= 1 {
			return b.read.Next(1)[0], nil
		}
		b.read = b.read.next
	}
}

// Until returns a slice ends with the delim in the buffer.
func (b *LinkBuffer) Until(delim byte) (line []byte, err error) {
	n := b.indexByte(delim, 0)
	if n < 0 {
		return nil, fmt.Errorf("link buffer read slice cannot find: '%b'", delim)
	}
	return b.Next(n + 1)
}

// Slice returns a new LinkBuffer, which is a zero-copy slice of this LinkBuffer,
// and only holds the ability of Reader.
//
// Slice will automatically execute a Release.
func (b *LinkBuffer) Slice(n int) (r Reader, err error) {
	if n <= 0 {
		return NewLinkBuffer(0), nil
	}
	// check whether enough or not.
	if b.Len() < n {
		return r, fmt.Errorf("link buffer readv[%d] not enough", n)
	}
	b.recalLen(-n) // re-cal length

	// just use for range
	p := &LinkBuffer{
		length: int32(n),
	}
	defer func() {
		// set to read-only
		p.flush = p.flush.next
		p.write = p.flush
	}()

	// single node
	if b.isSingleNode(n) {
		node := b.read.Refer(n)
		p.head, p.read, p.flush = node, node, node
		return p, nil
	}
	// multiple nodes
	var l = b.read.Len()
	node := b.read.Refer(l)
	b.read = b.read.next

	p.head, p.read, p.flush = node, node, node
	for ack := n - l; ack > 0; ack = ack - l {
		l = b.read.Len()
		if l >= ack {
			p.flush.next = b.read.Refer(ack)
			p.flush = p.flush.next
			break
		} else if l > 0 {
			p.flush.next = b.read.Refer(l)
			p.flush = p.flush.next
		}
		b.read = b.read.next
	}
	return p, b.Release()
}

// ------------------------------------------ implement zero-copy writer ------------------------------------------

// Malloc pre-allocates memory, which is not readable, and becomes readable data after submission(e.g. Flush).
func (b *LinkBuffer) Malloc(n int) (buf []byte, err error) {
	if n <= 0 {
		return
	}
	b.mallocSize += n
	b.growth(n)
	return b.write.Malloc(n), nil
}

// MallocLen implements Writer.
func (b *LinkBuffer) MallocLen() (length int) {
	return b.mallocSize
}

// MallocAck will keep the first n malloc bytes and discard the rest.
func (b *LinkBuffer) MallocAck(n int) (err error) {
	if n < 0 {
		return fmt.Errorf("link buffer malloc ack[%d] invalid", n)
	}
	b.mallocSize = n
	b.write = b.flush

	var l int
	for ack := n; ack > 0; ack = ack - l {
		l = b.write.malloc - len(b.write.buf)
		if l >= ack {
			b.write.malloc = ack + len(b.write.buf)
			break
		}
		b.write = b.write.next
	}
	// discard the rest
	for node := b.write.next; node != nil; node = node.next {
		node.off, node.malloc, node.refer, node.buf = 0, 0, 1, node.buf[:0]
	}
	return nil
}

// Flush will submit all malloc data and must confirm that the allocated bytes have been correctly assigned.
func (b *LinkBuffer) Flush() (err error) {
	b.mallocSize = 0
	// FIXME: The tail node must not be larger than 8KB to prevent Out Of Memory.
	if cap(b.write.buf) > pagesize {
		b.write.next = newLinkBufferNode(0)
		b.write = b.write.next
	}
	var n int
	for node := b.flush; node != b.write.next; node = node.next {
		delta := node.malloc - len(node.buf)
		if delta > 0 {
			n += delta
			node.buf = node.buf[:node.malloc]
		}
	}
	b.flush = b.write
	// re-cal length
	b.recalLen(n)
	return nil
}

// Append implements Writer.
func (b *LinkBuffer) Append(w Writer) (err error) {
	var buf, ok = w.(*LinkBuffer)
	if !ok {
		return errors.New("unsupported writer which is not LinkBuffer")
	}
	return b.WriteBuffer(buf)
}

// WriteBuffer will not submit(e.g. Flush) data to ensure normal use of MallocLen.
// you must actively submit before read the data.
// The argument buf can't be used after calling WriteBuffer. (set it to nil)
func (b *LinkBuffer) WriteBuffer(buf *LinkBuffer) (err error) {
	if buf == nil {
		return
	}
	bufLen, bufMallocLen := buf.Len(), buf.MallocLen()
	if bufLen+bufMallocLen <= 0 {
		return nil
	}
	b.write.next = buf.read
	b.write = buf.write

	// close buf, prevents reuse.
	for buf.head != buf.read {
		nd := buf.head
		buf.head = buf.head.next
		nd.Release()
	}
	for buf.write = buf.write.next; buf.write != nil; {
		nd := buf.write
		buf.write = buf.write.next
		nd.Release()
	}
	buf.length, buf.mallocSize, buf.head, buf.read, buf.flush, buf.write = 0, 0, nil, nil, nil, nil

	// DON'T MODIFY THE CODE BELOW UNLESS YOU KNOW WHAT YOU ARE DOING !
	//
	// You may encounter a chain of bugs and not be able to
	// find out within a week that they are caused by modifications here.
	//
	// After release buf, continue to adjust b.
	b.write.next = nil
	if bufLen > 0 {
		b.recalLen(bufLen)
	}
	b.mallocSize += bufMallocLen
	return nil
}

// WriteString implements Writer.
func (b *LinkBuffer) WriteString(s string) (n int, err error) {
	if len(s) == 0 {
		return
	}
	buf := unsafeStringToSlice(s)
	return b.WriteBinary(buf)
}

// WriteBinary implements Writer.
func (b *LinkBuffer) WriteBinary(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return
	}
	b.mallocSize += n

	// TODO: Verify that all nocopy is possible under mcache.
	if n > BinaryInplaceThreshold {
		// expand buffer directly with nocopy
		b.write.next = newLinkBufferNode(0)
		b.write = b.write.next
		b.write.buf, b.write.malloc = p[:0], n
		return n, nil
	}
	// here will copy
	b.growth(n)
	malloc := b.write.malloc
	b.write.malloc += n
	return copy(b.write.buf[malloc:b.write.malloc], p), nil
}

// WriteDirect cannot be mixed with WriteString or WriteBinary functions.
func (b *LinkBuffer) WriteDirect(p []byte, remainLen int) error {
	n := len(p)
	if n == 0 || remainLen < 0 {
		return nil
	}
	// find origin
	origin := b.flush
	malloc := b.mallocSize - remainLen // calculate the remaining malloc length
	for t := origin.malloc - len(origin.buf); t <= malloc; t = origin.malloc - len(origin.buf) {
		malloc -= t
		origin = origin.next
	}
	// Add the buf length of the original node
	malloc += len(origin.buf)

	// Create dataNode and newNode and insert them into the chain
	dataNode := newLinkBufferNode(0)
	dataNode.buf, dataNode.malloc = p[:0], n

	newNode := newLinkBufferNode(0)
	newNode.off = malloc
	newNode.buf = origin.buf[:malloc]
	newNode.malloc = origin.malloc
	newNode.readonly = false
	origin.malloc = malloc
	origin.readonly = true

	// link nodes
	dataNode.next = newNode
	newNode.next = origin.next
	origin.next = dataNode

	// adjust b.write
	for b.write.next != nil {
		b.write = b.write.next
	}

	b.mallocSize += n
	return nil
}

// WriteByte implements Writer.
func (b *LinkBuffer) WriteByte(p byte) (err error) {
	dst, err := b.Malloc(1)
	if len(dst) == 1 {
		dst[0] = p
	}
	return err
}

// Close will recycle all buffer.
func (b *LinkBuffer) Close() (err error) {
	atomic.StoreInt32(&b.length, 0)
	b.mallocSize = 0
	// just release all
	b.Release()
	for node := b.head; node != nil; {
		nd := node
		node = node.next
		nd.Release()
	}
	b.head, b.read, b.flush, b.write = nil, nil, nil, nil
	return nil
}

// ------------------------------------------ implement connection interface ------------------------------------------

// Bytes returns all the readable bytes of this LinkBuffer.
func (b *LinkBuffer) Bytes() []byte {
	node, flush := b.read, b.flush
	if node == flush {
		return node.buf[node.off:]
	}
	n := 0
	p := make([]byte, b.Len())
	for ; node != flush; node = node.next {
		if node.Len() > 0 {
			n += copy(p[n:], node.buf[node.off:])
		}
	}
	n += copy(p[n:], flush.buf[flush.off:])
	return p[:n]
}

// GetBytes will read and fill the slice p as much as possible.
func (b *LinkBuffer) GetBytes(p [][]byte) (vs [][]byte) {
	node, flush := b.read, b.flush
	var i int
	for i = 0; node != flush && i < len(p); node = node.next {
		if node.Len() > 0 {
			p[i] = node.buf[node.off:]
			i++
		}
	}
	if i < len(p) {
		p[i] = flush.buf[flush.off:]
		i++
	}
	return p[:i]
}

// book will grow and malloc buffer to hold data.
//
// bookSize: The size of data that can be read at once.
// maxSize: The maximum size of data between two Release(). In some cases, this can
// 	guarantee all data allocated in one node to reduce copy.
func (b *LinkBuffer) book(bookSize, maxSize int) (p []byte) {
	l := cap(b.write.buf) - b.write.malloc
	// grow linkBuffer
	if l == 0 {
		l = maxSize
		b.write.next = newLinkBufferNode(maxSize)
		b.write = b.write.next
	}
	if l > bookSize {
		l = bookSize
	}
	return b.write.Malloc(l)
}

// bookAck will ack the first n malloc bytes and discard the rest.
//
// length: The size of data in inputBuffer. It is used to calculate the maxSize
func (b *LinkBuffer) bookAck(n int) (length int, err error) {
	b.write.malloc = n + len(b.write.buf)
	b.write.buf = b.write.buf[:b.write.malloc]
	b.flush = b.write

	// re-cal length
	length = b.recalLen(n)
	return length, nil
}

// calcMaxSize will calculate the data size between two Release()
func (b *LinkBuffer) calcMaxSize() (sum int) {
	for node := b.head; node != b.read; node = node.next {
		sum += len(node.buf)
	}
	sum += len(b.read.buf)
	return sum
}

// indexByte returns the index of the first instance of c in buffer, or -1 if c is not present in buffer.
func (b *LinkBuffer) indexByte(c byte, skip int) int {
	size := b.Len()
	if skip >= size {
		return -1
	}
	var unread, n, l int
	node := b.read
	for unread = size; unread > 0; unread -= n {
		l = node.Len()
		if l >= unread { // last node
			n = unread
		} else { // read full node
			n = l
		}

		// skip current node
		if skip >= n {
			skip -= n
			node = node.next
			continue
		}
		i := bytes.IndexByte(node.Peek(n)[skip:], c)
		if i >= 0 {
			return (size - unread) + skip + i // past_read + skip_read + index
		}
		skip = 0 // no skip bytes
		node = node.next
	}
	return -1
}

// resetTail will reset tail node or add an empty tail node to
// guarantee the tail node is not larger than 8KB
func (b *LinkBuffer) resetTail(maxSize int) {
	// FIXME: The tail node must not be larger than 8KB to prevent Out Of Memory.
	if maxSize <= pagesize {
		b.write.Reset()
		return
	}

	// set nil tail
	b.write.next = newLinkBufferNode(0)
	b.write = b.write.next
	b.flush = b.write
	return
}

// recalLen re-calculate the length
func (b *LinkBuffer) recalLen(delta int) (length int) {
	return int(atomic.AddInt32(&b.length, int32(delta)))
}

// ------------------------------------------ implement link node ------------------------------------------

// newLinkBufferNode create or reuse linkBufferNode.
// Nodes with size <= 0 are marked as readonly, which means the node.buf is not allocated by this mcache.
func newLinkBufferNode(size int) *linkBufferNode {
	var node = linkedPool.Get().(*linkBufferNode)
	// reset node offset
	node.off, node.malloc, node.refer, node.readonly = 0, 0, 1, false
	if size <= 0 {
		node.readonly = true
		return node
	}
	if size < LinkBufferCap {
		size = LinkBufferCap
	}
	node.buf = malloc(0, size)
	return node
}

var linkedPool = sync.Pool{
	New: func() interface{} {
		return &linkBufferNode{
			refer: 1, // 自带 1 引用
		}
	},
}

type linkBufferNode struct {
	buf      []byte          // buffer
	off      int             // read-offset
	malloc   int             // write-offset
	refer    int32           // reference count
	readonly bool            // read-only node, introduced by Refer, WriteString, WriteBinary, etc., default false
	origin   *linkBufferNode // the root node of the extends
	next     *linkBufferNode // the next node of the linked buffer
}

func (node *linkBufferNode) Len() (l int) {
	return len(node.buf) - node.off
}

func (node *linkBufferNode) IsEmpty() (ok bool) {
	return node.off == len(node.buf)
}

func (node *linkBufferNode) Reset() {
	if node.origin != nil || atomic.LoadInt32(&node.refer) != 1 {
		return
	}
	node.off, node.malloc = 0, 0
	node.buf = node.buf[:0]
	return
}

func (node *linkBufferNode) Next(n int) (p []byte) {
	off := node.off
	node.off += n
	return node.buf[off:node.off]
}

func (node *linkBufferNode) Peek(n int) (p []byte) {
	return node.buf[node.off : node.off+n]
}

func (node *linkBufferNode) Malloc(n int) (buf []byte) {
	malloc := node.malloc
	node.malloc += n
	return node.buf[malloc:node.malloc]
}

// Refer holds a reference count at the same time as Next, and releases the real buffer after Release.
// The node obtained by Refer is read-only.
func (node *linkBufferNode) Refer(n int) (p *linkBufferNode) {
	p = newLinkBufferNode(0)
	p.buf = node.Next(n)

	if node.origin != nil {
		p.origin = node.origin
	} else {
		p.origin = node
	}
	atomic.AddInt32(&p.origin.refer, 1)
	return p
}

// Release consists of two parts:
// 1. reduce the reference count of itself and origin.
// 2. recycle the buf when the reference count is 0.
func (node *linkBufferNode) Release() (err error) {
	if node.origin != nil {
		node.origin.Release()
	}
	// release self
	if atomic.AddInt32(&node.refer, -1) == 0 {
		// readonly nodes cannot recycle node.buf, other node.buf are recycled to mcache.
		if !node.readonly {
			free(node.buf)
		}
		node.buf, node.origin, node.next = nil, nil, nil
		linkedPool.Put(node)
	}
	return nil
}

// ------------------------------------------ private function ------------------------------------------

// growth directly create the next node, when b.write is not enough.
func (b *LinkBuffer) growth(n int) {
	if n <= 0 {
		return
	}
	// Must skip read-only node.
	for b.write.readonly || cap(b.write.buf)-b.write.malloc < n {
		if b.write.next == nil {
			b.write.next = newLinkBufferNode(n)
			b.write = b.write.next
			return
		}
		b.write = b.write.next
	}
}

// isSingleNode determines whether reading needs to cross nodes.
// Must require b.Len() > 0
func (b *LinkBuffer) isSingleNode(readN int) (single bool) {
	if readN <= 0 {
		return true
	}
	l := b.read.Len()
	for l == 0 {
		b.read = b.read.next
		l = b.read.Len()
	}
	return l >= readN
}

// zero-copy slice convert to string
func unsafeSliceToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// zero-copy slice convert to string
func unsafeStringToSlice(s string) (b []byte) {
	p := unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&s)).Data)
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = uintptr(p)
	hdr.Cap = len(s)
	hdr.Len = len(s)
	return b
}

// mallocMax is 8MB
const mallocMax = block8k * block1k

// malloc limits the cap of the buffer from mcache.
func malloc(size, capacity int) []byte {
	if capacity > mallocMax {
		return make([]byte, size, capacity)
	}
	return mcache.Malloc(size, capacity)
}

// free limits the cap of the buffer from mcache.
func free(buf []byte) {
	if cap(buf) > mallocMax {
		return
	}
	mcache.Free(buf)
}
