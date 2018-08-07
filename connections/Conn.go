package connections

import (
	"net"
	"time"
)

// Abstracts the type of underlying socket used
// Altered to more closly represent the net.Conn
// Unable to use net.Conn as only ReadFrom returns the senders address
type Conn interface {
	ReadFrom(b []byte) (int, net.IP, error)
	Write(b []byte) (int, error)

	Close() error

	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error

	UnicastConn(net.IP, net.IP) (Conn, error)
}
