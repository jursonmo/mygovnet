package vnet

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"netstat"
	"os"
	"packet"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mylog"

	"github.com/lab11/go-tuntap/tuntap"
)

const (
	HeadSize = 2

	KeepAliveIdle = 60
	KeepAliveCnt  = 3
	KeepAliveIntv = 5

	L2PktMinSize = 42
	L2PktMaxSize = 1514
)

var (
	tcMap    map[string]*Client
	ChanSize = flag.Int("chanSize", 1024, "chan Size")
	BindTun  = flag.Bool("bindtun", true, " conn bind tun")

	UpRateLimit   = flag.Int64("uprate", 0, "UpRateLimit, 0 means no limit")
	DownRateLimit = flag.Int64("downrate", 0, "DownRateLimit, 0 means no limit")
	//Vids          = flag.String("vids", "", "support vids")
	Vids []int
)

type Client struct {
	cio       VnetIO
	pktchan   chan *packet.PktBuf
	reconnect chan bool
	isClosed  bool
	p2pFwd    bool
	peer      *Client
	master    *Client
	sync.RWMutex
	rx_bytes      uint64
	tx_bytes      uint64
	last_rx_bytes uint64
	last_tx_bytes uint64
	minSize       int
	maxSize       int
	hb            *heartBeat
	fdbPortId     int
	valid         bool
	stats         Stats
	pbp           *sync.Pool
	isClient      bool
	fdbJoined     map[int]fdbPort
	cryptType     byte
}

var ClientMasterLock sync.Mutex
var ClientMaster map[string]*Client
var VnetStats map[string]*Stats

type Stats struct {
	NodeId           string
	AccessRemoteHost string
	SendSpd          uint64
	ReceiveSpd       uint64
}

type VnetIO interface {
	Write(pb *packet.PktBuf) (n int, err error)
	Read(pb *packet.PktBuf) (n int, err error)
	//io.Writer
	//io.Reader
	Close() error
	String() string
	setClient(c *Client)
	PutToRecvQueue(pkt *packet.PktBuf, slave *Client)
}

func init() {
	ClientMaster = make(map[string]*Client)
	tcMap = make(map[string]*Client)
	VnetStats = make(map[string]*Stats)
	go vnetRoute()
}

func NewClient(cio VnetIO) *Client {
	c := &Client{
		cio:       cio,
		pktchan:   make(chan *packet.PktBuf, *ChanSize),
		reconnect: make(chan bool, 1),
		isClosed:  false,
		p2pFwd:    false,
		minSize:   L2PktMinSize,
		maxSize:   L2PktMaxSize,
		pbp:       NewPktBufPool(),
		fdbJoined: make(map[int]fdbPort),
	}
	c.cio.setClient(c)
	return c
}

func NewPktBufPool() *sync.Pool {
	return packet.NewPktBufPool()
}

func CreateConnClient(conn net.Conn) (*Client, error) {
	vc := NewVnetConn(conn)
	return NewClient(vc), nil
}

func CreateTunClient(auto bool) (*Client, error) {
	tun, err := OpenTun(*Br, *TunName, *TunType, *Ipstr, *Mac, *Vid, auto)
	if err != nil {
		return nil, err
	}
	ct := NewClient(tun)
	//tun don't need to check, just valid == true
	ct.valid = true
	return ct, nil
}

func (c *Client) setCryptType(ct byte) {
	c.cryptType = ct
}

func (c1 *Client) bindPeer(c2 *Client) {
	c1.peer = c2
}

func bindPairClient(c1, c2 *Client) {
	c1.p2pFwd = true
	c2.p2pFwd = true
	c1.peer = c2
	c2.peer = c1
	c1.setAcsSize()
	c2.setAcsSize()
}

func (c *Client) setAcsSize() {
	if t, ok := c.peer.cio.(*mytun); ok {
		min, max := t.getPktRange()
		c.minSize = min
		c.maxSize = max
		log.Printf("%s: range size, %d-%d\n", c.String(), c.minSize, c.maxSize)
		return
	}
}

func (c *Client) releasePeer() (peer *Client) {
	if c.peer == nil {
		return nil
	}
	peer = c.peer
	c.peer = nil
	if peer.peer == c {
		peer.peer = nil
	}
	return peer
}

func (c *Client) peerString() string {
	if c.peer == nil {
		return "nil"
	}
	return c.peer.String()
}

func ClientMasterAdd(c *Client) {
	ClientMasterLock.Lock()
	ClientMaster[c.String()] = c
	ClientMasterLock.Unlock()
}

func ClientMasterDel(c *Client) {
	ClientMasterLock.Lock()
	delete(ClientMaster, c.String())
	ClientMasterLock.Unlock()
}

type Dialer interface {
	Connect(dialInfo interface{}) net.Conn
}

func ConnBindTun(dialInfo interface{}, dialer Dialer, tunconf TunConf) {
	*TunName = tunconf.TunName
	mylog.Info("----------set *TunName=%s -----\n", *TunName)
	tun, err := OpenTun(tunconf.Br, tunconf.TunName, tunconf.TunType, tunconf.Ipstr, tunconf.Mac, tunconf.Vid, false)
	if err != nil {
		log.Panicf("======OpenTunfail, tun=%s============\n", tunconf.TunName)
	}

	vtc := NewClient(tun)
	vtc.valid = true
	if netstat.IsEnable() {
		netstat.SetNetZone(tun.vid)
	}
	if err = vtc.joinFdbById(tun.vid); err != nil {
		panic(err)
	}
	vtc.Working()

	for {
		conn := dialer.Connect(dialInfo)
		vcc, _ := CreateConnClient(conn)

		mylog.Info("binding %s to %s", vtc.String(), vcc.String())
		bindPairClient(vcc, vtc)
		tcMap[vtc.cio.(*mytun).Name()] = vcc

		vcc.setCryptType(CryptType)
		vcc.Working()
		vcc.isClient = true

		if vcc.isClient {
			ClientMasterAdd(vcc)
			vcc.JoinAllFdb()
			vcc.reportFdbMsg()
			<-vcc.reconnect
			ClientMasterDel(vcc)
			close(vcc.reconnect)
			time.Sleep(time.Second * 2)
			mylog.Info("============reconnecting %s\n", vcc.String())
		}
	}
}

func ConnectNc(dialInfo interface{}, dialer Dialer) {
	dis := reflect.ValueOf(dialInfo)
	if dis.Kind() != reflect.Slice {
		panic("dis.Kind() != reflect.Slice")
	}
	for i := 0; i < dis.Len(); i++ {
		di := dis.Index(i).Interface()
		go func() {
			for {
				conn := dialer.Connect(di)
				HandleConn(conn, true)
			}
		}()
	}
}

func HandleConn(conn net.Conn, isClient bool) {
	vcc, _ := CreateConnClient(conn)
	if *BindTun {
		//if is socket client, tun dev name should auto generate
		/*
			autoGenTunName := !isClient
			vtc, err := CreateTunClient(autoGenTunName)
			if err != nil {
				mylog.Error("%s, so Close %s", err.Error(), vcc.String())
				vcc.Close()
				return
			}
			mylog.Info("binding %s to %s", vtc.String(), vcc.String())
			bindPairClient(vcc, vtc)
			tcMap[vtc.cio.(*mytun).Name()] = vcc
			vtc.Working()
		*/
	} else {
		// add client to fdb
		//vcc.JoinAllFdb()
		/*
			portId := fdb.JoinFwdPort(vcc)
			if portId == 0 {
				mylog.Error("======fdb.AddPort fail, portid=%d============\n", portId)
				vcc.Close()
				return
			}
			vcc.fdbPortId = portId
		*/
	}
	vcc.setCryptType(CryptType)
	vcc.Working()
	vcc.isClient = isClient

	//if is socket client, it means auto reconnect
	if vcc.isClient {
		ClientMasterAdd(vcc)
		vcc.JoinAllFdb()
		//TODO, send all fdb id; clientMaster连接成功后,无论是否设置了Vids，都会发fdbIdsMsg消息给上级,因为不知道上级什么情况
		vcc.reportFdbMsg()
		<-vcc.reconnect
		ClientMasterDel(vcc)
		close(vcc.reconnect)
		time.Sleep(time.Second * 2)
		mylog.Info("============reconnecting %s\n", vcc.String())
	} /* 无论是否设置了Vids，接受到客户端的连接都先不要加入fdb, 等fdbIdsMsg消息在加入，(无论客户端是否设置了Vids，都会发fdbIdsMsg消息过来)
		如果设置了Vids，只加入fdbIdsMsg消息交集的fdb,
		else {
		if *Vids != "" {
			//if have set Vids, so just add all fdbs, and don't care fdbIdsMsg
			//if not set Vids, wait for fdbIdsMsg to join
			vcc.JoinAllFdb()
		}
	} */
}

func HandleTuns(vtuns []TunConf) {
	setFlag := 0
	for _, tunconf := range vtuns {
		if tunconf.TunType != int(tuntap.DevTap) {
			mylog.Error("===== only create tap, tun=%s, type =%d============\n", tunconf.TunName, tunconf.TunType)
			continue
		}
		if setFlag == 0 && tunconf.Br == "" {
			setFlag = 1
			*TunName = tunconf.TunName
			mylog.Info("----------set *TunName=%s -----\n", *TunName)
		}
		tun, err := OpenTun(tunconf.Br, tunconf.TunName, tunconf.TunType, tunconf.Ipstr, tunconf.Mac, tunconf.Vid, false)
		if err != nil {
			log.Panicf("======OpenTunfail, tun=%s============\n", tunconf.TunName)
			continue
		}

		vtc := NewClient(tun)
		//tun don't need to check, just valid == true
		vtc.valid = true

		if err = vtc.joinFdbById(tun.vid); err != nil {
			panic(err)
		}
		if netstat.IsEnable() {
			netstat.SetNetZone(tun.vid)
		}
		vtc.Working()
	}
}

func (c *Client) Working() {
	go c.ReadForward()
	go c.WriteFromChan()
	go c.HeartBeat()
	go c.statstics()

	c.EchoReqSend()
}

func (c *Client) PutPktToChan(pkt *packet.PktBuf) {
	if c.valid {
		c.PutPktToChan2(pkt)
	}
}

func (c *Client) PutPktToChan2(pkt *packet.PktBuf) {
	if !c.IsClose() {
		pkt.HoldPktBuf()
		c.pktchan <- pkt
	}
}

func (c *Client) String() string {
	if c.cio == nil {
		return fmt.Sprintf("Client cio is unknown")
	}
	return c.cio.String()
}

func (c *Client) RemoteAddr() string {
	if c.cio == nil {
		return fmt.Sprintf("Client cio is unknown")
	}
	vconn, ok := c.cio.(*vnetConn)
	if !ok {
		return ""
	}
	return vconn.RemoteAddr()
}

func (c *Client) IsClose() bool {
	return c.isClosed
}

func (c *Client) Close() error {
	c.Lock()
	if !c.isClosed {
		mylog.Notice("%s  is  closing, peer is %s\n", c.String(), c.peerString())
		c.isClosed = true
		c.Unlock()

		c.hbTimerReset(time.Millisecond * 10)
		close(c.pktchan)
		c.cio.Close()
		c.quitAllFdb()
		//fdb.ReleaseFwdPort(c.fdbPortId)

		//if not set custom vid, and c isn't ClientMaster, updateMasterFdb and reportFdbMsg
		if len(Vids) == 0 && !c.isClient {
			updateMasterFdb()
		}

		// if peer := c.releasePeer(); peer != nil {
		// 	peer.Reconnect()
		// }

		mylog.Notice("%s is closed \n", c.String())
		return nil
	}
	c.Unlock()
	return errors.New("close a closed conn")
}

func (c *Client) Reconnect() {
	if err := c.Close(); err != nil {
		return
	}
	c.reconnect <- true
}

func UserDataPktHandle(c *Client, cr io.Reader, pb *packet.PktBuf, ph *PktHeader) (rn int, err error) {
	pktLen := int(ph.pktLen)
	if pktLen < c.minSize || pktLen > c.maxSize {
		mylog.Error("parase pktLen=%d out of range:%d-%d\n", pktLen, c.minSize, c.maxSize)
		err = fmt.Errorf("parase pktLen=%d out of range:%d-%d\n", pktLen, c.minSize, c.maxSize)
		return
	}
	return DataPktHandle(c, cr, pb, ph)
}

func DataPktHandle(c *Client, cr io.Reader, pb *packet.PktBuf, ph *PktHeader) (rn int, err error) {
	pktLen := ph.pktLen
	offset := pb.GetDataLen()
	//pkt := pb.LoadBuf()
	pkt := pb.LoadAndUseBuf(pktLen)

	rn, err = io.ReadFull(cr, pkt)
	if err != nil {
		mylog.Error("ReadFull fail: %s, rn=%d, want=%d\n", err.Error(), rn, pktLen)
		return
	}

	// if invalid, don't forward, and the client will be close by heartbeat, because c.rx_bytes == 0
	if !c.valid {
		return
	}

	if ph.pktCrypt != 0 {
		block, ok := crypts[ph.pktCrypt]
		if !ok {
			err = fmt.Errorf("crypType =%d, not support\n", ph.pktCrypt)
			return
		}
		cryptLock.Lock()
		block.Decrypt(pkt, pkt)
		cryptLock.Unlock()
		setCryptoType(pb.LoadData(), 0)
	}

	if *DebugEn {
		ShowPktInfo(pkt, "conn read")
	}

	pb.SetPktType(ph.pktType)
	pb.SetPktVid(ph.vid)
	pb.SetUserDataOff(int(offset))

	ForwardPkt(c, pb)
	rn = int(pb.GetDataLen())
	return
}

func HearbeatPktHandle(c *Client, cr io.Reader, pb *packet.PktBuf, ph *PktHeader) (rn int, err error) {
	pktLen := ph.pktLen
	if int(pktLen) > c.maxSize {
		err = fmt.Errorf("HearbeatPktHandle: recv pktLen =%d is invalid", pktLen)
		return
	}
	if ph.pktType != HearbeatReq && ph.pktType != HearbeatRpl {
		//panic("^_^")
		err = fmt.Errorf("HearbeatPktHandle: recv pktType =%d is invalid", ph.pktType)
		return
	}
	pkt := pb.LoadAndUseBuf(pktLen)

	rn, err = io.ReadFull(cr, pkt)
	if err != nil {
		mylog.Error("ReadFull fail: %s, rn=%d, want=%d\n", err.Error(), rn, pktLen)
		return
	}

	if ph.pktCrypt != 0 {
		log.Printf("HearbeatPktHandle :ph.pktCrypt=%d\n", ph.pktCrypt)
		block, ok := crypts[ph.pktCrypt]
		if !ok {
			err = fmt.Errorf("crypType =%d, not support\n", ph.pktCrypt)
			return
		}
		cryptLock.Lock()
		block.Decrypt(pkt, pkt)
		cryptLock.Unlock()
	}

	if !c.checkHeartBeat(pkt) {
		mylog.Error("==========type is heartbeat, but content isn't, never happen  ============\n")
	}
	return
}

func (c *Client) ReadForward() {
	defer func() { log.Printf("\n=====%s ReadForward quit=======\n", c.String()) }()
	defer c.Reconnect()
	var rn int
	var err error
	for {
		pb := c.getPktBuf()
		rn, err = c.cio.Read(pb)
		if err != nil {
			return
		}
		atomic.AddUint64(&c.rx_bytes, uint64(rn))
		putPktBuf(pb)
	}
}

func (c *Client) getPktBuf() *packet.PktBuf {
	return packet.GetPktFromPool(c.pbp)
}

func putPktBuf(pb *packet.PktBuf) {
	packet.PutPktToPool(pb)
}

func (c *Client) WriteFromChan() {
	var wn int
	var err error
	defer c.Reconnect()

	for pkt := range c.pktchan {
		wn, err = c.cio.Write(pkt)
		if err != nil {
			mylog.Error(" write to %s len=%d, err=%s\n", c.String(), wn, err.Error())
			return
		}
		atomic.AddUint64(&c.tx_bytes, uint64(wn))
		putPktBuf(pkt)
	}
	mylog.Notice(" %s WriteFromChan quit \n", c.String())
}

func (c *Client) FwdToPeer(pkt *packet.PktBuf) {
	if c.peer != nil {
		c.peer.PutPktToChan(pkt)
	}
}

func ForwardPkt(c *Client, pkt *packet.PktBuf) {
	if c.p2pFwd {
		c.FwdToPeer(pkt)
		if netstat.IsEnable() {
			netstat.NetStatPut(pkt)
		}
		return
	}
	if m := c.master; m != nil {
		m.cio.PutToRecvQueue(pkt, c)
		return
	}
	//TODO FDB FORWARD
	if fp, ok := c.GetFdbById(int(pkt.GetPktVid())); ok {
		if fp.fdb.Forward(c, pkt) && netstat.IsEnable() {
			netstat.NetStatPut(pkt)
		}
	}
}

func ShowBaseInfo() {
	//check info
	if *Br != "" { //must be tap
		if *TunType != int(tuntap.DevTap) {
			log.Panicf("bridge %s can't not addif tuntype=%d(tap:%d,tun:%d)\n", *Br, *TunType, int(tuntap.DevTap), int(tuntap.DevTun))
		}
		if *TunName == *Br {
			log.Panicf("*TunName =%s can't be same as *Br=%s\n", *TunName, *Br)
		}
		if *TunName == "" {
			*TunName = *Br + "_tap"
			log.Printf("TunName is nil, auto set TunName=%s by $br_tap\n", *TunName)
		}
	}
	log.Printf("Br=%s, TunName=%s, TunType=%d, Ipstr=%s, Mac=%s, BindTun=%v, ChanSize=%d, rate up=%d, down=%d\n", *Br, *TunName, *TunType, *Ipstr,
		*Mac, *BindTun, *ChanSize, *UpRateLimit, *DownRateLimit)
}

func fileExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func (c *Client) statstics() {
	idFileName := "/dev/shm/product_id"
	var txs, txCurrent, rxs, rxCurrent uint64
	vc, ok := c.cio.(*vnetConn)
	if !ok {
		return
	}
	stats := &c.stats
	stats.AccessRemoteHost = vc.conn.RemoteAddr().String()
	key := vc.String()
	VnetStats[key] = stats

	if !fileExist(idFileName) {
		idFileName = "/dev/shm/node_id"
	}
	buf, err := ioutil.ReadFile(idFileName)
	if err != nil {
		log.Println(err)
	} else {
		idstr := strings.TrimSpace(string(buf))
		stats.NodeId = strings.Trim(idstr, "\n")
	}

	for {
		time.Sleep(time.Second)
		if c.isClosed {
			delete(VnetStats, key)
			return
		}
		txCurrent = atomic.LoadUint64(&c.tx_bytes)
		stats.SendSpd = txCurrent - txs
		txs = txCurrent
		rxCurrent = atomic.LoadUint64(&c.rx_bytes)
		stats.ReceiveSpd = rxCurrent - rxs
		rxs = rxCurrent
	}
}
