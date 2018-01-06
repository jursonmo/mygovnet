package packet

import (
	"log"
	"sync"
	"sync/atomic"
)

const (
	pktBufSize = 1528
)

type PktBuf struct {
	pool     *sync.Pool
	ref      int32
	ptype    byte
	len      uint16
	vid      uint16
	outBound bool
	//userDataOff     uint16
	macHeader       int
	networkHeader   int
	transportHeader int
	buf             [pktBufSize]byte
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
	pb.len = 0
	pb.HoldPktBuf()
	return pb
}

func PutPktToPool(pb *PktBuf) {
	ref := pb.ReleasePktBuf()
	if ref == 0 {
		//fmt.Println(" put back to pool")
		pb.pool.Put(pb)
		return
	}
	if ref < 0 {
		panic("ref < 0")
	}
}

func (pb *PktBuf) HoldPktBuf() {
	//fmt.Println("hold, ref:", atomic.AddInt32(&pkt.ref, 1))
	atomic.AddInt32(&pb.ref, 1)
}

func (pb *PktBuf) ReleasePktBuf() int32 {
	return atomic.AddInt32(&pb.ref, -1)
}

func (pb *PktBuf) StoreData(data []byte) {
	pb.len = uint16(len(data))
	copy(pb.buf[:], data[:pb.len])
}

func (pb *PktBuf) LoadData() []byte {
	return pb.buf[:pb.len]
}

func (pb *PktBuf) LoadUserData() []byte {
	return pb.buf[pb.macHeader:pb.len]
}

func (pb *PktBuf) LoadBuf() []byte {
	return pb.buf[:]
}

func (pb *PktBuf) LoadRestBuf() []byte {
	return pb.buf[pb.len:]
}

func (pb *PktBuf) LoadTailData(len uint16) []byte {
	if len > pb.len {
		log.Panicf("len=%d > pb.len=%d\n", len, pb.len)
		//panic("")
	}
	return pb.buf[pb.len-len : pb.len]
}

func (pb *PktBuf) LoadTailBuf(len uint16) []byte {
	if int(len+pb.len) > pktBufSize {
		log.Panicf("len=%d+pb.len=%d > cap(pb.buf)=%d\n", len, pb.len, pktBufSize)
		//panic("")
	}
	return pb.buf[pb.len : pb.len+len]
}

func (pb *PktBuf) LoadAndUseBuf(len uint16) []byte {
	if int(len+pb.len) > pktBufSize {
		log.Panicf("len=%d+pb.len=%d > cap(pb.buf)=%d\n", len, pb.len, pktBufSize)
		//panic("")
	}
	start := pb.len
	pb.len += len
	return pb.buf[start:pb.len]

}

func (pb *PktBuf) GetDataLen() uint16 {
	return pb.len
}

func (pb *PktBuf) SetDataLen(len int) {
	if len < 0 || len > pktBufSize {
		log.Panicf("len=%d > cap(pb.buf)=%d\n", len, pktBufSize)
		//panic("")
	}
	pb.len = uint16(len)
}

func (pb *PktBuf) GetUserDataOffset() int {
	return pb.macHeader
}

func (pb *PktBuf) ExtendDataLen(extendLen uint16) {
	if extendLen < 0 || int(extendLen+pb.len) > pktBufSize {
		log.Panicf("extendLen=%d > cap(pb.buf)=%d\n", extendLen, pktBufSize)
		//panic("")
	}
	pb.len += uint16(extendLen)
}

func (pb *PktBuf) SetPktType(pt byte) {
	pb.ptype = pt
}

func (pb *PktBuf) GetPktType() byte {
	return pb.ptype
}

func (pb *PktBuf) GetPktVid() uint16 {
	return pb.vid
}

func (pb *PktBuf) SetPktVid(vid uint16) {
	pb.vid = vid
}

func (pb *PktBuf) SetUserDataOff(offset int) {
	pb.macHeader = offset
}

func (pb *PktBuf) GetUserDataLen() uint16 {
	return pb.len - uint16(pb.macHeader)
}

func (pb *PktBuf) SetNetworkHeader(offset int) {
	pb.networkHeader = pb.macHeader + offset
}

func (pb *PktBuf) LoadNetworkData() []byte {
	return pb.buf[pb.networkHeader:]
}

func (pb *PktBuf) SetTransportHeader(offset int) {
	pb.transportHeader = pb.networkHeader + offset
}

func (pb *PktBuf) LoadTransportData() []byte {
	return pb.buf[pb.transportHeader:]
}

func (pb *PktBuf) SetOutBound(b bool) {
	pb.outBound = b
}

func (pb *PktBuf) IsOutBound() bool {
	return pb.outBound
}
