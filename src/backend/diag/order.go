package diag

import "encoding/binary"

// hostOrder returns the four bytes of an IPv4 address as the kernel prints an address in
// /proc/net: the bytes of a 32-bit number in the byte order of this machine, written as a
// hexadecimal number.
func hostOrder(address []byte) []byte {
	result := make([]byte, 4)
	binary.BigEndian.PutUint32(result, binary.NativeEndian.Uint32(address))
	return result
}
