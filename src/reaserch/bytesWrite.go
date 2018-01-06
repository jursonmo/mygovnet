package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func main() {
	pkt := bytes.NewBuffer(make([]byte, 0, 5))
	//pkt := bytes.NewBuffer(make([]byte, 5))
	fmt.Printf("len=%d, cap=%d\n", len(pkt.Bytes()), cap(pkt.Bytes()))
	fmt.Printf("len=%d, cap=%d\n", pkt.Len(), pkt.Cap())
	binary.Write(pkt, binary.BigEndian, uint16(0))
	pkt.Write([]byte{0x01})
	pkt.Write([]byte{0x02})
	pkt.Write([]byte{0x03, 0x04})
	fmt.Printf("len=%d, cap=%d\n", pkt.Len(), pkt.Cap())
	fmt.Println(pkt.Bytes())

	binary.BigEndian.PutUint16(pkt.Bytes(), uint16(pkt.Len()))
	fmt.Println(pkt.Bytes())

}
