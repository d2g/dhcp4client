package connections

import (
	"net"
	"time"
)

// Connections handle themselves based on the send from address and the receiving address.
// This is because handling unicasts and building the connections becomes complicated to handle at the client level as each implementation has different variations.
// For example packet sockets write to the raw ethernet connection by device id, adding the header information to the packet which is unrelated to the connection.
// This means when switching from broadcast to unicast it's still the same connection to the ethernet port. Therefore existing connections don't need to be closed
// between connections to other hosts. This information is only know by the connection type not by the client. Therefore rather than the client handle this the connection
// package itself is responsible for the implementation details.
// The idea of having transport here is to allow the implementation to manage pooling much like the HTTP transport in the standard lib.
type Transport struct {
	Dialer   Dialer
	Listener Listener

	//Used to better support backwards compatibility.
	Close func() error
}

func (t *Transport) Dial(l *net.UDPAddr, r *net.UDPAddr) (UDPConn, error) {
	return t.Dialer(l, r)
}

func (t *Transport) Listen(l *net.UDPAddr) (UDPConn, error) {
	return t.Listener(l)
}

type Dialer func(*net.UDPAddr, *net.UDPAddr) (UDPConn, error)

type Listener func(*net.UDPAddr) (UDPConn, error)

type UDPConn interface {
	ReadFrom(b []byte) (int, *net.UDPAddr, error)
	Write(b []byte) (int, error)

	Close() error

	LocalAddr() *net.UDPAddr
	RemoteAddr() *net.UDPAddr

	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}
