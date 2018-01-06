package fdb

import (
	"packet"
	"sync"
	"sync/atomic"
)

const (
	MAXPORT = 128
)

type portIO interface {
	PutPktToChan(pkt *packet.PktBuf)
	String() string
}

type portPools struct {
	sync.Mutex
	portIdPool [MAXPORT]int
	allocNum   int32
}

//var portPool portPools

type portMaps struct {
	sync.RWMutex
	ports map[int]portIO
}

//var portMap map[int]portIO
//var portMapLock sync.RWMutex

func init() {
	//portMap = make(map[int]portIO)
	//initPortPool()
}

func (f *FDB) initPortPool() {
	for port := 1; port < MAXPORT; port++ {
		f.portIdFree(port)
	}
}

func (f *FDB) initPortMap() {
	f.portMap = portMaps{
		ports: make(map[int]portIO),
	}
}

func (f *FDB) incPortNum() {
	atomic.AddInt32(&f.portPool.allocNum, 1)
}

func (f *FDB) decPortNum() {
	n := atomic.AddInt32(&f.portPool.allocNum, -1)
	if n < 0 {
		panic("allocNum < 0")
	}
}

func (f *FDB) getPortNum() int {
	return int(atomic.LoadInt32(&f.portPool.allocNum))
}

func (f *FDB) portIdAlloc() int {
	f.portPool.Lock()
	portId := f.portPool.portIdPool[0]
	f.portPool.portIdPool[0] = f.portPool.portIdPool[portId]

	f.portPool.Unlock()
	return portId
}

func (f *FDB) portIdFree(portId int) {
	f.portPool.Lock()
	f.portPool.portIdPool[portId] = f.portPool.portIdPool[0]
	f.portPool.portIdPool[0] = portId
	f.portPool.Unlock()
}

func (f *FDB) getPortMap(portId int) (portIO, bool) {
	f.portMap.RLock()
	pio, ok := f.portMap.ports[portId]
	f.portMap.RUnlock()
	return pio, ok
}

func (f *FDB) addPortMap(portId int, pio portIO) {
	f.portMap.Lock()
	f.portMap.ports[portId] = pio
	f.portMap.Unlock()
}

func (f *FDB) delPortMap(portId int) {
	f.portMap.Lock()
	delete(f.portMap.ports, portId)
	f.portMap.Unlock()
}

func (f *FDB) JoinFwdPort(pio portIO, inc bool) (portId int) {
	portId = f.portIdAlloc()
	if portId == 0 {
		return
	}
	if inc {
		f.incPortNum()
	}
	f.addPortMap(portId, pio)
	return
}

func (f *FDB) ReleaseFwdPort(portId int, dec bool) {
	if portId == 0 {
		return
	}
	if pio, ok := f.getPortMap(portId); ok {
		f.DelFmnByPortIO(pio)
		f.delPortMap(portId)
		f.portIdFree(portId)
		if dec {
			f.decPortNum()
		}
	}
}
