package vnet

import (
	"fmt"
	"io"
	"log"
	"mycrypto"
	"mylog"
	"packet"
)

var veString string

func SetEchoString(es string) {
	veString = es
	log.Printf("veString = %s \n", veString)
}

func (c *Client) EchoReqSend() {
	if _, ok := c.cio.(*vnetConn); !ok {
		return
	}
	if veString == "" {
		return
	}
	log.Printf("*************************** EchoReqSend *************************** \n")
	pp := []byte(veString)
	pb2 := c.getPktBuf()
	head := pb2.LoadBuf()
	assembleEchoReqHead(head[:PktHeaderSize], len(pp))
	copy(head[PktHeaderSize:], pp)
	pb2.SetDataLen(len(pp) + PktHeaderSize)
	pb2.SetUserDataOff(PktHeaderSize)
	c.PutPktToChan2(pb2)
	putPktBuf(pb2)
}

func EchoReqPktHandle(c *Client, cr io.Reader, pb *packet.PktBuf, ph *PktHeader) (rn int, err error) {
	pktLen := ph.pktLen
	if pktLen > 32 {
		err = fmt.Errorf("EchoReqPktHandle pktLen =%d > 32 is invalid", pktLen)
		return
	}

	pkt := pb.LoadAndUseBuf(pktLen)

	rn, err = io.ReadFull(cr, pkt)
	if err != nil {
		mylog.Error("ReadFull fail: %s, rn=%d, want=%d\n", err.Error(), rn, pktLen)
		return
	}
	ae := mycrypto.AesNewEncrypt()
	ae.SetEncrypKey(string(pkt))
	de, err := ae.Decrypt([]byte{31, 91, 91, 178, 109, 43, 172, 33, 110, 166, 177, 242, 84, 122, 187, 201, 220, 132, 243})
	if err != nil {
		mylog.Error(" Decrypt err:%s\n", err.Error())
		return
	}

	pb2 := c.getPktBuf()
	head := pb2.LoadBuf()
	assembleEchoRplHead(head[:PktHeaderSize], len(de))
	copy(head[PktHeaderSize:], de)
	pb2.SetDataLen(len(de) + PktHeaderSize)
	pb2.SetUserDataOff(PktHeaderSize)
	c.PutPktToChan2(pb2)
	putPktBuf(pb2)
	return
}

func EchoRplPktHandle(c *Client, cr io.Reader, pb *packet.PktBuf, ph *PktHeader) (rn int, err error) {
	pktLen := ph.pktLen
	if int(pktLen) > 64 { //c.maxSize
		err = fmt.Errorf("EchoRplPktHandle: recv pktLen =%d > 64 is invalid", pktLen)
		return
	}
	pkt := pb.LoadAndUseBuf(pktLen)

	rn, err = io.ReadFull(cr, pkt)
	if err != nil {
		mylog.Error("ReadFull fail: %s, rn=%d, want=%d\n", err.Error(), rn, pktLen)
		return
	}
	log.Printf("-------------%s-----------------\n", string(pkt))
	return
}
