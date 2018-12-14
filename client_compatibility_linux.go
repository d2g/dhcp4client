package dhcp4client

import (
	"github.com/d2g/dhcp4client/connections"
	"github.com/d2g/dhcp4client/connections/pktsocket"
)

func NewPacketSock(ifindex int) (connections.Transport, error) {

	c, err := pktsocket.NewPacketSock(ifindex)
	if err != nil {
		return connections.Transport{}, err
	}

	t := connections.Transport{
		Dialer:   c.Dialer(),
		Listener: c.Listener(),
		Close:    c.Close,
	}

	return t, nil
}
