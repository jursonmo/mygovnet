package packet

import (
	"encoding/binary"
	"net"
)

type Operation uint16

// Operation constants which indicate an ARP request or reply.
const (
	OperationRequest Operation = 1
	OperationReply   Operation = 2
	ArpPacketSize              = 28
)

func IsArpRelpy(b []byte) bool {
	if len(b) < ArpPacketSize+EtherSize {
		return false
	}
	ether := TranEther(b)
	if !ether.IsArp() {
		return false
	}
	arpPkt := b[EtherSize:]
	return OperationReply == Operation(binary.BigEndian.Uint16(arpPkt[6:8]))
}

type ArpPacket struct {
	// HardwareType specifies an IANA-assigned hardware type, as described
	// in RFC 826.
	HardwareType uint16

	// ProtocolType specifies the internetwork protocol for which the ARP
	// request is intended.  Typically, this is the IPv4 EtherType.
	ProtocolType uint16

	// HardwareAddrLength specifies the length of the sender and target
	// hardware addresses included in a Packet.
	HardwareAddrLength uint8

	// IPLength specifies the length of the sender and target IPv4 addresses
	// included in a Packet.
	IPLength uint8

	// Operation specifies the ARP operation being performed, such as request
	// or reply.
	Operation Operation

	// SenderHardwareAddr specifies the hardware address of the sender of this
	// Packet.
	SenderHardwareAddr net.HardwareAddr

	// SenderIP specifies the IPv4 address of the sender of this Packet.
	SenderIP net.IP

	// TargetHardwareAddr specifies the hardware address of the target of this
	// Packet.
	TargetHardwareAddr net.HardwareAddr

	// TargetIP specifies the IPv4 address of the target of this Packet.
	TargetIP net.IP
}
