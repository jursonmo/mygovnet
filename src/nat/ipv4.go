package nat

import "encoding/binary"

const (
	versIHL  = 0
	tos      = 1
	totalLen = 2
	id       = 4
	flagsFO  = 6
	ttl      = 8
	protocol = 9
	checksum = 10
	srcAddr  = 12
	dstAddr  = 16
)

// IPv4Fields contains the fields of an IPv4 packet. It is used to describe the
// fields of a packet that needs to be encoded.
type IPv4Fields struct {
	// IHL is the "internet header length" field of an IPv4 packet.
	IHL uint8

	// TOS is the "type of service" field of an IPv4 packet.
	TOS uint8

	// TotalLength is the "total length" field of an IPv4 packet.
	TotalLength uint16

	// ID is the "identification" field of an IPv4 packet.
	ID uint16

	// Flags is the "flags" field of an IPv4 packet.
	Flags uint8

	// FragmentOffset is the "fragment offset" field of an IPv4 packet.
	FragmentOffset uint16

	// TTL is the "time to live" field of an IPv4 packet.
	TTL uint8

	// Protocol is the "protocol" field of an IPv4 packet.
	Protocol uint8

	// Checksum is the "checksum" field of an IPv4 packet.
	Checksum uint16

	// SrcAddr is the "source ip address" of an IPv4 packet.
	SrcAddr uint32

	// DstAddr is the "destination ip address" of an IPv4 packet.
	DstAddr uint32
}

type IPv4 []byte

const (
	// IPv4MinimumSize is the minimum size of a valid IPv4 packet.
	IPv4MinimumSize = 20

	// IPv4MaximumHeaderSize is the maximum size of an IPv4 header. Given
	// that there are only 4 bits to represents the header length in 32-bit
	// units, the header cannot exceed 15*4 = 60 bytes.
	IPv4MaximumHeaderSize = 60

	// IPv4AddressSize is the size, in bytes, of an IPv4 address.
	IPv4AddressSize = 4

	// IPv4ProtocolNumber is IPv4's network protocol number.
	//IPv4ProtocolNumber tcpip.NetworkProtocolNumber = 0x0800

	// IPv4Version is the version of the ipv4 procotol.
	IPv4Version = 4
)

const (
	IPv4FlagMoreFragments = 1 << iota
	IPv4FlagDontFragment
)

// IPVersion returns the version of IP used in the given packet. It returns -1
// if the packet is not large enough to contain the version field.
func IPVersion(b []byte) int {
	// Length must be at least offset+length of version field.
	if len(b) < versIHL+1 {
		return -1
	}
	return int(b[versIHL] >> 4)
}

func (b IPv4) IsIPv4() bool {
	return b[versIHL]>>4 == IPv4Version
}

// HeaderLength returns the value of the "header length" field of the ipv4
// header.
func (b IPv4) HeaderLength() uint8 {
	return (b[versIHL] & 0xf) * 4
}

// ID returns the value of the identifier field of the the ipv4 header.
func (b IPv4) ID() uint16 {
	return binary.BigEndian.Uint16(b[id:])
}

// Protocol returns the value of the protocol field of the the ipv4 header.
func (b IPv4) Protocol() uint8 {
	return b[protocol]
}

// Flags returns the "flags" field of the ipv4 header.
func (b IPv4) Flags() uint8 {
	return uint8(binary.BigEndian.Uint16(b[flagsFO:]) >> 13)
}

// TTL returns the "TTL" field of the ipv4 header.
func (b IPv4) TTL() uint8 {
	return b[ttl]
}

// FragmentOffset returns the "fragment offset" field of the ipv4 header.
func (b IPv4) FragmentOffset() uint16 {
	return binary.BigEndian.Uint16(b[flagsFO:]) << 3
}

// TotalLength returns the "total length" field of the ipv4 header.
func (b IPv4) TotalLength() uint16 {
	return binary.BigEndian.Uint16(b[totalLen:])
}

func (b IPv4) SourceAddress() uint32 {
	return binary.BigEndian.Uint32(b[srcAddr : srcAddr+IPv4AddressSize])
}

func (b IPv4) DestinationAddress() uint32 {
	return binary.BigEndian.Uint32(b[dstAddr : dstAddr+IPv4AddressSize])
}

func (b IPv4) SourceAddrBuf() []byte {
	return b[srcAddr : srcAddr+IPv4AddressSize]
}

func (b IPv4) DestinationAddrBuf() []byte {
	return b[dstAddr : dstAddr+IPv4AddressSize]
}

// SetSourceAddress sets the "source address" field of the ipv4 header.
func (b IPv4) SetSourceAddress(ip uint32) {
	//copy(b[srcAddr:srcAddr+IPv4AddressSize], addr)
	binary.BigEndian.PutUint32(b[srcAddr:srcAddr+IPv4AddressSize], ip)
}

// SetDestinationAddress sets the "destination address" field of the ipv4
// header.
func (b IPv4) SetDestinationAddress(ip uint32) {
	//copy(b[dstAddr:dstAddr+IPv4AddressSize], addr)
	binary.BigEndian.PutUint32(b[dstAddr:dstAddr+IPv4AddressSize], ip)
}

// CalculateChecksum calculates the checksum of the ipv4 header.
func (b IPv4) CalculateChecksum() uint16 {
	return Checksum(b[:b.HeaderLength()], 0)
}

func (b IPv4) SetChecksum(v uint16) {
	binary.BigEndian.PutUint16(b[checksum:], v)
}
