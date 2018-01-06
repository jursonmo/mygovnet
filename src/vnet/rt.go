package vnet

import (
	"bufio"
	"fmt"
	"log"
	"mylog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var routeLock sync.Mutex
var routeConf string = "rt.txt"

func vnetRoute() {
	setRtSig := make(chan os.Signal)
	signal.Notify(setRtSig, syscall.SIGUSR1)
	for {
		//time.Sleep(time.Minute)
		<-setRtSig
		setRoute()
	}
}

func SetRouteConf(rc string) {
	if rc != "" {
		routeConf = rc
	}
	log.Printf("=====SetRouteConf : rc=%s, routeConf=%s====\n", rc, routeConf)
}

func setRoute() {
	var cmd string
	log.Printf("====================== set route begin, *TunName=%s=================\n", *TunName)
	routeLock.Lock()
	defer routeLock.Unlock()
	rtFile, err := os.Open(routeConf)
	if err != nil {
		mylog.Error("open err:%s \n", err.Error())
		return
	}
	//ensure there is no rules about 5588 table when add
	for i := 0; i < 100; i++ {
		err = exec.Command("sh", "-c", "ip ru del from all table 5588").Run()
		if err != nil {
			log.Printf("ip ru del ,time=%d, err=%s \n", i+1, err.Error())
			break
		}
	}
	err = exec.Command("sh", "-c", "ip ru add from all table 5588").Run()
	if err != nil {
		mylog.Error("ip ru add from all table 5588 err:%s \n", err.Error())
		return
	}

	scanner := bufio.NewScanner(rtFile)
	err = exec.Command("sh", "-c", "ip route flush table 5588").Run()
	if err != nil {
		mylog.Error("flush 5588 err:%s \n", err.Error())
	}
	for scanner.Scan() {
		line := scanner.Text()
		rt := strings.Split(line, ",")
		fmt.Println(rt)
		if len(rt) == 1 { // for ec tun point to point mode,it is NOARP
			cmd = fmt.Sprintf("ip route add %s dev %s table 5588", rt[0], *TunName)
		} else if len(rt) == 2 {
			//cmd = fmt.Sprintf("ip route add %s via %s dev %s table 5588", rt[0], rt[1], *TunName)
			cmd = fmt.Sprintf("ip route add %s via %s table 5588", rt[0], rt[1])
		} else {
			cmd = fmt.Sprintf("ip route add %s via %s dev %s table 5588", rt[0], rt[1], rt[2])
		}
		err = exec.Command("sh", "-c", cmd).Run()
		if err != nil {
			mylog.Error("err:%s, cmd = %s \n", err.Error(), cmd)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		//os.Exit(1)
	}
	log.Println("---------------set route over ------------------")
}
