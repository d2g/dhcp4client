package testsocket

import (
	"net"
	"time"

	"github.com/d2g/dhcp4client/connections"
)

const (
	MaxDHCPLen = 576
)

var ReadResponse = [2][MaxDHCPLen]byte{
	{0},
	{0},
}

type TestSocket struct {
	ReadPosition int
}

func (ts *TestSocket) Dialer() func(*net.UDPAddr, *net.UDPAddr) (connections.UDPConn, error) {
	return func(l *net.UDPAddr, r *net.UDPAddr) (connections.UDPConn, error) {
		return &TestSocket{}, nil
	}
}

func (ts *TestSocket) Listener() func(*net.UDPAddr) (connections.UDPConn, error) {
	return func(l *net.UDPAddr) (connections.UDPConn, error) {
		return &TestSocket{}, nil
	}
}

func (ts *TestSocket) LocalAddr() *net.UDPAddr {
	return &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 68,
	}
}

func (ts *TestSocket) RemoteAddr() *net.UDPAddr {
	return &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 67,
	}
}

func (ts *TestSocket) ReadFrom(b []byte) (int, *net.UDPAddr, error) {
	return 0, nil, nil
}

func (ts *TestSocket) Close() error {
	return nil
}

func (ts *TestSocket) SetDeadline(t time.Time) error {
	return nil
}

func (ts *TestSocket) SetReadDeadline(t time.Time) error {
	return nil
}

func (ts *TestSocket) SetWriteDeadline(t time.Time) error {
	return nil
}

func (ts *TestSocket) Write(b []byte) (int, error) {
	return 0, nil
}
