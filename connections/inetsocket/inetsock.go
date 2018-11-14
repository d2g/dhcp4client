package inetsocket

import (
	"net"

	"github.com/d2g/dhcp4client/connections"
)

type InetSock struct {
	*net.UDPConn
}

func (is *InetSock) Dialer() func(*net.UDPAddr, *net.UDPAddr) (connections.UDPConn, error) {
	return func(l *net.UDPAddr, r *net.UDPAddr) (connections.UDPConn, error) {
		u, err := net.DialUDP("udp", l, r)
		if err != nil {
			return nil, err
		}
		return &InetSock{u}, nil
	}
}

func (is *InetSock) Listener() func(*net.UDPAddr) (connections.UDPConn, error) {
	return func(l *net.UDPAddr) (connections.UDPConn, error) {

		u, err := net.ListenUDP("udp", l)
		return &InetSock{u}, err
	}
}

func (is *InetSock) LocalAddr() *net.UDPAddr {
	return is.UDPConn.LocalAddr().(*net.UDPAddr)
}

func (is *InetSock) RemoteAddr() *net.UDPAddr {
	return is.UDPConn.RemoteAddr().(*net.UDPAddr)
}

func (is *InetSock) ReadFrom(b []byte) (int, *net.UDPAddr, error) {
	a, r, e := is.UDPConn.ReadFrom(b)
	if r != nil {
		return a, r.(*net.UDPAddr), e
	}
	return a, nil, e
}
