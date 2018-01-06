package vnet

import (
	"encoding/json"
	"fdb"
	"flag"
	"fmt"
	"log"
	"mylog"
	"net/http"
	"net/url"
	"netstat"
	"packet"
)

var (
	DebugEn      = flag.Bool("DebugEn", false, "debug, show ip packet information")
	ShowInfoAddr = flag.String("showInfoAddr", "localhost:18181", "show info addr, show stat, clientmac")
)
var handlers map[string]func(http.ResponseWriter, *http.Request)
var version string = "version is not set"

func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

type httpHandlers struct {
	path    string
	handler func(http.ResponseWriter, *http.Request)
}
type Debug struct{}

func DebugInfoServe(showInfoAddr string) {
	if showInfoAddr == "" {
		showInfoAddr = *ShowInfoAddr
	}
	log.Printf("========= showInfoAddr = %s =============\n", showInfoAddr)
	registerHandlers(regHandlers)
	go http.ListenAndServe(showInfoAddr, &Debug{})
}

func registerHandlers(hhs []httpHandlers) {
	handlers = make(map[string]func(http.ResponseWriter, *http.Request))
	for _, hh := range hhs {
		handlers[hh.path] = hh.handler
	}
}
func showRegHandlersPath() string {
	var ss string
	for _, hh := range regHandlers {
		ss = ss + hh.path
	}
	return ss
}

var regHandlers = []httpHandlers{
	httpHandlers{
		path:    "/debug",
		handler: enableDebug,
	},
	httpHandlers{
		path:    "/nodebug",
		handler: disableDebug,
	},
	httpHandlers{
		path:    "/tunstat",
		handler: showTunStat,
	},
	httpHandlers{
		path:    "/clientmac",
		handler: showClientMac,
	},
	httpHandlers{
		path:    "/vnetStat",
		handler: showVnetStat,
	},
	httpHandlers{
		path:    "/clientMaster",
		handler: showClientMasterInfo,
	},
	httpHandlers{
		path:    "/conntrack",
		handler: ShowConntrack,
	},
	httpHandlers{
		path:    "/version",
		handler: showVersion,
	},
	httpHandlers{
		path:    "/mylog",
		handler: showLogInfo,
	},
}

func showLogInfo(w http.ResponseWriter, req *http.Request) {
	queryForm, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	if len(queryForm["level"]) > 0 {
		level := queryForm["level"][0]
		if level != "" {
			if err := mylog.SetLogLevel(level); err == nil {
				fmt.Fprintf(w, "set log level %s success\n", level)
			} else {
				fmt.Fprintf(w, "%s\n", err.Error())
			}
			return
		}
	}
	fmt.Fprintf(w, "/mylog?level =%s\n", mylog.ShowSupportLevels())
}

func showVersion(w http.ResponseWriter, req *http.Request) {
	jsonData, err := json.Marshal(version)
	if err != nil {
		log.Println(err)
		return
	}
	w.Write(jsonData)
}

func ShowConntrack(w http.ResponseWriter, req *http.Request) {
	ctInfo := netstat.ShowConntrack()
	netstatInfo, err := json.MarshalIndent(ctInfo, "", "\t")
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(netstatInfo)
}

func showClientMasterInfo(w http.ResponseWriter, req *http.Request) {
	cm := showClientMaster()
	cmBuf, err := json.MarshalIndent(cm, "", "\t")
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(cmBuf)
}

func enableDebug(w http.ResponseWriter, req *http.Request) {
	*DebugEn = true
	log.Printf("set debug = true, *DebugEn=%v \n", *DebugEn)
	fmt.Fprintf(w, "set debug = true, *DebugEn=%v \n", *DebugEn)
}

func disableDebug(w http.ResponseWriter, req *http.Request) {
	*DebugEn = false
	log.Printf("set debug = false, *DebugEn=%v \n", *DebugEn)
	fmt.Fprintf(w, "set debug = false, *DebugEn=%v \n", *DebugEn)
}

func showTunStat(w http.ResponseWriter, req *http.Request) {
	var statstr string
	for devName, v := range tcMap {
		statstr += fmt.Sprintf("dev name %s: conn %s, rx %d,tx %d bytes\n", devName, v.String(), v.rx_bytes, v.tx_bytes)
	}
	fmt.Fprintf(w, statstr)
}

func showClientMac(w http.ResponseWriter, req *http.Request) {
	mc := fdb.ShowClientMac()
	mcjson, err := json.MarshalIndent(mc, "", "\t")
	if err != nil {
		log.Println(err)
		return
	}
	//log.Println(string(mcjson))
	w.Write(mcjson)
}

func showVnetStat(w http.ResponseWriter, req *http.Request) {
	//statBuf, err := json.Marshal(VnetStats)
	statBuf, err := json.MarshalIndent(VnetStats, "", "\t")
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(statBuf)
}

func (db *Debug) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if h, ok := handlers[req.URL.Path]; ok {
		h(w, req)
	} else {
		fmt.Fprintf(w, "just support: %s\n", showRegHandlersPath())
	}
}

func ShowPktInfo(pkt []byte, ss string) {
	if *TunType == 1 {
		ether := packet.TranEther(pkt)
		if ether.IsBroadcast() && ether.IsArp() {
			log.Println("---------arp broadcast from tun/tap ----------")
			log.Printf("dst mac :%s", ether.DstMac.String())
			log.Printf("src mac :%s", ether.SrcMac.String())
		}
		/*
			if !ether.IsArp() && !ether.IsIpPtk() {
				//mylog.Warning(" not arp ,and not ip packet, ether type =0x%02x===============\n", ether.Proto)
				continue
			}*/

		if ether.IsIpPtk() {
			iphdr, err := packet.ParseIPHeader(pkt[packet.EtherSize:])
			if err != nil {
				log.Printf("%s, ParseIPHeader err: %s\n", ss, err.Error())
			}
			fmt.Printf("%s: %s\n", ss, iphdr.String())
		}
	} else {
		iphdr, err := packet.ParseIPHeader(pkt)
		if err != nil {
			log.Printf("%s,ParseIPHeader err: %s\n", ss, err.Error())
		}
		fmt.Printf("%s: %s\n", ss, iphdr.String())
	}
}

func showClientMaster() []string {
	var cm []string
	var fdbs, slaves, info string
	for _, client := range ClientMaster {
		for fdbid, _ := range client.fdbJoined {
			fdbs += fmt.Sprintf("%d", fdbid)
		}
		// if ml, ok := client.cio.(*multiLinkIO); ok {
		// 	for _, slave := range ml.slaves {
		// 		slaves += slave.String()
		// 	}
		// 	info = ml.info()
		// }
		cm = append(cm, fmt.Sprintf("%s, fdbs:%s, slaves=%s", client.String()+info, fdbs, slaves))
	}
	return cm
}
