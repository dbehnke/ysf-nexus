package bridge

import (
	"net"
)

// NetworkServer interface defines the methods required for bridge network operations
type NetworkServer interface {
	SendPacket(data []byte, addr *net.UDPAddr) error
	GetListenAddress() *net.UDPAddr
}
