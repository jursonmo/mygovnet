package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"

	"net/http"
	_ "net/http/pprof"
	"time"

	"mylog"
	"netstat"
	"vnet"

	"github.com/BurntSushi/toml"
)

type tlsConfig struct {
	TlsEnable bool
	TlsSK     string
	TlsSP     string
}

type TunConfig struct {
	Tuns []vnet.TunConf
}

type HeartbeatConfig struct {
	HeartbeatIdle int
	HeartbeatCnt  int
	HeartbeatIntv int
}

type vnetConfig struct {
	ListenAddr string
	SerAddr    []string
	CryptType  string
	Vids       []int

	BackupLinkAddr []string

	HeartbeatConf HeartbeatConfig
	TlsConf       tlsConfig
	TunConf       TunConfig

	UpRateLimit   int64
	DownRateLimit int64
	LogFile       string
	LogLevel      string
	ShowInfoAddr  string
	RouteConf     string

	CheckTunPkt   bool
	NetStatEnable bool

	PprofEnable bool
	PpAddr      string
	Ve          string
}

var (
	buildTime   string
	goVersion   string
	version     string
	commitId    string
	appVersion  = "2.0.0"
	vnetConf    vnetConfig
	showVersion = flag.Bool("v", false, "show version information")
	listenAddr  = flag.String("listenAddr", "", " listen addr, like 203.156.34.98:7878")
	tlsEnable   = flag.Bool("tls", false, "enable tls connect")
	tlsSK       = flag.String("server.key", "./config/server.key", "tls server.key")
	tlsSP       = flag.String("server.pem", "./config/server.pem", "tls server.pem")
	configFile  = flag.String("c", "", "config file")
)

func newListener() net.Listener {
	var ln net.Listener
	var err error
	if *tlsEnable {
		cert, err := tls.LoadX509KeyPair(*tlsSP, *tlsSK)
		if err != nil {
			log.Fatalln(err, *tlsSP, *tlsSK)
		}
		tlsconf := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		ln, err = tls.Listen("tcp4", *listenAddr, tlsconf)
	} else {
		ln, err = net.Listen("tcp4", *listenAddr)
	}
	//ln, err := net.Listen("tcp4", *listenAddr)
	if err != nil {
		log.Fatalln(err)
	}
	return ln
}

type myDial struct{}

var myDialer *myDial

func (md *myDial) Connect(dialInfo interface{}) (conn net.Conn) {
	var err error
	n := 1
	serverAddr, ok := dialInfo.(string)
	if !ok {
		panic("myDial dialInfo invalid")
	}
	if len(serverAddr) == 0 {
		panic("myDial len(serverAddr) == 0")
	}

ReConnect:
	mylog.Info("now connecting to  %s \n", serverAddr)
	if *tlsEnable {
		tlsconf := &tls.Config{
			InsecureSkipVerify: true,
		}
		conn, err = tls.Dial("tcp", serverAddr, tlsconf)
	} else {
		//c.conn, err = net.Dial("tcp4", serverAddr)
		conn, err = net.DialTimeout("tcp4", serverAddr, time.Second*5)
	}

	if err != nil {
		mylog.Notice("try to connect to  %s time =%d, err=%s\n", serverAddr, n, err.Error())
		n += 1
		time.Sleep(time.Second * 2)
		goto ReConnect
	}

	mylog.Info("=======success ,client:%s connect to Server:%s ===========\n", conn.LocalAddr().String(), conn.RemoteAddr().String())
	return
}

func main() {
	var ln net.Listener
	var lnAddr string
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	initConfig()
	mylog.InitLog(vnetConf.LogLevel, vnetConf.LogFile)

	log.Printf("appVersion=%s, goVersion=%s, buildTime=%s, commitId=%s\n", appVersion, goVersion, buildTime, commitId)

	log.Printf("listenAddr=%s ,serAddr=%v, enable pprof %v, ppaddr=%s\n", *listenAddr, vnetConf.SerAddr, vnetConf.PprofEnable, vnetConf.PpAddr)
	vnet.ShowBaseInfo()
	vnet.SetVersion(version)
	vnet.DebugInfoServe(vnetConf.ShowInfoAddr)
	vnet.SetVids(vnetConf.Vids)
	vnet.SetDefaultCryptType(vnetConf.CryptType)
	vnet.SetRateLimit(vnetConf.UpRateLimit, vnetConf.DownRateLimit)
	vnet.SetRouteConf(vnetConf.RouteConf)
	vnet.SetCheckTunPkt(vnetConf.CheckTunPkt)
	HeartbeatConf := vnetConf.HeartbeatConf
	vnet.SetHeartbeat(HeartbeatConf.HeartbeatIdle, HeartbeatConf.HeartbeatCnt, HeartbeatConf.HeartbeatIntv)
	netstat.Enable(vnetConf.NetStatEnable)

	if vnetConf.PprofEnable {
		go func() {
			log.Println(http.ListenAndServe(vnetConf.PpAddr, nil))
		}()
	}

	if len(vnetConf.TunConf.Tuns) == 1 && len(vnetConf.SerAddr) == 1 && *listenAddr == "" {
		mylog.Info("============ bind conn to tun, p2p ec mode============\n")
		vnet.ConnBindTun(vnetConf.SerAddr[0], myDialer, vnetConf.TunConf.Tuns[0])
		goto waitLoop
	}

	if len(vnetConf.TunConf.Tuns) > 0 {
		vnet.HandleTuns(vnetConf.TunConf.Tuns)
	}

	if len(vnetConf.BackupLinkAddr) > 0 {
		go vnet.NcAccessBackup(vnetConf.BackupLinkAddr, myDialer)
	}

	if len(vnetConf.SerAddr) > 0 {
		vnet.ConnectNc(vnetConf.SerAddr, myDialer)
	}

	if *listenAddr != "" {
		ln = newListener()
		lnAddr = *listenAddr
	}

	if ln != nil {
		for {
			mylog.Info("\n ............ %s listenning .......\n", lnAddr)
			conn, err := ln.Accept()
			if err != nil {
				log.Fatalln(err)
			}

			go vnet.HandleConn(conn, false)
		}
	}

waitLoop:
	for {
		//runtime.GC() //check GODEBUG="gctrace=1" ./govnet -c ...
		//time.Sleep(time.Second)
		time.Sleep(time.Minute)
	}
}

func initConfig() {
	if *configFile == "" {
		return
	}

	if _, err := toml.DecodeFile(*configFile, &vnetConf); err != nil {
		panic(err)
		return
	}
	fmt.Printf("%#v\n", vnetConf)

	*tlsEnable = vnetConf.TlsConf.TlsEnable
	*tlsSK = vnetConf.TlsConf.TlsSK
	*tlsSP = vnetConf.TlsConf.TlsSP
	*listenAddr = vnetConf.ListenAddr
}
