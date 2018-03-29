package nat

import (
	"ilist"
	"log"
	"packet"
	"sync"
	"time"
)

const (
	FRAG_LAST_IN = 1 << iota
	FRAG_COMPLETE
)

type fragTable struct {
	sync.RWMutex
	fragQueueMap map[fragInfo]*fragQueue
}

type fragInfo struct {
	proto uint8
	id    uint16
	sip   uint32
	dip   uint32
}

type fragQueue struct {
	list       ilist.List
	qLen       uint16
	meat       uint16
	flag       uint8
	createTime time.Time
}

type fragment struct {
	ilist.Entry
	offset uint16
	len    uint16
	last   uint16
	more   bool
	pkb    *packet.PktBuf
}

func newfragTable() *fragTable {
	return &fragTable{
		fragQueueMap: make(map[fragInfo]*fragQueue),
	}
}

func newfragQueue() *fragQueue {
	fq := new(fragQueue)
	fq.list.Reset()
	fq.createTime = time.Now()
	return fq
}

func createFragment(offset, len uint16, more bool, pkb *packet.PktBuf) *fragment {
	return &fragment{offset: offset, len: len, last: offset + len, more: more, pkb: pkb}
}

func (fq *fragQueue) ProcessFragment(offset, len uint16, more bool, pkb *packet.PktBuf) (ok, done bool) {
	if fq.flag&FRAG_COMPLETE != 0 {
		//panic ? have completed
		return false, false
	}

	nfrag := createFragment(offset, len, more, pkb)
	log.Printf("-------nfrag=%#v----------\n", nfrag)
	log.Printf("-------nfrag.offset=%d, len=%d, last=%d----------\n", nfrag.offset, nfrag.len, nfrag.last)

	if fq.list.Empty() {
		fq.list.PushFront(nfrag)
		fq.meat += nfrag.len
		fq.qLen = nfrag.last
		return true, false
	}

	if !more {
		fq.flag |= FRAG_LAST_IN
		if tail := fq.list.Back(); tail != nil {
			if tail.(*fragment).last > nfrag.offset {
				log.Panicf("over overlapped, tail.last=%d, nfrag.offset=%d\n", tail.(*fragment).last, nfrag.offset)
			}
		}
		fq.list.PushBack(nfrag)
		fq.meat += nfrag.len
		fq.qLen = nfrag.last
		if fq.qLen == fq.meat {
			fq.flag |= FRAG_COMPLETE
			return true, true
		}
		return true, false
	}

	for e := fq.list.Back(); e != nil; e = e.Prev() {
		frag, _ := e.(*fragment)
		if nfrag.offset >= frag.last {
			if next := e.Next(); next != nil {
				if nfrag.last > next.(*fragment).offset {
					log.Panicf("over overlapped, nfrag.last=%d, next.offset=%d\n", nfrag.last, next.(*fragment).offset)
					return false, false
				}
			}
			fq.list.InsertAfter(e, nfrag)
			if fq.qLen < nfrag.last {
				fq.qLen = nfrag.last
			}
			fq.meat += len
			if fq.flag&FRAG_LAST_IN != 0 && fq.qLen == fq.meat {
				fq.flag |= FRAG_COMPLETE
				return true, true
			}
			return true, false
		}
	}
	return false, false
}

func (fq *fragQueue) completed() (first_pkb *packet.PktBuf) {
	if fq.flag&FRAG_COMPLETE == 0 {
		log.Panicf("frag uncompleted")
	}
	log.Println("completed")
	var prev, npkb *packet.PktBuf
	first := true
	for e := fq.list.Front(); e != nil; e = e.Next() {
		npkb = e.(*fragment).pkb
		if first {
			first_pkb = npkb
			first = false
		} else {
			prev.SetNext(npkb)
		}
		prev = npkb
	}
	return
}

func (nct *netConntrack) fragQueueFind(fi fragInfo) (fq *fragQueue, ok bool) {
	nct.fragTables.RLock()
	fq, ok = nct.fragTables.fragQueueMap[fi]
	nct.fragTables.RUnlock()
	return
}

func (nct *netConntrack) fragQueueAdd(fi fragInfo, fq *fragQueue) {
	nct.fragTables.Lock()
	nct.fragTables.fragQueueMap[fi] = fq
	nct.fragTables.Unlock()
}

func (nct *netConntrack) fragQueueDel(fi fragInfo) {
	nct.fragTables.Lock()
	delete(nct.fragTables.fragQueueMap, fi)
	nct.fragTables.Unlock()
}
