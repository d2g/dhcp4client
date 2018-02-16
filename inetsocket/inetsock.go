package inetsocket

import (
	"net"
)

type inetSock struct {
	*net.UDPConn

	laddr net.UDPAddr
	raddr net.UDPAddr
}

func NewInetSock(options ...func(*inetSock) error) (*inetSock, error) {
	c := &inetSock{
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

func (c *inetSock) setOption(options ...func(*inetSock) error) error {
	for _, opt := range options {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

func SetLocalAddr(l net.UDPAddr) func(*inetSock) error {
	return func(c *inetSock) error {
		c.laddr = l
		return nil
	}
}

func SetRemoteAddr(r net.UDPAddr) func(*inetSock) error {
	return func(c *inetSock) error {
		c.raddr = r
		return nil
	}
}

func (c *inetSock) Write(packet []byte) (int, error) {
	n, err := c.WriteToUDP(packet, &c.raddr)
	return n, err
}

func (c *inetSock) ReadFrom(b []byte) (int, net.Addr, error) {
	return c.ReadFromUDP(b)
}

// UnicastFactory funcation
func (c *inetSock) UnicastConn() func(src, dest net.IP) (*inetSock, error) {
	return func(src, dest net.IP) (*inetSock, error) {
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
}
