package nat

import (
	"bytes"
	"fmt"
	"strconv"
)

func ctStatusString(st uint8) string {
	switch st {
	case CT_NEW:
		return "NEW"
	case CT_REPLY:
		return "REPLY"
	case CT_ESTABLISHED:
		return "ESTABLISHED"
	case CT_DEL:
		return "DEL"
	default:
		return "unknown"
	}
}

func ctProtoString(p uint8) string {
	switch p {
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	default:
		return "unknown"
	}
}

func ipString(ipaddr uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", byte(ipaddr>>24), byte(ipaddr>>16), byte(ipaddr>>8), byte(ipaddr))
}

func (t ctTuple) String() string {
	var buf bytes.Buffer
	buf.WriteString("src=")
	buf.WriteString(ipString(t.saddr))
	buf.WriteString("dst=")
	buf.WriteString(ipString(t.daddr))
	buf.WriteString("sport=")
	buf.WriteString(strconv.Itoa(int(t.sport)))
	buf.WriteString("dport=")
	buf.WriteString(strconv.Itoa(int(t.dport)))
	buf.WriteString("proto=")
	buf.WriteString(strconv.Itoa(int(t.proto)))
	return buf.String()
	//return fmt.Sprintf("src=%s,dst=%s,sport=%d,dport=%d,proto=%d", ipString(t.saddr), ipString(t.daddr), t.sport, t.dport, t.proto)
}

func (ct *conntrack) String() string {
	return fmt.Sprintf("status:%s origin:%s(bytes:%d), reply:%s(bytes:%d),outBound=%v, netZone=%d", ctStatusString(ct.status),
		ct.tuple[ctOrigin].String(), ct.stats[ctOrigin], ct.tuple[ctReply].String(), ct.stats[ctReply], ct.outBound, ct.nct.netZone)
}

func ShowConntrack() map[uint16]map[uint32]string {
	conntrackInfo := make(map[uint16]map[uint32]string)
	for zone, nct := range globalConntrack {
		nctInfo := make(map[uint32]string)
		ctIndex := uint32(0)
		checkTuple := make(map[ctTuple]struct{})
		for tuple, ct := range nct.conntracks {
			if _, ok := checkTuple[tuple]; !ok {
				checkTuple[invert(tuple)] = struct{}{}
				nctInfo[ctIndex] = ct.String()
				ctIndex++
			}
		}
		conntrackInfo[zone] = nctInfo
	}
	return conntrackInfo
}
