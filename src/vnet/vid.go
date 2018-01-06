package vnet

import (
	"bytes"
	"encoding/binary"
	"fdb"
	"fmt"
	"io"
	"log"
	"mylog"
	"packet"
)

type fdbPort struct {
	fdb       *fdb.FDB
	fdbPortId int
}

func SetVids(vidset []int) {
	if len(vidset) == 0 {
		return
	}
	Vids = vidset
	mylog.Info("=====Vids: %v ==================\n", Vids)
	for _, vid := range Vids {
		fdb.NewFdb(vid)
	}
}

func (c *Client) JoinAllFdb() {
	ids := fdb.GetFdbIds()
	c.joinFdbByIds(ids)
}

func (c *Client) joinFdbById(id int) error {
	f := fdb.NewFdb(id)
	fp := fdbPort{}
	fp.fdb = f
	fp.fdbPortId = f.JoinFwdPort(c, !c.isClient)
	if fp.fdbPortId == 0 {
		mylog.Warning("fdb id=%d is full====================\n", id)
		return fmt.Errorf("fdb (id=%d) is full", id)
	}
	c.fdbJoined[id] = fp
	return nil
}

func (c *Client) joinFdbByIds(ids []int) {
	c.Lock()
	for _, id := range ids {
		c.joinFdbById(id)
	}
	c.Unlock()
}

func (c *Client) quitFdbByIds(ids []int) {
	c.Lock()
	for _, id := range ids {
		if fp, ok := c.fdbJoined[id]; ok {
			fp.fdb.ReleaseFwdPort(fp.fdbPortId, !c.isClient)
			delete(c.fdbJoined, id)
			if len(Vids) == 0 {
				fdb.TryToDelFdbById(id)
			}
		}
	}
	c.Unlock()
}

func (c *Client) quitAllFdb() {
	c.Lock()
	for fpid, fp := range c.fdbJoined {
		fp.fdb.ReleaseFwdPort(fp.fdbPortId, !c.isClient)
		delete(c.fdbJoined, fpid)
		if len(Vids) == 0 {
			fdb.TryToDelFdbById(fpid)
		}
	}
	c.fdbJoined = nil
	c.Unlock()
}

func (c *Client) GetFdbById(id int) (fdbPort, bool) {
	c.RLock()
	fp, ok := c.fdbJoined[id]
	c.RUnlock()
	return fp, ok
}

func (c *Client) GetFdbJoinIds() []int {
	var ids []int
	for id, _ := range c.fdbJoined {
		ids = append(ids, id)
	}
	return ids
}

func (c *Client) handleFdbIds(NewVids []int) {
	var joinIds, quitIds []int
	newVidMap := make(map[int]int, len(NewVids))
	for _, id := range NewVids {
		newVidMap[id] = id
	}
	c.RLock()
	for id, _ := range newVidMap {
		if _, ok := c.fdbJoined[id]; !ok {
			joinIds = append(joinIds, id)
		}
	}
	for id, _ := range c.fdbJoined {
		if _, ok := newVidMap[id]; !ok {
			quitIds = append(quitIds, id)
		}
	}
	c.RUnlock()
	c.quitFdbByIds(quitIds)
	c.joinFdbByIds(joinIds)
}

func (c *Client) handleFdbIdsMsg(msg []byte) error {
	var vid uint16
	var MsgVids, vids []int
	var err error
	if len(msg)%2 != 0 {
		return fmt.Errorf("len(msg)=%d,  %2 != 0 ", len(msg))
	}
	buf := bytes.NewBuffer(msg)
	for {
		err = binary.Read(buf, binary.BigEndian, &vid)
		if err != nil {
			break
		}
		MsgVids = append(MsgVids, int(vid))
	}
	log.Println("======================== handleFdbIdsMsg vids :", MsgVids)
	if len(Vids) > 0 {
		//check if MsgVids in the Vids
		for _, id := range MsgVids {
			if _, ok := fdb.GetFdbById(id); ok {
				vids = append(vids, id)
			}
		}
		log.Println("========================have set Vids, so finnal vids :", vids)
		c.handleFdbIds(vids)
	} else {
		c.handleFdbIds(MsgVids)
		//go updateMasterFdb()
		updateMasterFdb() //只有动态模式才需要更新MasterFdb
	}

	return nil
}

func (c *Client) reportFdbMsg() {
	master := c
	if c.master != nil {
		master = c.master
	}
	pb := c.getPktBuf()
	fis := pb.LoadBuf()

	buf := bytes.NewBuffer(fis[PktHeaderSize:PktHeaderSize])
	for fdbid, _ := range master.fdbJoined {
		binary.Write(buf, binary.BigEndian, uint16(fdbid))
	}
	assembleFdbIdsHead(fis[:PktHeaderSize], buf.Len())
	pb.SetDataLen(buf.Len() + PktHeaderSize)
	pb.SetUserDataOff(PktHeaderSize)
	c.PutPktToChan2(pb)
	putPktBuf(pb)
}

func updateMasterFdb() {
	var joinIds, quitIds []int
	FdbIds := fdb.GetFdbIds()
	newVidMap := make(map[int]int, len(FdbIds))
	for _, id := range FdbIds {
		newVidMap[id] = id
	}
	for _, c := range ClientMaster {
		c.RLock()
		if c.fdbJoined == nil {
			c.RUnlock()
			continue
		}
		for id, _ := range newVidMap {
			if _, ok := c.fdbJoined[id]; !ok {
				joinIds = append(joinIds, id)
			}
		}
		for id, _ := range c.fdbJoined {
			if _, ok := newVidMap[id]; !ok {
				quitIds = append(quitIds, id)
			}
		}
		c.RUnlock()
		c.quitFdbByIds(quitIds)
		c.joinFdbByIds(joinIds)
		if len(joinIds) > 0 || len(quitIds) > 0 {
			c.reportFdbMsg()
		}
	}
}

func FdbIdsMsgPktHandle(c *Client, cr io.Reader, pb *packet.PktBuf, ph *PktHeader) (rn int, err error) {
	pktLen := ph.pktLen
	if int(pktLen) > c.maxSize {
		err = fmt.Errorf("FdbIdsMsgPktHandle: recv pktLen =%d is invalid", pktLen)
		return
	}

	pkt := pb.LoadAndUseBuf(pktLen)

	rn, err = io.ReadFull(cr, pkt)
	if err != nil {
		mylog.Error("ReadFull fail: %s, rn=%d, want=%d\n", err.Error(), rn, pktLen)
		return
	}

	if ph.pktCrypt != 0 {
		log.Printf("ph.pktCrypt=%d\n", ph.pktCrypt)
		block, ok := crypts[ph.pktCrypt]
		if !ok {
			err = fmt.Errorf("crypType =%d, not support\n", ph.pktCrypt)
			return
		}
		cryptLock.Lock()
		block.Decrypt(pkt, pkt)
		cryptLock.Unlock()
	}

	if err = c.handleFdbIdsMsg(pkt); err != nil {
		mylog.Error("handleFdbIdsMsg: %s \n", err.Error())
		return
	}
	return
}
