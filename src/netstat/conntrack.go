package netstat

import (
	"errors"
	"log"
	"sync"
	"time"
	"timer"
	"mylog"
)

var (
	errZone       = errors.New("unknow zone")
	errCtNil      = errors.New("ct isn't exist")
	netStatEnable = false
)

const (
	ctOrigin = 0
	ctReply  = 1
	ctDirMax = 2

	CT_NEW         = 0
	CT_REPLY       = 1
	CT_ESTABLISHED = 2
	CT_DEL         = 3

	CT_TCP_SYN_TIMEOUT         = 3
	CT_TCP_SYN_RCVD_TIMEOUT    = 3
	CT_TCP_ESTABLISHED_TIMEOUT = 240 //20 test
	CT_TCP_FIN_TIMEOUT         = 10
	CT_DEL_TIMEOUT             = 10
	CT_UDP_NEW_TIMEOUT         = 15
	CT_UDP_ESTABLISHED_TIMEOUT = 120
)

type ctTuple struct {
	saddr uint32
	daddr uint32
	sport uint16
	dport uint16
	proto uint8
}

type conntrack struct {
	sync.Mutex
	status uint8
	stats  [ctDirMax]uint64
	//lastStats [ctDirMax]uint64
	tuple    [ctDirMax]ctTuple
	timer    *timer.Timer
	nct      *netConntrack
	time     time.Time
	lastTime time.Time //finish time
	reported bool
	outBound bool
}

type netConntrack struct {
	netZone uint16
	sync.RWMutex
	conntracks map[ctTuple]*conntrack
}

var globalCtLock sync.RWMutex
var globalConntrack map[uint16]*netConntrack

func init() {
	globalConntrack = make(map[uint16]*netConntrack)
}

func Enable(b bool) {
	netStatEnable = b
}

func IsEnable() bool {
	return netStatEnable
}

func SetNetZone(netZone int) {
	globalCtLock.Lock()
	if _, ok := globalConntrack[uint16(netZone)]; !ok {
		globalConntrack[uint16(netZone)] = newNetConntrack(uint16(netZone))
	}
	globalCtLock.Unlock()
}

func SetNetZones(netZones []int) {
	for _, netZone := range netZones {
		SetNetZone(netZone)
	}
}

func DelNetZone(netZone int) {
	globalCtLock.Lock()
	delete(globalConntrack, uint16(netZone))
	globalCtLock.Unlock()
}

func DelNetZones(netZones []int) {
	for _, netZone := range netZones {
		DelNetZone(netZone)
	}
}

func newConntrack() *conntrack {
	return &conntrack{}
}

func newNetConntrack(netZone uint16) *netConntrack {
	return &netConntrack{
		netZone:    netZone,
		conntracks: make(map[ctTuple]*conntrack),
	}
}

func setGlobalConntrack(vids []int) {
	for _, id := range vids {
		globalConntrack[uint16(id)] = newNetConntrack(uint16(id))
	}
}

func GetNctByZone(zone uint16) (*netConntrack, bool) {
	globalCtLock.RLock()
	nct, ok := globalConntrack[zone]
	globalCtLock.RUnlock()
	return nct, ok
}

func (nct *netConntrack) findConntrack(t ctTuple) (*conntrack, bool) {
	nct.RLock()
	ct, ok := nct.conntracks[t]
	nct.RUnlock()
	return ct, ok
}

func (nct *netConntrack) addConntrack(ct *conntrack) {
	nct.Lock()
	nct.conntracks[ct.tuple[ctOrigin]] = ct
	nct.conntracks[ct.tuple[ctReply]] = ct
	nct.Unlock()
}

func (nct *netConntrack) delConntrack(ct *conntrack) {
	nct.Lock()
	delete(nct.conntracks, ct.tuple[ctOrigin])
	delete(nct.conntracks, ct.tuple[ctReply])
	nct.Unlock()
}

func (ct *conntrack) Dir(t ctTuple) int {
	if ct.tuple[ctOrigin] == t {
		return 0
	}
	return 1
}

func invert(t ctTuple) (reply ctTuple) {
	reply.saddr = t.daddr
	reply.daddr = t.saddr
	reply.sport = t.dport
	reply.dport = t.sport
	reply.proto = t.proto
	return
}

// make sure there is no same conntrack
func (nct *netConntrack) CreateConntrack(t ctTuple) (ct *conntrack, err error) {
	//check if exist
	// ct, ok := nct.findConntrack(t)
	// if ok {
	// 	fmt.Printf("ct exist: %v", ct)
	// 	dir := ct.Dir(t)
	// 	if dir == ctOrigin {
	// 		return
	// 	}
	// 	panic("there is exist ct but dir reverse")
	// }

	//create
	ct = newConntrack()
	ct.tuple[ctOrigin] = t
	ct.tuple[ctReply] = invert(t)
	ct.status = CT_NEW
	ct.nct = nct
	ct.time = time.Now().UTC()
	//ct.lastTime = ct.time

	nct.addConntrack(ct)
	return
}

func ctTCPHandShakeTimeout(t time.Time, args ...interface{}) {
	ct, ok := args[0].(*conntrack)
	if !ok {
		log.Panicf("%v\n", args)
	}
	ct.nct.delConntrack(ct)
	ct.status = CT_DEL
	mylog.Info("ctTCPHandShakeTimeout,time:%v, ct:%s \n", t, ct.String())
}

func ctTimeoutDel(t time.Time, args ...interface{}) {
	ct, ok := args[0].(*conntrack)
	if !ok {
		log.Panicf("%v\n", args)
	}
	ct.status = CT_DEL
	ct.nct.delConntrack(ct)
	mylog.Info("ctTimeout,time:%v, ct:%s \n", t, ct.String())
}

func ctESTABLISHEDTimeout(t time.Time, args ...interface{}) {
	if len(args) != 2 {
		log.Panicf("len(args)=%d != 3\n", len(args))
	}

	ct, _ := args[0].(*conntrack)
	ct.Lock()
	defer ct.Unlock()

	oldStats, _ := args[1].([ctDirMax]uint64)

	if ct.status == CT_DEL { //if receive rst or fin packet, ct is CT_DEL, and have lunch ctTimeoutDel,  so just return
		return
	}
	if oldStats != ct.stats {
		mylog.Debug("ct %s have get some bytes, now stats=%v, old stats=%v, reset timer\n", ct.tuple[ctOrigin].String(), ct.stats, oldStats)
		//ct.timer.Stop()
		timeout := getProtoEstablishTimeout(ct.tuple[ctOrigin].proto)
		ct.timer = timer.NewTimerFunc(time.Second*timeout, ctESTABLISHEDTimeout, ct, ct.stats)
		return
	}

	// ct.nct.delConntrack(ct)
	// log.Printf("ct ESTABLISHEDT Timeout,time:%v, ct:%s \n", t, ct.String())
	mylog.Debug("ct ESTABLISHED Timeout, go in del timer: ct:%s \n", ct.tuple[ctOrigin].String())
	ct.status = CT_DEL
	ct.lastTime = time.Now().UTC() // record die timout time
	ct.timer = timer.NewTimerFunc(time.Second*CT_DEL_TIMEOUT, ctTimeoutDel, ct)
}

func getProtoEstablishTimeout(p uint8) time.Duration {
	switch transportProtoNum(p) {
	case TCPProtocolNumber:
		return CT_TCP_ESTABLISHED_TIMEOUT
	case UDPProtocolNumber:
		return CT_UDP_ESTABLISHED_TIMEOUT
	default:
		panic("unknow proto")
	}
}
