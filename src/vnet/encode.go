package vnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"packet"
)

type vnetPkt struct {
	pktType byte
	pktLen  uint16
	payload []byte
}

const (
	PktHeaderSize = 6
	UserData      = byte(0x01)
	HearbeatReq   = byte(0x02)
	HearbeatRpl   = byte(0x03)
	FdbIdsMsg     = byte(0x04)
	MultiLinkData = byte(0x05)
	CryptoData    = byte(0x06)
)

type PktHeader struct {
	pktType  byte
	pktLen   uint16
	vid      uint16
	pktCrypt byte
}

func init() {
	pktHandlersReg()
}

var pktHandles map[byte]func(c *Client, cr io.Reader, pb *packet.PktBuf, ph *PktHeader) (n int, err error)

func pktHandlersReg() {
	pktHandles = make(map[byte]func(c *Client, cr io.Reader, pb *packet.PktBuf, ph *PktHeader) (n int, err error), 5)
	pktHandles[UserData] = UserDataPktHandle

	pktHandles[HearbeatReq] = HearbeatPktHandle
	pktHandles[HearbeatRpl] = HearbeatPktHandle

	pktHandles[FdbIdsMsg] = FdbIdsMsgPktHandle
}

func assembleUserPkt(data []byte) ([]byte, error) {
	return assemblePkt(UserData, data)
}

func assembleUserPktHead(headBuf []byte, dataLen int, vid int) error {
	return assemblePktHead(UserData, headBuf, dataLen, vid)
}

func assembleHbReq(data []byte) ([]byte, error) {
	return assemblePkt(HearbeatReq, data)
}

func assembleHbReqHead(headBuf []byte, dataLen int) error {
	return assemblePktHead(HearbeatReq, headBuf, dataLen, 0)
}

func assembleHbRpl(data []byte) ([]byte, error) {
	return assemblePkt(HearbeatRpl, data)
}

func assembleHbRplHead(headBuf []byte, dataLen int) error {
	return assemblePktHead(HearbeatRpl, headBuf, dataLen, 0)
}

func assembleFdbIdsHead(headBuf []byte, dataLen int) error {
	return assemblePktHead(FdbIdsMsg, headBuf, dataLen, 0)
}

func assemblePkt(t byte, data []byte) ([]byte, error) {
	l := len(data)
	buf := bytes.NewBuffer(make([]byte, 0, PktHeaderSize+l))
	buf.WriteByte(t)
	binary.Write(buf, binary.BigEndian, uint16(l))
	buf.Write(data)
	if buf.Len() != PktHeaderSize+l {
		return nil, fmt.Errorf("dataLen %d ,HeadSize=%d,PktHeaderSize=%d,l=%d", buf.Len(), HeadSize, PktHeaderSize, l)
	}
	return buf.Bytes(), nil
}

func assemblePktHead(t byte, headBuf []byte, dataLen int, vid int) error {
	headBuf[0] = t
	binary.BigEndian.PutUint16(headBuf[1:], uint16(dataLen))
	binary.BigEndian.PutUint16(headBuf[3:], uint16(vid))
	headBuf[5] = 0 //CRY_AES256
	return nil
}

func parsePktHeader(data []byte, ph *PktHeader) {
	// ph := &PktHeader{}
	// buf := bytes.NewBuffer(data)
	// ph.pktType, _ = buf.ReadByte()
	// binary.Read(buf, binary.BigEndian, &ph.pktLen)
	// return ph

	ph.pktType = data[0]

	ph.pktLen = binary.BigEndian.Uint16(data[1:])
	ph.vid = binary.BigEndian.Uint16(data[3:])
	ph.pktCrypt = data[5]
}

func setCryptoType(data []byte, cryptoType byte) {
	data[5] = cryptoType
	// t := data[0]
	// data[0] = (cryptoType<<4 | t)
}
