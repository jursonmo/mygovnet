package nat

import (
	"encoding/binary"
	"fmt"
	"log"
	"mylog"
	"net"
	"packet"
	"time"
	"timer"
	"unsafe"
)

const (
	ACCEPT = 0
	DROP   = 1
	STOLEN = 2
)

var snatEnable bool
var snatIP uint32

func IsEnable() bool {
	return snatEnable
}
func SetSnatIP(enable bool, ip string) {
	snatEnable = enable
	ipaddr := net.ParseIP(ip)
	if ipaddr == nil {
		log.Fatalf("Bad IP address: %v", ip)
	}
	snatIP = binary.BigEndian.Uint32(ipaddr.To4())
	mylog.Info("doSnat =%v ,ip=%s, snatIP=%d, ", snatEnable, ip, snatIP)
}

type networkProtoNum uint16
type transportProtoNum uint8

const (
	IPv4ProtocolNumber networkProtoNum   = 0x0800
	ARPProtocolNumber  networkProtoNum   = 0X0806
	ICMPProtocolNumber transportProtoNum = 1
	TCPProtocolNumber  transportProtoNum = 6
	UDPProtocolNumber  transportProtoNum = 17
)

type networkHandler struct {
	proto  networkProtoNum
	handle func(pkb **packet.PktBuf) int
}
type transportHandler struct {
	proto  transportProtoNum
	handle func(pkb *packet.PktBuf) int
}

//var netStatQueue chan *packet.PktBuf
var networkProtos map[networkProtoNum]*networkHandler
var transportProtos map[transportProtoNum]*transportHandler

var networkHandlers []*networkHandler = []*networkHandler{
	&networkHandler{IPv4ProtocolNumber, ipHandler},
	&networkHandler{ARPProtocolNumber, arpHandler},
}
var transportHandlers []*transportHandler = []*transportHandler{
	&transportHandler{TCPProtocolNumber, tcpHandler},
	&transportHandler{UDPProtocolNumber, udpHandler},
	&transportHandler{ICMPProtocolNumber, icmpHandler},
}

func init() {
	//netStatQueue = make(chan *packet.PktBuf, 4096)
	networkProtos = make(map[networkProtoNum]*networkHandler)
	transportProtos = make(map[transportProtoNum]*transportHandler)
	registerNetworkProto()
	registerTransportProto()
	//go natHandle()
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

// func NatPut(pkb *packet.PktBuf) {
// 	pkb.HoldPktBuf()
// 	netStatQueue <- pkb
// }

// func natHandle() {
// 	for pkb := range netStatQueue {
// 		etherPktHandle(pkb)
// 		pkb.ReleasePktBuf()
// 	}
// }

func DoNat(pkb **packet.PktBuf) int {
	if snatIP == 0 {
		return ACCEPT
	}
	return etherPktHandle(pkb)
}

func etherPktHandle(ppkb **packet.PktBuf) int {
	pkb := *ppkb
	etherData := pkb.LoadUserData()
	etherHeader := packet.TranEther(etherData)
	handler, ok := networkProtos[networkProtoNum(etherHeader.GetProto())]
	if !ok {
		//TODO
		return DROP
	}
	pkb.SetNetworkHeader(packet.EtherSize)
	return handler.handle(ppkb)
}

func arpHandler(pkb **packet.PktBuf) int {
	return ACCEPT
}

func ipHandler(ppkb **packet.PktBuf) int {
	pkb := *ppkb
	ipHeader := IPv4(pkb.LoadNetworkData())
	if !ipHeader.IsIPv4() {
		return DROP
	}
	offset := ipHeader.FragmentOffset()
	more := (ipHeader.Flags() & IPv4FlagMoreFragments) != 0
	log.Printf("more:%v\n", more)
	if offset != 0 || more {
		log.Printf("=========Fragment FragmentOffset FragmentOffset============\n")
		// ipHeader.SetSourceAddress(snatIP) //ct.tuple[rdir].daddr
		// ipHeader.SetChecksum(0)
		// ipHeader.SetChecksum(^ipHeader.CalculateChecksum())
		if pkb.NetworkDataLen() != ipHeader.TotalLength() {
			log.Panicf("pkb.NetworkDataLen() =%d, ipHeader.TotalLength()=%d\n", pkb.NetworkDataLen(), ipHeader.TotalLength())
		}
		ipDataLen := ipHeader.TotalLength() - uint16(ipHeader.HeaderLength())

		zone := pkb.GetPktVid()
		nct, ok := GetNctByZone(zone)
		if !ok {
			fmt.Println("GetNctByZone fail, zone=%d\n", zone)
			return DROP
		}
		fc := fragInfo{sip: ipHeader.SourceAddress(), dip: ipHeader.DestinationAddress(), id: ipHeader.ID(), proto: ipHeader.Protocol()}
		fq, ok := nct.fragQueueFind(fc)
		if !ok {
			//frag := createFragment(offset, ipDataLen, pkb)
			fq = newfragQueue()
			nct.fragQueueAdd(fc, fq)
		}
		ok, done := fq.ProcessFragment(offset, ipDataLen, more, pkb)
		if !ok {
			//overlapped ? TODO: delete fq
			return DROP
		}
		//in the fraqueue, so hold
		pkb.HoldPktBuf()

		if !done {
			return STOLEN
		}
		//done ,defrag completed, return head
		*ppkb = fq.completed()
		pkb = *ppkb
	}
	handler, ok := transportProtos[transportProtoNum(ipHeader.Protocol())]
	if !ok {
		if pkb.Next() != nil { //frag_liist
			for p := pkb; p != nil; {
				dropPkb := p
				p = p.Next()
				dropPkb.PutPktToPool()
			}
		}
		return DROP
	}
	pkb.SetTransportHeader(int(ipHeader.HeaderLength()))
	return handler.handle(pkb)
}

func icmpHandler(pkb *packet.PktBuf) int {
	return ACCEPT
}
func tcpHandler(pkb *packet.PktBuf) int {
	log.Printf("unsuport tcp for now")
	return ACCEPT
}

func udpHandler(pkb *packet.PktBuf) int {
	pktLen := uint64(pkb.GetDataLen())
	fmt.Printf("-----------------udp udp ----------\n")
	tuple := getUDPTuple(pkb)
	zone := pkb.GetPktVid()
	nct, ok := GetNctByZone(zone)
	if !ok {
		fmt.Println("GetNctByZone fail, zone=%d\n", zone)
		return DROP
	}

	ct, ok := nct.findConntrack(tuple)
	if !ok {
		ct, _ = nct.CreateConntrack(tuple)
		pkb.SetCt((unsafe.Pointer)(ct))
		//if pkb.TryNat() {
		if !getUniqueTuple(pkb) {
			log.Panic("getUniqueTuple fail")
			return DROP
		}
		//}
		doNat(pkb)

		ct.stats[ctOrigin] += pktLen
		ct.timer = timer.NewTimerFunc(time.Second*CT_UDP_NEW_TIMEOUT, ctTimeoutDel, ct, ct.stats)
		return ACCEPT
	}

	pkb.SetCt((unsafe.Pointer)(ct))
	dir := ct.Dir(tuple)
	pkb.SetDir(dir)

	doNat(pkb)

	ct.stats[dir] += pktLen
	if ct.status == CT_ESTABLISHED {
		return ACCEPT
	}
	if dir == ctReply {
		//ct.Lock()
		if ct.timer.Stop() {
			ct.status = CT_ESTABLISHED
			ct.timer = timer.NewTimerFunc(time.Second*CT_UDP_ESTABLISHED_TIMEOUT, ctESTABLISHEDTimeout, ct, ct.stats)
		}
		//ct.Unlock() // if check timer.Stop, if means there is no ctTimeoutDel handling at the same time, so don't need to Lock()
	}
	return ACCEPT
}
func rDir(dir int) (rdir int) {
	if dir == 0 {
		rdir = 1
	} else {
		rdir = 0
	}
	return
}
func doNat(pkb *packet.PktBuf) {
	ct := (*conntrack)(pkb.GetCt()) //ct := (*conntrack)(nil) is ok, type data
	if ct == nil {
		log.Panic("ct == nil")
	}
	if ct.natFlag == 0 {
		return
	}
	var doSnat bool
	dir := pkb.GetDir()
	rdir := rDir(dir)
	if ct.natFlag == snatFlag {
		if dir == ctOrigin {
			doSnat = true
		}
	} else {
		return
	}
	if ct.tuple[ctOrigin].proto == uint8(UDPProtocolNumber) {
		ipHeader := IPv4(pkb.LoadNetworkData())
		length := ipHeader.TotalLength() - uint16(ipHeader.HeaderLength())

		// if flag := ipHeader.Flags(); flag&IPv4FlagMoreFragments != 0 {
		// 	//save sip, dip , id -->ct
		// 	fc := fragInfo{sip: ipHeader.SetSourceAddress(), dip: ipHeader.DestinationAddress(), id: ipHeader.ID(), proto: ipHeader.Protocol()}
		// 	ct.nct.fragMapAdd(fc, ct)
		// }

		udpheader := UDP(pkb.LoadTransportData())
		if doSnat {
			ipHeader.SetSourceAddress(ct.tuple[rdir].daddr)

			if ct.tuple[dir].sport != ct.tuple[rdir].dport {
				udpheader.SetSourcePort(ct.tuple[rdir].dport)
			}
			log.Printf("do snat, change sip %s, sport %d \n", ipString(ct.tuple[rdir].daddr), ct.tuple[rdir].dport)
		} else { //dnat
			ipHeader.SetDestinationAddress(ct.tuple[rdir].saddr)
			udpheader.SetDestinationPort(ct.tuple[rdir].sport)
		}
		ipHeader.SetChecksum(0)
		ipHeader.SetChecksum(^ipHeader.CalculateChecksum())
		udpheader.SetChecksum(0)
		xsum := PseudoHeaderChecksum(uint8(UDPProtocolNumber), ipHeader.SourceAddrBuf(), ipHeader.DestinationAddrBuf())
		xsum = udpheader.CalculateChecksum(xsum, length)
		xsum = udpheader.CalChecksum(xsum, length)
		log.Printf("--------------udp.Length()=%d-------------\n", udpheader.Length())

		i := 0
		for npkb := pkb.Next(); npkb != nil; npkb = npkb.Next() {
			i++
			log.Printf("frag Checksum i=%d\n", i)
			ipHeader = IPv4(npkb.LoadNetworkData())
			ipHeader.SetSourceAddress(ct.tuple[rdir].daddr)
			ipHeader.SetChecksum(0)
			ipHeader.SetChecksum(^ipHeader.CalculateChecksum())
			npkb.SetTransportHeader(int(ipHeader.HeaderLength()))

			xsum = Checksum(npkb.LoadTransportData(), xsum)
			log.Printf(" finish frag Checksum i=%d\n", i)
		}

		//udpheader.SetChecksum(^xsum)
		udpheader.SetChecksum(0)
	}
}

func getUniqueTuple(pkb *packet.PktBuf) bool {
	ct := (*conntrack)(pkb.GetCt()) //ct := (*conntrack)(nil) is ok, type data
	if ct == nil {
		log.Panic("ct == nil")
	}
	replyTuple := &ct.tuple[ctReply]
	replyTuple.daddr = snatIP
	for {
		_, ok := ct.nct.findConntrack(*replyTuple)
		if !ok {
			ct.natFlag = snatFlag
			log.Printf("getUniqueTuple ok,origin:%s, reply:%s\n", ct.tuple[ctOrigin].String(), ct.tuple[ctReply].String())
			ct.ctConfirm()
			log.Printf("all ct: %v\n", ShowConntrack())
			return true
		}
		replyTuple.dport++
	}
	return false
}
