package inetsocket

import (
	"net"

	"github.com/d2g/dhcp4client/connections"
)

type InetSock struct {
	*net.UDPConn

	laddr net.UDPAddr
	raddr net.UDPAddr
}

func NewInetSock(options ...func(*InetSock) error) (*InetSock, error) {
	c := &InetSock{
		laddr: net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 68},
		raddr: net.UDPAddr{IP: net.IPv4bcast, Port: 67},
	}

	err := c.setOption(options...)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", &c.laddr)
	if err != nil {
		return nil, err
	}

	c.UDPConn = conn

	return c, err
}

func (c *InetSock) setOption(options ...func(*InetSock) error) error {
	for _, opt := range options {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

func SetLocalAddr(l net.UDPAddr) func(*InetSock) error {
	return func(c *InetSock) error {
		c.laddr = l
		return nil
	}
}

func SetRemoteAddr(r net.UDPAddr) func(*InetSock) error {
	return func(c *InetSock) error {
		c.raddr = r
		return nil
	}
}

func (c *InetSock) Write(packet []byte) (int, error) {
	n, err := c.WriteToUDP(packet, &c.raddr)
	return n, err
}

func (c *InetSock) ReadFrom(b []byte) (int, net.IP, error) {
	i, src, err := c.ReadFromUDP(b)
	if src != nil {
		return i, src.IP, err
	}
	return i, nil, err
}

// UnicastFactory funcation
func (c *InetSock) UnicastConn(src, dest net.IP) (connections.Conn, error) {
	//Work out the UDP addresses from the IP provided and the ports in the current connection.
	laddr := net.UDPAddr{
		IP:   src,
		Port: c.laddr.Port,
	}

	raddr := net.UDPAddr{
		IP:   dest,
		Port: c.raddr.Port,
	}

	return NewInetSock(SetLocalAddr(laddr), SetRemoteAddr(raddr))
}
