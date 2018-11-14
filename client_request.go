package dhcp4client

import (
	"github.com/krolaw/dhcp4"
)

//Create Request Packet
func (c *Client) RequestPacket(offerPacket *dhcp4.Packet) dhcp4.Packet {
	offerOptions := offerPacket.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(c.hardwareAddr)

	//BUG(DG): Shouldn't we generate a new xid?
	packet.SetXId(offerPacket.XId())
	packet.SetBroadcast(true)

	packet.SetCIAddr(offerPacket.CIAddr())
	packet.SetSIAddr(offerPacket.SIAddr())

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})
	packet.AddOption(dhcp4.OptionRequestedIPAddress, (offerPacket.YIAddr()).To4())
	packet.AddOption(dhcp4.OptionServerIdentifier, offerOptions[dhcp4.OptionServerIdentifier])

	return packet
}

//Send Request Based On the offer Received.
func (c *Client) SendRequest(offerPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	return c.SendRequestWithOptions(offerPacket, nil)
}

func (c *Client) SendRequestWithOptions(offerPacket *dhcp4.Packet, opts DHCP4ClientOptions) (requestPacket dhcp4.Packet, err error) {
	requestPacket = c.RequestPacket(offerPacket)
	for _, opt := range opts[dhcp4.Request] {
		requestPacket.AddOption(opt.Code, opt.Value)
	}
	requestPacket.PadToMinSize()

	_, err = c.BroadcastPacket(requestPacket)
	return
}

// RenewalRequestPacket Generates Create Request Packet For a Renew
func (c *Client) RenewalRequestPacket(acknowledgement *dhcp4.Packet) dhcp4.Packet {
	messageid := make([]byte, 4)
	c.generateXID(messageid)

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetXId(messageid)

	packet.SetCHAddr(acknowledgement.CHAddr()) //c.hardwareaddress?
	packet.SetCIAddr(acknowledgement.YIAddr())

	packet.SetBroadcast(false)
	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})

	opts := acknowledgement.ParseOptions()
	packet.AddOption(dhcp4.OptionServerIdentifier, opts[dhcp4.OptionServerIdentifier])

	return packet
}
