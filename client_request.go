package dhcp4client

import (
	"net"

	"github.com/krolaw/dhcp4"
)

//Create Request Packet
//r is the IP to request, s is server to request it from
func (c *Client) RequestPacket(r net.IP, s net.IP) dhcp4.Packet {
	messageid := make([]byte, 4)
	c.generateXID(messageid)

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetXId(messageid)
	packet.SetBroadcast(true)
	packet.SetCHAddr(c.hardwareAddr)
	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})

	//The 'requested IP address' option MUST be set to the value of 'yiaddr' in the DHCPOFFER message from the server.[RFC 2131 p15]
	packet.AddOption(dhcp4.OptionRequestedIPAddress, r.To4())
	packet.AddOption(dhcp4.OptionServerIdentifier, s.To4())

	return packet
}

func (c *Client) RequestPacketWithOptions(r net.IP, s net.IP, opts DHCP4ClientOptions) dhcp4.Packet {
	packet := c.RequestPacket(r, s)

	for _, opt := range opts[dhcp4.Request] {
		packet.AddOption(opt.Code, opt.Value)
	}
	packet.PadToMinSize()

	return packet
}

func (c *Client) RequestPacketFromOfferPacket(offerPacket *dhcp4.Packet) dhcp4.Packet {
	offerOptions := offerPacket.ParseOptions()
	packet := c.RequestPacket(offerPacket.YIAddr(), offerOptions[dhcp4.OptionServerIdentifier])
	packet.PadToMinSize()
	return packet
}

//Send Request Based On the offer Received.
func (c *Client) SendRequestFromOfferPacket(offerPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	return c.SendRequestFromOfferPacketWithOptions(offerPacket, nil)
}

func (c *Client) SendRequestFromOfferPacketWithOptions(offerPacket *dhcp4.Packet, opts DHCP4ClientOptions) (requestPacket dhcp4.Packet, err error) {
	requestPacket = c.RequestPacketFromOfferPacket(offerPacket)
	for _, opt := range opts[dhcp4.Request] {
		requestPacket.AddOption(opt.Code, opt.Value)
	}
	requestPacket.PadToMinSize()

	_, err = c.BroadcastPacket(requestPacket)
	return
}
