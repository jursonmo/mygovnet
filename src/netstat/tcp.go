package netstat

import "encoding/binary"

const (
	srcPort     = 0
	dstPort     = 2
	seqNum      = 4
	ackNum      = 8
	dataOffset  = 12
	tcpFlags    = 13
	winSize     = 14
	tcpChecksum = 16
	urgentPtr   = 18
)

// Flags that may be set in a TCP segment.
const (
	TCPFlagFin = 1 << iota
	TCPFlagSyn
	TCPFlagRst
	TCPFlagPsh
	TCPFlagAck
	TCPFlagUrg
)

// TCPFields contains the fields of a TCP packet. It is used to describe the
// fields of a packet that needs to be encoded.
type TCPFields struct {
	// SrcPort is the "source port" field of a TCP packet.
	SrcPort uint16

	// DstPort is the "destination port" field of a TCP packet.
	DstPort uint16

	// SeqNum is the "sequence number" field of a TCP packet.
	SeqNum uint32

	// AckNum is the "acknowledgement number" field of a TCP packet.
	AckNum uint32

	// DataOffset is the "data offset" field of a TCP packet.
	DataOffset uint8

	// Flags is the "flags" field of a TCP packet.
	Flags uint8

	// WindowSize is the "window size" field of a TCP packet.
	WindowSize uint16

	// Checksum is the "checksum" field of a TCP packet.
	Checksum uint16

	// UrgentPointer is the "urgent pointer" field of a TCP packet.
	UrgentPointer uint16
}

// TCP represents a TCP header stored in a byte array.
type TCP []byte

const (
	// TCPMinimumSize is the minimum size of a valid TCP packet.
	TCPMinimumSize = 20

	// TCPProtocolNumber is TCP's transport protocol number.
	//TCPProtocolNumber tcpip.TransportProtocolNumber = 6
)

// SourcePort returns the "source port" field of the tcp header.
func (b TCP) SourcePort() uint16 {
	return binary.BigEndian.Uint16(b[srcPort:])
}

// DestinationPort returns the "destination port" field of the tcp header.
func (b TCP) DestinationPort() uint16 {
	return binary.BigEndian.Uint16(b[dstPort:])
}

// Flags returns the flags field of the tcp header.
func (b TCP) Flags() uint8 {
	return b[tcpFlags]
}

func (b TCP) IsTCPFlagSyn() bool {
	return b.Flags() == TCPFlagSyn
}

func (b TCP) IsTCPFlagFin() bool {
	return b.Flags() == TCPFlagFin
}

func (b TCP) IsTCPFlagRst() bool {
	return b.Flags() == TCPFlagRst
}

func IsSyn(tcpFlag uint8) bool {
	return tcpFlag == TCPFlagSyn
}

func IsAck(tcpFlag uint8) bool {
	return tcpFlag == TCPFlagAck
}

func IsSynAck(tcpFlag uint8) bool {
	return tcpFlag&TCPFlagSyn != 0 && tcpFlag&TCPFlagAck != 0
}
