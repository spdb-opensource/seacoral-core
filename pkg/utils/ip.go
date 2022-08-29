package utils

import (
	"encoding/binary"
	"net"
)

// IPToUint32 convert a IP string to unit32
func IPToUint32(ip string) uint32 {
	addr := net.ParseIP(ip)
	if addr == nil {
		return 0
	}
	return binary.BigEndian.Uint32(addr.To4())
}

// Uint32ToIP convert a unit32 to IP
func Uint32ToIP(cidr uint32) string {
	addr := make([]byte, 4)
	binary.BigEndian.PutUint32(addr, cidr)
	return net.IP(addr).String()
}
