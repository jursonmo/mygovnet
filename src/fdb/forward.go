package fdb

import (
	"log"
	"mylog"
	"packet"
)

func (f *FDB) flood(pio portIO, pkt *packet.PktBuf) (fwd bool) {
	//log.Printf("-------------- flooding  ------------\n")
	for _, p := range f.portMap.ports {
		if p != pio {
			p.PutPktToChan(pkt)
			fwd = true
		} else { //for test
			//log.Printf(" it is %s need to flood\n", p.String())
		}
	}
	//log.Printf("-------------- flood end  ------------\n")
	return
}

//payload offset
//func Forward(pio portIO, pkt []byte, offset int) {
func (f *FDB) Forward(pio portIO, pkt *packet.PktBuf) bool {
	//len := len(pkt)
	data := pkt.LoadUserData()
	ether := packet.TranEther(data)
	if !ether.IsArp() && !ether.IsIpPtk() {
		return false
	}

	if fmn, ok := f.Get(ether.SrcMac); ok {
		if fmn.pio == pio {
			fmn.updateTime()
		} else {
			if fmn.maybeExpire() || packet.IsArpRelpy(data) {
				log.Printf("-------------- update mac=%s, change port: %s to %s  ------------\n", ether.SrcMac.String(), fmn.pio.String(), pio.String())
				fmn.updatePio(pio)
				fmn.updateTime()
				//Fdb().Set(ether.SrcMac, fmn) //update mactable
			} else {
				log.Printf("-------------- DROP, maybe loop, mac=%s, org port: %s, now recve from port %s ----------\n", ether.SrcMac.String(), fmn.pio.String(), pio.String())
				return false
			}
		}
	} else {
		f.Add(ether.SrcMac, pio)
	}
	//arp broadcast
	if ether.IsArp() && ether.IsBroadcast() {
		mylog.Info("-------------- ARP Broadcast vid=%d------------\n", pkt.GetPktVid())
		log.Printf("dst  mac %s\n", ether.DstMac.String())
		log.Printf("src  mac %s\n", ether.SrcMac.String())
		//flood(c, pkt, len)
		return f.flood(pio, pkt)
	} else {
		//it is ip packet or unicast arp
		if fmn, ok := f.Get(ether.DstMac); ok {
			if fmn.pio != pio {
				fmn.pio.PutPktToChan(pkt)
			}
			return true
		} else {
			log.Printf("%s ,src mac %s ,dst mac %s, vid=%d, dst mac is unkown ,so flood\n", pio.String(), ether.DstMac.String(), ether.SrcMac.String(), pkt.GetPktVid())
			//flood(pio, pkt, len)
			return f.flood(pio, pkt)
		}
	}
}
