package dhcp4client

import (
	"syscall"

	"github.com/krolaw/dhcp4"
)

//Create Discover Packet
func (c *Client) DiscoverPacket() dhcp4.Packet {
	messageid := make([]byte, 4)
	c.generateXID(messageid)

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(c.hardwareAddr)
	packet.SetXId(messageid)
	packet.SetBroadcast(true)

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Discover)})
	return packet
}

//Send the Discovery Packet to the Broadcast Channel
func (c *Client) SendDiscoverPacket() (dhcp4.Packet, error) {
	return c.SendDiscoverPacketWithOptions(nil)
}

func (c *Client) SendDiscoverPacketWithOptions(opts DHCP4ClientOptions) (dhcp4.Packet, error) {
	discoveryPacket := c.DiscoverPacket()
	for _, opt := range opts[dhcp4.Discover] {
		discoveryPacket.AddOption(opt.Code, opt.Value)
	}
	discoveryPacket.PadToMinSize()

	_, e := c.BroadcastPacket(discoveryPacket)
	if e != nil {
		//Ignore Network Down Errors
		if sc, ok := e.(syscall.Errno); ok && sc == syscall.ENETDOWN {
			return discoveryPacket, nil
		}

		err := &DHCP4Error{
			Err:  e,
			Src:  &c.laddr,
			Dest: &c.broadcastaddr,
		}

		return discoveryPacket, err
	}
	return discoveryPacket, nil
}
