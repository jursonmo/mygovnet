package vnet

import (
	"fmt"
	"log"
	"mylog"
	"packet"
	"reflect"
	"sync"
	"time"
)

type backupLink struct {
	c           *Client
	activeSlave *Client
	sync.Mutex
	backupChan    chan int
	backupClients []*Client
	recvQueue     chan *packet.PktBuf
	rxBytes       uint64
	txBytes       uint64
	rxDropBytes   uint64
	txDropBytes   uint64
}

var wg sync.WaitGroup

func (bl *backupLink) setClient(c *Client) {
	bl.c = c
}

func (bl *backupLink) Close() error {
	return nil
}

func (bl *backupLink) String() string {
	return "this is backupLink"
}

func (bl *backupLink) PutToRecvQueue(pkt *packet.PktBuf, slave *Client) {
	pkt.HoldPktBuf()
	if bl.activeSlave == slave {
		bl.recvQueue <- pkt
	}
}

func (bl *backupLink) Read(pb *packet.PktBuf) (n int, err error) {
	for pkt := range bl.recvQueue {
		ForwardPkt(bl.c, pkt)
		bl.rxBytes += uint64(pkt.GetDataLen())
		putPktBuf(pkt)
	}
	err = fmt.Errorf("%s read quit, never happen", bl.String())
	return
}

func (bl *backupLink) Write(pkt *packet.PktBuf) (n int, err error) {
	slave := bl.activeSlave
	if slave != nil {
		slave.PutPktToChan(pkt)
		n = int(pkt.GetDataLen())
		bl.txBytes += uint64(n)
	} else {
		bl.txDropBytes += uint64(pkt.GetDataLen())
	}
	return
}

func newbackupLink(backupNum int) *backupLink {
	return &backupLink{
		backupChan:    make(chan int, 1),
		backupClients: make([]*Client, backupNum),
		recvQueue:     make(chan *packet.PktBuf, *ChanSize),
	}
}

func NcAccessBackup(dialInfo interface{}, dialer Dialer) {
	dis := reflect.ValueOf(dialInfo)
	if dis.Kind() != reflect.Slice {
		panic("dis.Kind() != reflect.Slice")
	}

	ncsNum := dis.Len()
	bl := newbackupLink(ncsNum)
	blc := NewClient(bl)
	blc.valid = true
	blc.setCryptType(CryptType)

	ClientMasterAdd(blc)
	blc.JoinAllFdb()
	blc.Working()

	for i := 0; i < ncsNum; i++ {
		//nc := ncAccessAddrs[i]
		di := dis.Index(i).Interface()
		go blc.BackupClient(true, dialer, di)
	}
	blc.MasterWorking()
	ClientMasterDel(blc)
}

func (c *Client) addBackup(bc *Client) {
	bl, ok := c.cio.(*backupLink)
	if !ok {
		log.Panicf("%s, c.cio is not *backupLink", c.String())
	}

	bl.Lock()
	for i := 0; i < cap(bl.backupClients); i++ {
		if bl.backupClients[i] == nil {
			bl.backupClients[i] = bc
			break
		}
	}
	if len(bl.backupChan) == 0 {
		bl.backupChan <- 1
	}
	bc.master = c
	bl.Unlock()
}

func (c *Client) delBackup(bc *Client) {
	if bc.master != c {
		log.Panicf("bc.master != c, bc=%s, c=%s\n", bc.String(), c.String())
	}
	bl, ok := c.cio.(*backupLink)
	if !ok {
		log.Panicf("%s, c.cio is not *backupLink", c.String())
	}
	bl.Lock()
	for i := 0; i < cap(bl.backupClients); i++ {
		if bl.backupClients[i] == bc {
			bl.backupClients[i] = nil
			break
		}
	}
	bc.master = nil
	if bl.activeSlave == bc {
		bl.activeSlave = nil
		wg.Done()
	}
	bl.Unlock()
}

func (master *Client) BackupClient(isReconnet bool, dialer Dialer, ncInfo interface{}) {
	for {
		conn := dialer.Connect(ncInfo)
		slave, _ := CreateConnClient(conn)
		master.addBackup(slave)
		slave.Working()
		slave.reportFdbMsg()

		if !isReconnet {
			break
		}

		<-slave.reconnect
		master.delBackup(slave)
		close(slave.reconnect)
		if master.IsClose() {
			log.Printf("master:'%s' is closed,so slave:'%s' don't reconnect,but never happen\n", master.String(), slave.String())
			break
		}
		time.Sleep(time.Second * 2)
		mylog.Info("reconnecting %s\n", slave.String())
	}
}

func (c *Client) choseBackup() *Client {
	var bc *Client = nil
	var minDelay time.Duration = 1<<63 - 1 //time.maxDuration
	firstIndex := -1
	bl, ok := c.cio.(*backupLink)
	if !ok {
		log.Panicf("%s, c.cio is not *backupLink", c.String())
	}
	bl.Lock()
	for i := 0; i < cap(bl.backupClients); i++ {
		if bl.backupClients[i] != nil && !bl.backupClients[i].IsClose() {
			if firstIndex == -1 {
				firstIndex = i
			}
			c := bl.backupClients[i]
			delay := c.heartBeatDelayAvg()
			if delay != 0 && delay < minDelay {
				minDelay = delay
				bc = c
			}
			//break
		}
	}
	// if can't find a bc, chose first backup client
	if bc == nil && firstIndex != -1 {
		bc = bl.backupClients[firstIndex]
	}
	bl.Unlock()
	return bc
}

func (c *Client) MasterWorking() {
	bl, ok := c.cio.(*backupLink)
	if !ok {
		log.Panicf("%s, c.cio is not *backupLink", c.String())
	}
	for {
		// if bl.activeSlave != nil {
		// 	time.Sleep(time.Second)
		// 	continue
		// }
		mylog.Info("---------%s chosing Nc access ,chan len =%d\n", c.String(), len(bl.backupChan))
		bc := c.choseBackup()
		if bc == nil {
			mylog.Info("there is no availd Nc access standby\n")
			<-bl.backupChan
			continue
		}
		mylog.Info("\n---------------------chose Nc access :%s---------------------\n", bc.String())
		bc.Lock()
		if !bc.IsClose() {
			bl.activeSlave = bc
			wg.Add(1)
		}
		bc.Unlock()
		wg.Wait() //if bc.IsClose() and didn't wg.Add(1), here will not wait ; or wait for bc close and delBackup;
	}
	log.Panicf("master %s MasterWorking quit? never happen\n", c.String())
}
