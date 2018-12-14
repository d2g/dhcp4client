package dhcp4client

import (
	"github.com/d2g/dhcp4client/connections"
	"github.com/d2g/dhcp4client/connections/inetsocket"
)

func NewInetSock() (connections.Transport, error) {

	d := inetsocket.InetSock{}

	t := connections.Transport{
		Dialer:   d.Dialer(),
		Listener: d.Listener(),
	}

	return t, nil
}
