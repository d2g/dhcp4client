package dhcp4client

import (
	"net"

	"github.com/krolaw/dhcp4"
)

//Release a lease backed on the Acknowledgement Packet.
//Returns Any Errors
func (c *Client) Release(dhcpaddr net.UDPAddr, acknowledgement dhcp4.Packet) error {
	release := c.ReleasePacket(&acknowledgement)
	release.PadToMinSize()

	//_, err := c.UnicastPacket(dhcpaddr, release)
	_, err := c.UnicastPacket(release)
	return err
}

/////////////////////////////////////////////////

//Create Release Packet For a Release
func (c *Client) ReleasePacket(acknowledgement *dhcp4.Packet) dhcp4.Packet {
	messageid := make([]byte, 4)
	c.generateXID(messageid)

	acknowledgementOptions := acknowledgement.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(acknowledgement.CHAddr())

	packet.SetXId(messageid)
	packet.SetBroadcast(true)
	packet.SetCIAddr(acknowledgement.YIAddr())

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Release)})
	packet.AddOption(dhcp4.OptionServerIdentifier, acknowledgementOptions[dhcp4.OptionServerIdentifier])

	return packet
}
