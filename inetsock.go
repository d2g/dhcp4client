package dhcp4client

import (
	"net"
	"time"
)

type inetSock struct {
	*net.UDPConn
}

func NewInetSock() (*inetSock, error) {
	address := net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 68}
	c, err := net.ListenUDP("udp4", &address)
	return &inetSock{c}, err
}

func (c *inetSock) Write(packet []byte) error {
	address := net.UDPAddr{IP: net.IPv4bcast, Port: 67}

	_, err := c.WriteToUDP(packet, &address)
	return err
}

func (c *inetSock) ReadFrom() ([]byte, net.IP, error) {
	readBuffer := make([]byte, MaxDHCPLen)
	n, source, err := c.ReadFromUDP(readBuffer)
	if source != nil {
		return readBuffer[:n], source.IP, err
	} else {
		return readBuffer[:n], net.IP{}, err
	}
}

func (c *inetSock) SetReadTimeout(t time.Duration) error {
	return c.SetReadDeadline(time.Now().Add(t))
}
