package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type PktBuf struct {
	sync.Mutex
	rb  *RingBuffer
	id  int
	ref int32
	len uint16
	buf [1024]byte
}

type RingBufferHeader struct {
	r, w, count uint16
	//size uint32
}

type RingBuffer struct {
	getLock sync.Mutex
	putLock sync.Mutex
	hdr     RingBufferHeader
	buf     []*PktBuf //存储指针
}

const (
	DataBufferNum = 4
)

//var DefaultRB *RingBuffer
//var RingBufferNum = DataBufferNum + 1

func init() {
	//DefaultRB = CreateRingBuffer(DataBufferNum)
}
func main() {
	rb := CreateRingBuffer(DataBufferNum)
	fmt.Println(rb)

	fmt.Printf("rb.buf len=%d\n", len(rb.buf))
	show(rb)

	for j := 0; j < DataBufferNum; j++ {
		pb := GetBuf(rb)
		if pb == nil {
			fmt.Println("pb == nil")
			return
		}
		for i := 0; i < DataBufferNum+j; i++ {
			HoldPktBuf(pb)
			handleBuf(pb)
		}
		PutBuf(pb)
	}
	show(rb)
	for j := 0; j < DataBufferNum; j++ {
		pb := GetBuf(rb)
		if pb == nil {
			fmt.Println("pb == nil")
			return
		}

		for i := 0; i < DataBufferNum+j; i++ {
			HoldPktBuf(pb)
			handleBuf(pb)
		}
		PutBuf(pb)
	}
	show(rb)
}

func show(rb *RingBuffer) {
	fmt.Println("===========show rb==========")
	for i, pb := range rb.buf {
		if pb != nil {
			fmt.Printf("i=%d, pb.id=%d , pb.ref=%d, pb.len=%d\n", i, pb.id, pb.ref, pb.len)
		}
	}
}
func CreateRingBuffer(n int) *RingBuffer {
	mp := make([]PktBuf, n)
	rb := NewRingBuffer(n + 1)
	for i := 0; i < n; i++ {
		mp[i].id = i
		if RBPutBuf(rb, &mp[i]) == false {
			fmt.Printf("i =%d, RBPutBuf error\n", i)
			panic("RBPutBuf error")
		}
	}
	return rb
}
func NewRingBuffer(size int) *RingBuffer {
	rb := &RingBuffer{
		buf: make([]*PktBuf, size),
	}
	rb.hdr.count = uint16(size)
	return rb
}

func RBPutBuf(rb *RingBuffer, pb *PktBuf) bool {
	rb.putLock.Lock()
	pb.rb = rb
	idx := (rb.hdr.w + 1) % rb.hdr.count
	//fmt.Printf("r=%d,w=%d,idx=%d\n", rb.hdr.r, rb.hdr.w, idx)
	if idx != rb.hdr.r {
		rb.buf[rb.hdr.w] = pb
		rb.hdr.w = idx
		rb.putLock.Unlock()
		return true
	}
	rb.putLock.Unlock()
	return false
}

func RBGetBuf(rb *RingBuffer) *PktBuf {
	rb.getLock.Lock()
	idx := rb.hdr.r
	if idx != rb.hdr.w {
		rb.hdr.r = (rb.hdr.r + 1) % rb.hdr.count
		rb.getLock.Unlock()
		return rb.buf[idx]
	}
	rb.getLock.Unlock()
	return nil
}

func GetBuf(rb *RingBuffer) *PktBuf {
	pb := RBGetBuf(rb)
	if pb == nil {
		return nil
	}
	HoldPktBuf(pb)
	return pb
}

func PutBuf(pb *PktBuf) bool {
	if pb == nil {
		panic("pb is nil")
		//return false
	}
	if pb.rb == nil {
		panic("pb.rb is nil")
		//return RBPutBuf(DefaultRB, pb)
	}
	ref := releasePktBuf(pb)
	if ref == 0 {
		//fmt.Println("PutBuf")
		return RBPutBuf(pb.rb, pb)
	}
	if ref < 0 {
		panic("ref < 0")
	}
	return false
}

func HoldPktBuf(pb *PktBuf) {
	//fmt.Println("hold, ref:", atomic.AddInt32(&pkt.ref, 1))
	atomic.AddInt32(&pb.ref, 1)
}

func releasePktBuf(pb *PktBuf) int32 {
	return atomic.AddInt32(&pb.ref, -1)
}

func (pb *PktBuf) StoreData(data []byte) {
	pb.len = uint16(len(data))
	copy(pb.buf[:], data[:pb.len])
}

func (pb *PktBuf) LoadData() []byte {
	return pb.buf[:pb.len]
}

func (pb *PktBuf) AppendData(data []byte) error {
	if int(pb.len)+len(data) > cap(pb.buf) {
		return fmt.Errorf("out of buf store range %d", cap(pb.buf))
	}
	copy(pb.buf[pb.len:], data)
	pb.len += uint16(len(data))
	return nil
}

func handleBuf(pb *PktBuf) {
	pb.Lock()
	pb.len++
	pb.Unlock()

	PutBuf(pb)
}

/*
type test struct {
	hdr RingBufferHeader
	buf	[8]byte
}
func main(){
	fmt.Println("begin")
	data := make([]byte,0, 32)
	buf := bytes.NewBuffer(data)
	for i:=0 ; i< 32; i++{
		buf.WriteByte(byte(i))
	}
	fmt.Println(buf.Bytes())
	d := buf.Bytes()
	rb := (*test)(unsafe.Pointer(&d[0]))
	aa := (*[4]byte)(unsafe.Pointer(&rb.buf[0]))
	fmt.Println(aa)
	fmt.Println(*aa)
}
*/
// begin
// [0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31]
// &[12 13 14 15]
// [12 13 14 15]
