package vnet

import (
	"flag"
	"fmt"
	"log"
	"mylog"
	"os"
	"os/exec"
	"packet"
	"strconv"
	"sync"

	"github.com/lab11/go-tuntap/tuntap"
)

var (
	checkTunPkt bool
	obcDevSlot  [DevSlotMax]int
	DevSlotLock sync.Mutex
	Vtuns       []TunConf
	Br          = flag.String("br", "", " add tun/tap to bridge")
	TunType     = flag.Int("tuntype", int(tuntap.DevTap), " type, 1 means tap and 0 means tun")
	TunName     = flag.String("tundev", "tap0", " tun dev name")
	Ipstr       = flag.String("ipstr", "", "set tun/tap or br ip address")
	Mac         = flag.String("mac", "", "set tun/tap or br mac address")
	Vid         = flag.Int("vid", 0, "set tun/tap vid")
	//HQMode      = flag.Bool("hq", false, "HQ mode ,default false")
)

type TunConf struct {
	Br      string `toml:"br"`
	TunType int    `toml:"tuntype"`
	TunName string `toml:"tundev"`
	Ipstr   string `toml:"ipstr"`
	Mac     string `toml:"mac"`
	Vid     int    `toml:"vid"`
}

const (
	DevSlotMax = 100
)

func getDevId() int {
	DevSlotLock.Lock()
	defer DevSlotLock.Unlock()
	for i := 0; i < DevSlotMax; i++ {
		if obcDevSlot[i] == 0 {
			obcDevSlot[i] = 1
			return i
		}
	}
	log.Fatalln("can not find a vaild DevId\n")
	return -1
}

func putDevId(devId int) {
	if devId >= DevSlotMax || devId < 0 {
		log.Panicf("devId =%d is out range 0-%d\n", devId, DevSlotMax)
	}
	DevSlotLock.Lock()
	obcDevSlot[devId] = 0
	DevSlotLock.Unlock()
}

type mytun struct {
	c       *Client
	tund    *tuntap.Interface
	devType int
	devId   int
	vid     int
}

func SetCheckTunPkt(b bool) {
	checkTunPkt = b
	log.Printf("======SetCheckTunPkt:  checkTunPkt =%v ===========\n", checkTunPkt)
}

func (tun *mytun) setClient(c *Client) {
	tun.c = c
}

func NewTun(devType int, vid int) *mytun {
	return &mytun{
		devType: devType,
		devId:   getDevId(),
		vid:     vid,
	}
}

func (tun *mytun) Name() string {
	return tun.tund.Name()
}

func OpenTun(br string, tunname string, tuntype int, ipstr string, mac string, vid int, auto bool) (tun *mytun, err error) {
	tun = NewTun(tuntype, vid)
	if auto {
		tunname = tunname + strconv.Itoa(tun.devId)
		// tunname = tunname + fmt.Sprintf("%d", tun.devId)
	}
	mylog.Info("create dev :%s ,(devId:%d), *tuntype=%d\n", tunname, tun.devId, tuntype)

	tun.tund, err = tuntap.Open(tunname, tuntap.DevKind(tuntype), false)
	if err != nil {
		mylog.Error("tun/tap open err:%s, tunname = %s \n", err.Error(), tunname)
		return nil, err
	}

	confs := fmt.Sprintf("ifconfig %s up\n", tunname)
	if br != "" { //must be tap
		if tuntype != int(tuntap.DevTap) {
			log.Panicf("br=%s can't not addif tuntype=%d(tap:%d,tun:%d)\n", br, tuntype, int(tuntap.DevTap), int(tuntap.DevTun))
		}
		confs += fmt.Sprintf("brctl addbr %s\n", br)
		confs += fmt.Sprintf("brctl addif %s %s\n", br, tunname)
		if ipstr != "" {
			confs += fmt.Sprintf("ifconfig %s %s\n", br, ipstr)
		}
		if mac != "" {
			confs += fmt.Sprintf("ifconfig %s hw ether %s\n", br, mac)
		}
	} else { // maybe tun or tap
		if ipstr != "" {
			confs += fmt.Sprintf("ifconfig %s %s\n", tunname, ipstr)
		}
		if mac != "" && tuntype == int(tuntap.DevTap) {
			confs += fmt.Sprintf("ifconfig %s hw ether %s\n", tunname, mac)
		}
	}

	confs += fmt.Sprintf("ifconfig %s txqueuelen 5000\n", tunname)
	err = exec.Command("sh", "-c", confs).Run()
	if err != nil {
		mylog.Error("open err:%s, confs = %s \n", err.Error(), confs)
		return nil, err
	}
	//l3 ec
	if br == "" { //tuntype == int(tuntap.DevTap)
		setRoute()
	}

	cmd := fmt.Sprintf(`./vnetDevUp.sh %s`, tunname)
	out, e := exec.Command("sh", "-c", cmd).CombinedOutput()
	if e != nil {
		mylog.Warning("open err:%s,out=%s\n", e.Error(), string(out))
	}
	log.Printf("================tun dev:%s open successfully==========\n", tun.tund.Name())
	return
}

func (tun *mytun) Read(pb *packet.PktBuf) (n int, err error) {
	var inpkt *tuntap.Packet
	n = 0
	buf := pb.LoadBuf()
ReRead:
	inpkt, err = tun.tund.ReadPacket2(buf[PktHeaderSize:])
	//inpkt, err := tun.tund.ReadPacket()
	if err != nil {
		log.Printf("==============%s ReadPacket error:%s===", tun.Name(), err.Error())
		log.Panicln(err)
		return
	}
	n = len(inpkt.Packet)

	if tun.devType == int(tuntap.DevTap) {
		if n < 42 || n > 1514 {
			log.Printf("======tun read len=%d out of range =======\n", n)
			//err = errors.New("invaild pkt of vnetTun")
			//return
			goto ReRead
		}
		ether := packet.TranEther(inpkt.Packet)
		if ether.IsBroadcast() && ether.IsArp() {
			mylog.Info("---------arp broadcast from %s, vid=%d----------", tun.Name(), tun.vid)
			log.Printf("dst mac :%s", ether.DstMac.String())
			log.Printf("src mac :%s", ether.SrcMac.String())
		}
		if !ether.IsArp() && !ether.IsIpPtk() {
			//mylog.Warning(" not arp ,and not ip packet, ether type =0x%0x%0x ===============\n", ether.Proto[0], ether.Proto[1])
			goto ReRead
			//err = errors.New("vnetFilter")
			//return
		}
		if *DebugEn && ether.IsIpPtk() {
			iphdr, err := packet.ParseIPHeader(inpkt.Packet[packet.EtherSize:])
			if err != nil {
				log.Printf("ParseIPHeader err: %s\n", err.Error())
			}
			log.Println("tun read ", iphdr.String())
		}
	} else {
		if n < 28 || n > 1500 {
			log.Printf("======tun read len=%d out of range =======\n", n)
			//err = errors.New("invaild pkt of vnetTun")
			//return
			goto ReRead
		}
	}

	// if *DebugEn {
	// 	ShowPktInfo(inpkt.Packet, tun.Name())
	// }

	// if tun.tund.Meta() {
	// 	copy(buf[PktHeaderSize:], inpkt.Packet[:n])
	// }

	assembleUserPktHead(buf[:PktHeaderSize], n, tun.vid)
	n += PktHeaderSize

	pb.SetPktType(UserData)
	pb.SetPktVid(uint16(tun.vid))
	pb.SetUserDataOff(PktHeaderSize)
	pb.SetDataLen(n)
	pb.SetOutBound(true)
	//pb.StoreData(buf[:rn])
	ForwardPkt(tun.c, pb)

	return
}
func (tun *mytun) Write(pb *packet.PktBuf) (n int, err error) {
	userData := pb.LoadUserData()

	if checkTunPkt {
		if tun.devType == int(tuntap.DevTap) {
			ether := packet.TranEther(userData)
			if !ether.IsArp() && !ether.IsIpPtk() {
				mylog.Warning("====== not arp and not ip packet, ether proto=%d=======\n", ether.GetProto())
				return 0, nil
			}
		}
	}

	inpkt := &tuntap.Packet{Packet: userData}
	err = tun.tund.WritePacket(inpkt)
	if err != nil {
		log.Panicln(err)
		return 0, err
	}
	n = len(userData)
	return
}

func RunCmd(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		return string(out), err
	}

	return string(out), nil
}

func (tun *mytun) Close() error {
	mylog.Notice("=====close dev =%s \n", tun.Name())
	// cmd := fmt.Sprintf(`ip link delete %s`, tun.Name())
	ipPath, err := exec.LookPath("ip")
	if err != nil {
		log.Fatal("lookpath error: ", err)
	}

	//log.Printf("tun.Name() = [%x]", tun.Name())
	out, err := RunCmd(ipPath, "link", "delete", tun.Name())
	if err != nil {
		mylog.Error("cmd run err: %s, cmd＝ip link delete %s\n", err.Error(), tun.Name())
		log.Println(out)
		os.Exit(1)
	}
	mylog.Info("ip link delete %s over\n", tun.Name()) //route will delelte

	cmd := fmt.Sprintf(`./vnetDevDown.sh %s`, tun.Name())
	output, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		mylog.Error("open err:%s,out=%s\n", err.Error(), string(output))
	}

	putDevId(tun.devId)
	delete(tcMap, tun.Name())
	return tun.tund.Close()
}

func (tun *mytun) String() string {
	return fmt.Sprintf("tun dev name=%s,type=%d, id=%d, vlanid=%d", tun.Name(), tun.devType, tun.devId, tun.vid)
}

func (tun *mytun) getPktRange() (int, int) {
	if tun.devType == int(tuntap.DevTap) {
		return L2PktMinSize, L2PktMaxSize
	} else {
		return L2PktMinSize - packet.EtherSize, L2PktMaxSize - packet.EtherSize
	}
}

func (tun *mytun) PutToRecvQueue(pb *packet.PktBuf, slave *Client) {
	return
}
