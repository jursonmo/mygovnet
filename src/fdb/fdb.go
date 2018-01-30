package fdb

import (
	"log"
	"packet"
	"sync"
	"time"
)

const (
	ExpireTime    = 300 //5*60
	DefMacNodeNum = 256
)

type FdbMacNode struct {
	pio portIO
	mac packet.MAC
	ft  uint64
}
type FDB struct {
	lock     *sync.RWMutex
	mactable map[packet.MAC]*FdbMacNode
	portPool portPools
	portMap  portMaps
}

type FdbMaps struct {
	sync.RWMutex
	fdbs map[int]*FDB
}

var FdbMap FdbMaps
var myfdb *FDB
var FdbTick uint64

func NewFdbMacNode(m packet.MAC, pio portIO) *FdbMacNode {
	return &FdbMacNode{
		pio: pio,
		mac: m,
		ft:  FdbTick,
	}
}
func (fmn *FdbMacNode) GetPortIO() portIO {
	return fmn.pio
}

func Fdb() *FDB {
	return myfdb
}
func (f *FDB) FdbMacTable() map[packet.MAC]*FdbMacNode {
	return f.mactable
}
func init() {
	FdbMap = FdbMaps{fdbs: make(map[int]*FDB, 16)}
	//default create fdb0
	//myfdb = NewFdb(0)
	go fdbtick()
}

func NewFdb(fdbId int) *FDB {
	FdbMap.Lock()
	fdb, ok := FdbMap.fdbs[fdbId]
	if !ok {
		fdb = &FDB{
			lock:     new(sync.RWMutex),
			mactable: make(map[packet.MAC]*FdbMacNode, DefMacNodeNum),
		}
		fdb.initPortPool()
		fdb.initPortMap()
		FdbMap.fdbs[fdbId] = fdb
	}
	FdbMap.Unlock()
	return fdb
}

func GetFdbById(fdbId int) (*FDB, bool) {
	FdbMap.RLock()
	fdb, ok := FdbMap.fdbs[fdbId]
	FdbMap.RUnlock()
	return fdb, ok
}

func GetFdbIds() []int {
	var ids []int
	for id, _ := range FdbMap.fdbs {
		ids = append(ids, id)
	}
	return ids
}

func DelFdbById(fdbId int) {
	FdbMap.Lock()
	delete(FdbMap.fdbs, fdbId)
	FdbMap.Unlock()
}

func TryToDelFdbById(fdbId int) {
	if f, ok := GetFdbById(fdbId); ok {
		if f.getPortNum() == 0 {
			DelFdbById(fdbId)
		}
	}
}

func fdbtick() {
	FdbTick = 0
	tt := time.Tick(time.Second)
	for _ = range tt {
		FdbTick = FdbTick + 1
		if FdbTick&63 == 0 {
			for _, f := range FdbMap.fdbs {
				if len(f.FdbMacTable()) > 128 {
					for mac, fmn := range f.FdbMacTable() {
						if FdbTick-fmn.ft > ExpireTime {
							f.Del(mac)
						}
					}
				}
			}
		}
	}
}

func (fmn *FdbMacNode) updateTime() {
	fmn.ft = FdbTick
}

func (fmn *FdbMacNode) updatePio(pio portIO) {
	fmn.pio = pio
}

func (fmn *FdbMacNode) maybeExpire() bool {
	return FdbTick-fmn.ft > 3
}

func (f *FDB) Get(m packet.MAC) (*FdbMacNode, bool) {
	f.lock.RLock()
	fmn, ok := f.mactable[m]
	f.lock.RUnlock()
	return fmn, ok
}
func (f *FDB) Set(m packet.MAC, fmn *FdbMacNode) {
	if m != fmn.mac {
		log.Panicf("m=%s, fmn.mac=%s\n", m.String(), fmn.mac.String())
	}
	f.lock.Lock()
	f.mactable[m] = fmn
	f.lock.Unlock()
}
func (f *FDB) Add(m packet.MAC, pio portIO) {
	fmn := NewFdbMacNode(m, pio)
	f.Set(m, fmn)
}
func (f *FDB) Del(m packet.MAC) {
	f.lock.Lock()
	delete(f.mactable, m)
	f.lock.Unlock()
}
func (f *FDB) DelFmnByPortIO(pio portIO) {
	for m, fmn := range f.FdbMacTable() {
		if fmn.pio == pio {
			f.Del(m)
		}
	}
}

func ShowClientMac() map[int]map[int][]string {
	fdbInfo := make(map[int]map[int][]string)
	for fdbId, fdb := range FdbMap.fdbs {
		portmac := make(map[int][]string)
		for portId, pio := range fdb.portMap.ports {
			//Fdb().lock.RLock()
			portmac[portId] = append(portmac[portId], pio.String())
			for m, fmn := range fdb.mactable {
				if fmn.pio == pio {
					// if len(portmac[portId]) == 0 { //first time
					// 	portmac[portId] = append(portmac[portId], fmn.pio.String())
					// }
					portmac[portId] = append(portmac[portId], m.String())
				}
			}
			//Fdb().lock.RUnlock()
		}
		fdbInfo[fdbId] = portmac
	}

	//log.Println(mt)
	return fdbInfo
}
