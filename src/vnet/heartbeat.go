package vnet

import (
	"encoding/binary"
	"mylog"
	"strings"
	"sync/atomic"
	"time"
)

const (
	HBRequest   = 0
	HBReply     = 1
	HearBeatReq = "HeartBeatReq" //len = 12
	HearBeatRpl = "HeartBeatRpl" //len = 12
	HBIDSize    = 2              //uint16
	HearBeatLen = 14             // 12+HBIDSize
	HBTimeout   = 60             //second
)

var (
	HeartbeatIdle = HBTimeout
	HeartbeatCnt  = 3
	HeartbeatIntv = 5
)

type heartBeat struct {
	hbTimer    *time.Timer
	hbID       uint16
	hbReqTime  time.Time
	hbDelay    time.Duration
	hbDelaySum time.Duration
	hbDelayAvg time.Duration
}

func SetHeartbeat(idle, count, intv int) {
	if idle <= 0 || count <= 0 || intv <= 0 {
		mylog.Error("SetHeartbeat error: idle=%d, count=%d, intv=%d\n", idle, count, intv)
		return
	}
	mylog.Info("SetHeartbeat : idle=%d, count=%d, intv=%d\n", idle, count, intv)
	HeartbeatIdle = idle
	HeartbeatCnt = count
	HeartbeatIntv = intv
}

func (c *Client) hbTimerReset(d time.Duration) {
	if c.hb != nil {
		if c.hb.hbTimer != nil {
			c.hb.hbTimer.Reset(d)
		}
	}
}

func (c *Client) heartBeatID() uint16 {
	if c.hb != nil {
		return c.hb.hbID
	}
	return 0
}

func (c *Client) heartBeatUpdate() {
	if c.hb != nil {
		c.hb.hbID += 1
		if c.hb.hbID == 0 {
			c.hb.hbID = 1
			c.hb.hbDelaySum = 0
		}
		c.hb.hbReqTime = time.Now()
	}
}

func (c *Client) heartBeatDelayCalc() {
	if c.hb != nil {
		c.hb.hbDelay = time.Now().Sub(c.hb.hbReqTime)
		c.hb.hbDelaySum += c.hb.hbDelay
		if c.hb.hbID != 0 {
			c.hb.hbDelayAvg = c.hb.hbDelaySum / time.Duration(c.hb.hbID)
		}
	}
}

//get last delay
func (c *Client) heartBeatDelay() time.Duration {
	if c.hb != nil {
		return c.hb.hbDelay / time.Millisecond
	}
	return 0
}

func (c *Client) heartBeatDelayAvg() time.Duration {
	if c.hb != nil {
		return c.hb.hbDelayAvg
	}
	return 0
}

func (c *Client) checkHeartBeat(pkt []byte) bool {
	id := binary.BigEndian.Uint16(pkt[:HBIDSize])
	if strings.Compare(string(pkt[HBIDSize:]), HearBeatReq) == 0 {
		mylog.Info("recv a heartbeat request(id:%d) from %s \n", id, c.String())
		//send heartbeat reply
		c.sendHeartBeat(HBReply, id)
		return true
	}
	if strings.Compare(string(pkt[HBIDSize:]), HearBeatRpl) == 0 {
		if c.hb == nil {
			mylog.Error("=======%s c.hb == nil ===========\n", c.String())
			return false
		}
		//c.rx_bytes += uint64(HearBeatLen)
		atomic.AddUint64(&c.rx_bytes, uint64(HearBeatLen))
		if id == c.heartBeatID() {
			c.valid = true
			c.heartBeatDelayCalc()
			mylog.Info("ok,recv a heartbeat reply id:%d on %s, delay=%d(%d ms), heartBeatDelayAvg()=%d(%d ms)\n", id, c.String(),
				c.hb.hbDelay, c.hb.hbDelay/time.Millisecond, c.heartBeatDelayAvg(), c.heartBeatDelayAvg()/time.Millisecond)
		} else {
			mylog.Notice("Notice ,%s recv a heartbeat reply id:%d ,but c.hb.hbID=%d\n", c.String(), id, c.hb.hbID)
			// if first heartbeat is fail, reture false and close client
			if c.heartBeatID() == 1 {
				return false
			}
		}
		//c.hbTimer.Reset(time.Second * time.Duration(HeartbeatIdle))
		return true
	}
	mylog.Error("========hb content:%s ,len=%d===========\n", string(pkt[HBIDSize:]), len((pkt[HBIDSize:])))
	return false
}

func (c *Client) HeartBeat() {
	if _, ok := c.cio.(*vnetConn); ok {
		c.hb = &heartBeat{} //TODO, should do it advance
		timeout_count := 0
		c.hb.hbTimer = time.NewTimer(time.Second * 5)
		defer c.Reconnect()
		defer c.hb.hbTimer.Stop()

		//send hb to checkdelay at beginning
		time.Sleep(time.Millisecond * 10)
		c.sendHeartBeat(HBRequest, 0)
		for {
			if c.IsClose() {
				mylog.Notice(" %s is closed, HeartBeat quit\n", c.String())
				return
			}
			rx := c.rx_bytes
			mylog.Info("%s HeartBeat wait to timer up timeout_count =%d, c.rx_bytes=%d, valid=%v,cryptType=%d\n", c.String(), timeout_count, c.rx_bytes, c.valid, c.cryptType)
			<-c.hb.hbTimer.C

			if c.IsClose() || !c.valid {
				mylog.Notice("==== %s is closed(%v), valid=%v, HeartBeat quit ====\n", c.String(), c.IsClose(), c.valid)
				return
			}

			if rx == c.rx_bytes {
				if timeout_count >= HeartbeatCnt {
					mylog.Warning("=== %s HeartBeat quit:  timeout_count =%d, HeartbeatCnt=%d, rx =%d, c.rx_bytes =%d =====\n", c.String(), timeout_count, HeartbeatCnt, rx, c.rx_bytes)
					return
				}
				//TODO send heartbeat requst packet
				mylog.Info("%d Second timeout, need to send HeartBeat request to %s, rx =%d, c.rx_bytes =%d\n", HeartbeatIdle, c.String(), rx, c.rx_bytes)
				c.sendHeartBeat(HBRequest, 0)
				c.hbTimerReset(time.Second * time.Duration(HeartbeatIntv))
				timeout_count++
			} else {
				mylog.Debug(" have received some pkt, no need to send HeartBeat to %s, rx =%d, c.rx_bytes =%d\n", c.String(), rx, c.rx_bytes)
				c.hbTimerReset(time.Second * time.Duration(HeartbeatIdle))
				timeout_count = 0
			}
		}
	}

	if _, ok := c.cio.(*mytun); ok {
		//TODO
	}
}

func (c *Client) sendHeartBeat(hbType int, hbID uint16) {
	var sendstring string
	pb := c.getPktBuf()
	hb := pb.LoadBuf()
	if hbType == HBRequest {
		sendstring = "request"
		c.heartBeatUpdate()
		hbID = c.heartBeatID()
		assembleHbReqHead(hb[:PktHeaderSize], len(HearBeatReq)+HBIDSize)
		binary.BigEndian.PutUint16(hb[PktHeaderSize:], hbID)
		copy(hb[PktHeaderSize+HBIDSize:], []byte(HearBeatReq))
		pb.StoreData(hb[:PktHeaderSize+len(HearBeatReq)+HBIDSize])
	} else {
		sendstring = "reply"
		assembleHbRplHead(hb[:PktHeaderSize], len(HearBeatRpl)+HBIDSize)
		binary.BigEndian.PutUint16(hb[PktHeaderSize:], hbID)
		copy(hb[PktHeaderSize+HBIDSize:], []byte(HearBeatRpl))
		pb.StoreData(hb[:PktHeaderSize+len(HearBeatRpl)+HBIDSize])
	}
	pb.SetUserDataOff(PktHeaderSize)
	c.PutPktToChan2(pb)
	putPktBuf(pb)
	mylog.Info("sending HeartBeat(id:%d)  %s for %s ok \n", hbID, sendstring, c.String())
}
