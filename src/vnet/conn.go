package vnet

import (
	"bufio"
	"fmt"
	"io"
	"mylog"
	"net"
	"packet"

	"github.com/juju/ratelimit"
)

type vnetConn struct {
	conn net.Conn
	cr   *bufio.Reader
	cw   io.Writer
	c    *Client
}

func SetRateLimit(up, down int64) {
	*UpRateLimit = up
	*DownRateLimit = down
	mylog.Info("========SetRateLimit up=%d, down=%d ========\n", *UpRateLimit, *DownRateLimit)
}

func (vconn *vnetConn) setClient(c *Client) {
	vconn.c = c
}

func NewVnetConn(conn net.Conn) *vnetConn {
	var cr *bufio.Reader
	var cw io.Writer
	if *DownRateLimit != 0 {
		bk := ratelimit.NewBucketWithRate(float64(*DownRateLimit), int64(*DownRateLimit))
		rd := ratelimit.Reader(conn, bk)
		cr = bufio.NewReader(rd)
	} else {
		cr = bufio.NewReader(conn)
	}

	if *UpRateLimit != 0 {
		bk := ratelimit.NewBucketWithRate(float64(*UpRateLimit), int64(*UpRateLimit))
		cw = ratelimit.Writer(conn, bk)
	} else {
		cw = conn
	}

	setTcpSockOpt(conn)
	return &vnetConn{
		conn: conn,
		cr:   cr,
		cw:   cw,
	}
}

func (vconn *vnetConn) Read(pb *packet.PktBuf) (rn int, err error) {
	var ph PktHeader
	// if pb.GetDataLen() != 0 {
	// 	log.Panicf("pb.GetDataLen() =%d\n", pb.GetDataLen())
	// }
	pkt := pb.LoadAndUseBuf(PktHeaderSize)

	rn, err = io.ReadFull(vconn.cr, pkt)
	if err != nil {
		mylog.Error("\n ----PktHeaderSize ReadFull fail: %s, rn=%d, want=%d-----\n", err.Error(), rn, PktHeaderSize)
		return
	}

	parsePktHeader(pkt, &ph)
	if handlePkt, ok := pktHandles[ph.pktType]; ok {
		if rn, err = handlePkt(vconn.c, vconn.cr, pb, &ph); err != nil {
			return
		}
	}
	return
}

func (vc *vnetConn) Write(pb *packet.PktBuf) (n int, err error) {
	if vc.c.cryptType != 0 {
		ct := vc.c.cryptType
		block, ok := crypts[ct]
		if !ok {
			err = fmt.Errorf("vc.c.crypType =%d, not support\n", ct)
			return
		}
		userPkt := pb.LoadUserData()
		cryptLock.Lock()
		block.Encrypt(userPkt, userPkt)
		cryptLock.Unlock()
		setCryptoType(pb.LoadData(), ct)
	}
	return vc.cw.Write(pb.LoadData())
}

func (vc *vnetConn) Close() error {
	return vc.conn.Close()
}

func (vc *vnetConn) String() string {
	return fmt.Sprintf("%s-%s", vc.conn.LocalAddr().String(), vc.conn.RemoteAddr().String())
}

func (vc *vnetConn) RemoteAddr() string {
	return vc.conn.RemoteAddr().String()
}

func (vc *vnetConn) PutToRecvQueue(pb *packet.PktBuf, slave *Client) {
	return
}

func setTcpSockOpt(conn net.Conn) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		/*
			if err := setTCPUserTimeout(tcpConn, time.Second * TcpUserTimeout); err != nil {
				log.Printf("setTCPUserTimeout fail, err=%s\n", err.Error())
			}
		*/
		/*
			kaConn, err := tcpkeepalive.EnableKeepAlive(tcpConn)
			if err != nil {
				log.Println(tcpConn.RemoteAddr(), err)
			} else {
				kaConn.SetKeepAliveIdle(time.Duration(KeepAliveIdle) * time.Second)
				kaConn.SetKeepAliveCount(KeepAliveCnt)
				kaConn.SetKeepAliveInterval(time.Duration(KeepAliveIntv) * time.Second)
			}
		*/
	}
}
