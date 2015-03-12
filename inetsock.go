package dhcp4client

import (
	"net"
	"time"
)

type inetSock struct {
	c *net.UDPConn
}

func NewInetSock() (*inetSock, error) {
	address := net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 68}
	c, err := net.ListenUDP("udp4", &address)
	return &inetSock{c}, err
}

func (c *inetSock) Close() error {
	return c.c.Close()
}

func (c *inetSock) Send(packet []byte) error {
	address := net.UDPAddr{IP: net.IPv4bcast, Port: 67}

	_, err := c.c.WriteToUDP(packet, &address)
	//I Keep experencing what seems to be random "invalid argument" errors
	//if err != nil {
	//	log.Printf("Error:%v\n", err)
	//}
	return err
}

func (c *inetSock) RecvFrom() ([]byte, net.IP, error) {
	readBuffer := make([]byte, maxDHCPLen)
	n, source, err := c.c.ReadFromUDP(readBuffer)
	return readBuffer[:n], source.IP, err
}

func (c *inetSock) SetReadTimeout(t time.Duration) error {
	return c.c.SetReadDeadline(time.Now().Add(t))
}
