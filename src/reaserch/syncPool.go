package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type PktBuf struct {
	pool *sync.Pool
	sync.Mutex
	ref int32
	len uint16
	buf [1518]byte
}

func NewPktBufPool() *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return &PktBuf{}
		},
	}
}

func GetPktFromPool(sp *sync.Pool) *PktBuf {
	pb, ok := sp.Get().(*PktBuf)
	if !ok {
		panic("type not *PKT")
	}
	if pb.pool == nil { //new alloc object
		pb.pool = sp
	}
	HoldPktBuf(pb)
	return pb
}

func PutPktToPool(pb *PktBuf) {
	ref := releasePktBuf(pb)
	if ref == 0 {
		//fmt.Println(" put back to pool")
		pb.pool.Put(pb)
		return
	}
	if ref < 0 {
		panic("ref < 0")
	}
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

//testing =========================
func handlePktBuf(pb *PktBuf, i int) {
	//do something with pkt.buf[:pkt.len], write(pkt.buf[:pkt.len])

	//show data and change data, just for testing
	pb.Lock()
	pb.AppendData([]byte(fmt.Sprintf("%d", i)))
	fmt.Println(i, string(pb.LoadData()))
	pb.Unlock()

	//put pkt buff back to pool when handle over
	PutPktToPool(pb)
}
func main() {
	sp := NewPktBufPool()
	pk := GetPktFromPool(sp)

	pk.StoreData([]byte("test sync.pool"))

	for i := 0; i < 2; i++ {
		HoldPktBuf(pk)
		go handlePktBuf(pk, i)
	}
	PutPktToPool(pk)

	for i := 0; i < 5; i++ {
		time.Sleep(time.Millisecond)
		pk1 := GetPktFromPool(sp) //maybe pk have been runtime GC
		fmt.Println(i, string(pk1.LoadData()))
	}
}
