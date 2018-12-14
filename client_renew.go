package dhcp4client

import (
	"net"

	"github.com/krolaw/dhcp4"
)

//
func (c *Client) RenewalRequestPacket(l net.IP, s net.IP) dhcp4.Packet {
	messageid := make([]byte, 4)
	c.generateXID(messageid)

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetXId(messageid)

	packet.SetCHAddr(c.hardwareAddr)
	//CIAddr should only be populated on renewals.
	packet.SetCIAddr(l.To4())
	packet.SetBroadcast(false)

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})
	packet.AddOption(dhcp4.OptionServerIdentifier, s.To4())

	return packet
}

// RenewalRequestPacket Generates Create Request Packet For a Renew
func (c *Client) RenewalRequestPacketFromAcknowledgment(a *dhcp4.Packet) dhcp4.Packet {
	opts := a.ParseOptions()

	//The 'yiaddr' field in the DHCPACK messages is filled in with the selected network address. [RFC 2131 p15]
	//Server identifier Must be included in the DHCPACK [RFC 2131 p28]
	return c.RenewalRequestPacket(a.YIAddr(), opts[dhcp4.OptionServerIdentifier])
}

//
func (c *Client) RenewalRequestPacketWithOptions(l net.IP, s net.IP, opts DHCP4ClientOptions) dhcp4.Packet {
	packet := c.RenewalRequestPacket(l, s)

	for _, opt := range opts[dhcp4.Request] {
		packet.AddOption(opt.Code, opt.Value)
	}

	return packet
}
