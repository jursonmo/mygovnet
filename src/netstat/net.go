package netstat

import (
	"log"
	"packet"
	"time"
	"timer"
	"mylog"
)

type networkProtoNum uint16
type transportProtoNum uint8

const (
	IPv4ProtocolNumber networkProtoNum   = 0x0800
	ARPProtocolNumber  networkProtoNum   = 0X0806
	TCPProtocolNumber  transportProtoNum = 6
	UDPProtocolNumber  transportProtoNum = 17
)

type networkHandler struct {
	proto  networkProtoNum
	handle func(pkb *packet.PktBuf)
}
type transportHandler struct {
	proto  transportProtoNum
	handle func(pkb *packet.PktBuf)
}

var netStatQueue chan *packet.PktBuf
var networkProtos map[networkProtoNum]*networkHandler
var transportProtos map[transportProtoNum]*transportHandler

var networkHandlers []*networkHandler = []*networkHandler{
	&networkHandler{IPv4ProtocolNumber, ipHandler},
	&networkHandler{ARPProtocolNumber, arpHandler},
}
var transportHandlers []*transportHandler = []*transportHandler{
	&transportHandler{TCPProtocolNumber, tcpHandler},
	&transportHandler{UDPProtocolNumber, udpHandler},
}

func init() {
	netStatQueue = make(chan *packet.PktBuf, 4096)
	networkProtos = make(map[networkProtoNum]*networkHandler)
	transportProtos = make(map[transportProtoNum]*transportHandler)
	registerNetworkProto()
	registerTransportProto()
	go netStatHandle()
}

func registerNetworkProto() {
	for _, nh := range networkHandlers {
		networkProtos[nh.proto] = nh
	}
}

func registerTransportProto() {
	for _, th := range transportHandlers {
		transportProtos[th.proto] = th
	}
}

func getTCPTuple(pkb *packet.PktBuf) ctTuple {
	ipHeader := IPv4(pkb.LoadNetworkData())
	saddr := ipHeader.SourceAddress()
	daddr := ipHeader.DestinationAddress()
	proto := ipHeader.Protocol()
	transportHeader := TCP(pkb.LoadTransportData())
	sport := transportHeader.SourcePort()
	dport := transportHeader.DestinationPort()
	return ctTuple{saddr, daddr, sport, dport, proto}
}

func getUDPTuple(pkb *packet.PktBuf) ctTuple {
	ipHeader := IPv4(pkb.LoadNetworkData())
	saddr := ipHeader.SourceAddress()
	daddr := ipHeader.DestinationAddress()
	proto := ipHeader.Protocol()
	transportHeader := UDP(pkb.LoadTransportData())
	sport := transportHeader.SourcePort()
	dport := transportHeader.DestinationPort()
	return ctTuple{saddr, daddr, sport, dport, proto}
}

func NetStatPut(pkb *packet.PktBuf) {
	pkb.HoldPktBuf()
	netStatQueue <- pkb
}

func netStatHandle() {
	for pkb := range netStatQueue {
		etherPktHandle(pkb)
		pkb.ReleasePktBuf()
	}
}

func etherPktHandle(pkb *packet.PktBuf) {
	etherData := pkb.LoadUserData()
	etherHeader := packet.TranEther(etherData)
	handler, ok := networkProtos[networkProtoNum(etherHeader.GetProto())]
	if !ok {
		//TODO
		return
	}
	pkb.SetNetworkHeader(packet.EtherSize)
	handler.handle(pkb)
}

func arpHandler(pkb *packet.PktBuf) {

}

func ipHandler(pkb *packet.PktBuf) {
	ipHeader := IPv4(pkb.LoadNetworkData())
	if !ipHeader.IsIPv4() {
		return
	}
	handler, ok := transportProtos[transportProtoNum(ipHeader.Protocol())]
	if !ok {
		return
	}
	pkb.SetTransportHeader(int(ipHeader.HeaderLength()))
	handler.handle(pkb)
}

func tcpHandler(pkb *packet.PktBuf) {
	pktLen := uint64(pkb.GetDataLen())

	tuple := getTCPTuple(pkb)
	zone := pkb.GetPktVid()
	nct, ok := GetNctByZone(zone)
	if !ok {
		return
	}

	tcpHeader := TCP(pkb.LoadTransportData())
	tcpFlag := tcpHeader.Flags()

	ct, ok := nct.findConntrack(tuple)
	//SYN-SENT
	if IsSyn(tcpFlag) {
		//TODO: add a conntrack
		if ct == nil {
			if !pkb.IsOutBound() {
				return
			}
			//log.Printf("SYN-SENT: syn packet,tuple:%s\n", tuple.String())
			mylog.Debug("SYN-SENT: syn packet,tuple:%s\n", tuple.String())
			ct, _ = nct.CreateConntrack(tuple)
			ct.outBound = true
			ct.stats[ctOrigin] += pktLen
			ct.timer = timer.NewTimerFunc(time.Second*CT_TCP_SYN_TIMEOUT, ctTimeoutDel, ct) //ctTCPHandShakeTimeout
		} else { //repeat syn ??
			log.Printf("repeat SYN-SENT ??: syn packet,tuple:%s\n", tuple.String())
			if ct.status == CT_NEW {
				//if ctTimeoutDel have invoking , this maybe lunch ctTimeoutDel later, del one conntrack twice is ok
				ct.timer.Reset(time.Second * CT_TCP_SYN_TIMEOUT)
			}
		}
		return
	}
	//if ct is nil, pkb isn't syn,
	if !ok {
		//log.Printf("can't find tuple:%v, zone=%d, and pkb is not syn\n", tuple, zone)
		return
	}

	if ct.status == CT_DEL {
		return
	}

	dir := ct.Dir(tuple)
	ct.stats[dir] += pktLen

	if tcpFlag&TCPFlagRst != 0 || tcpFlag&TCPFlagFin != 0 {
		//log.Printf("CT_DEL: rst or fin packet,tuple=%s\n", tuple.String())
		mylog.Debug("CT_DEL: rst or fin packet,tuple=%s\n", tuple.String())
		ct.Lock()
		if ct.status == CT_DEL { //ctESTABLISHEDTimeout have invoke,
			ct.Unlock()
			return
		}
		ct.status = CT_DEL
		ct.lastTime = time.Now().UTC() // record die timout time
		if ct.timer != nil {
			if ct.timer.Stop() { //ctESTABLISHEDTimeout可能正在执行, return true, 表明成功删除了定时器，即连接没有因超时而被删除
				ct.timer = timer.NewTimerFunc(time.Second*CT_DEL_TIMEOUT, ctTimeoutDel, ct) //如果此时ctESTABLISHEDTimeout也执行了, 所以要加锁
			}
		}
		ct.Unlock()
		return
	}

	if ct.status == CT_ESTABLISHED {
		return
	}

	//SYN-RCVD
	if IsSynAck(tcpFlag) {
		//TODO: change conntrack state
		if dir != ctReply {
			log.Printf("Error :SYN-RCVD but dir != ctReply,tuple=%s\n", tuple.String())
			return
		}
		//log.Printf("SYN-RCVD: SynAck packet,tuple=%s\n", tuple.String())
		mylog.Debug("SYN-RCVD: SynAck packet,tuple=%s\n", tuple.String())

		if ct.timer.Reset(time.Second * CT_TCP_SYN_RCVD_TIMEOUT) { // syn ctTimeoutDel 并发执行的情况, 如果ctTimeoutDel先触发，就会删除这个连接跟踪，netstat丢失这个连接信息而已
			// CT_TCP_SYN_RCVD_TIMEOUT会再次触发ctTimeoutDel,没有影响，
			ct.status = CT_REPLY
		}
		return
	}

	if IsAck(tcpFlag) && ct.status == CT_REPLY {
		//TODO: change conntrack state
		//log.Printf("CT_ESTABLISHED: Ack packet,tuple=%s, ct.stats=%v\n", tuple.String(), ct.stats)
		mylog.Info("CT_ESTABLISHED: Ack packet,tuple=%s, ct.stats=%v\n", tuple.String(), ct.stats)
		//ct.timer.Reset(time.Second * 15) //testing

		if ct.timer.Stop() { // CT_TCP_SYN_RCVD_TIMEOUT  ctTimeoutDel 并发执行的情况，如果ctTimeoutDel先触发，就会删除这个连接跟踪，这里会返回false
			ct.status = CT_ESTABLISHED
			ct.timer = timer.NewTimerFunc(time.Second*CT_TCP_ESTABLISHED_TIMEOUT, ctESTABLISHEDTimeout, ct, ct.stats)
		}
		return
	}
}

func udpHandler(pkb *packet.PktBuf) {
	pktLen := uint64(pkb.GetDataLen())

	tuple := getUDPTuple(pkb)
	zone := pkb.GetPktVid()
	nct, ok := GetNctByZone(zone)
	if !ok {
		return
	}

	ct, ok := nct.findConntrack(tuple)
	if !ok {
		ct, _ = nct.CreateConntrack(tuple)
		if pkb.IsOutBound() {
			ct.outBound = true
		}
		ct.stats[ctOrigin] += pktLen
		ct.timer = timer.NewTimerFunc(time.Second*CT_UDP_NEW_TIMEOUT, ctTimeoutDel, ct, ct.stats)
		return
	}

	if ct.status == CT_DEL {
		return
	}

	dir := ct.Dir(tuple)
	ct.stats[dir] += pktLen
	if ct.status == CT_ESTABLISHED {
		return
	}

	if dir == ctReply {
		//ct.Lock()
		if ct.timer.Stop() {
			ct.status = CT_ESTABLISHED
			ct.timer = timer.NewTimerFunc(time.Second*CT_UDP_ESTABLISHED_TIMEOUT, ctESTABLISHEDTimeout, ct, ct.stats)
		}
		//ct.Unlock() // if check timer.Stop, if means there is no ctTimeoutDel handling at the same time, so don't need to Lock()
	}
}
