// Copyright 2016 The Netstack Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package header provides the implementation of the encoding and decoding of
// network protocol headers.
package nat

// Checksum calculates the checksum (as defined in RFC 1071) of the bytes in the
// given byte array.
func Checksum(buf []byte, initial uint16) uint16 {
	v := uint32(initial)

	l := len(buf)
	if l&1 != 0 {
		l--
		v += uint32(buf[l]) << 8
	}

	for i := 0; i < l; i += 2 {
		v += (uint32(buf[i]) << 8) + uint32(buf[i+1])
	}

	return ChecksumCombine(uint16(v), uint16(v>>16))
}

// ChecksumCombine combines the two uint16 to form their checksum. This is done
// by adding them and the carry.
func ChecksumCombine(a, b uint16) uint16 {
	v := uint32(a) + uint32(b)
	return uint16(v + v>>16)
}

/*
  1. func CheckSum(data []byte) uint16 {
  2.     var (
  3.         sum    uint32
  4.         length int = len(data)
  5.         index  int
  6.     )
  7.     for length > 1 {
  8.         sum += uint32(data[index])<<8 + uint32(data[index+1])
  9.         index += 2
  10.         length -= 2
  11.     }
  12.     if length > 0 {
  13.         sum += uint32(data[index]) << 8
  14.     }
		  sum = (sum >> 16) + (sum & 0xFFFF);
  15.     sum += (sum >> 16)
  16.
  17.     return uint16(^sum)
  18. }
*/
// PseudoHeaderChecksum calculates the pseudo-header checksum for the
// given destination protocol and network address, ignoring the length
// field. Pseudo-headers are needed by transport layers when calculating
// their own checksum.
func PseudoHeaderChecksum(protocol uint8, srcAddr []byte, dstAddr []byte) uint16 {
	xsum := Checksum(srcAddr, 0)
	xsum = Checksum(dstAddr, xsum)
	return Checksum([]byte{0, protocol}, xsum)
}
