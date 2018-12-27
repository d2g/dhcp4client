package dhcp4client

import (
	"bytes"
	"errors"
	"net"
	"time"

	"github.com/krolaw/dhcp4"
)

// Retrieve Acknowledgement
// Wait for the offer for a specific Request Packet.
func (c *Client) GetAcknowledgement(requestPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	return c.GetAcknowledgementWithOptions(requestPacket, nil)
}

func (c *Client) GetAcknowledgementWithOptions(requestPacket *dhcp4.Packet, opts DHCP4ClientOptions) (dhcp4.Packet, error) {
	start := time.Now()

	for {
		timeout := c.timeout - time.Since(start)
		if timeout <= 0 {
			return dhcp4.Packet{}, &DHCP4Error{Err: errors.New("Timed Out"), Src: &c.laddr, Dest: &c.broadcastaddr, IsTimeout: true, IsTemporary: true}
		}

		con, err := c.connection.Listen(&c.laddr)
		if err != nil {
			return dhcp4.Packet{}, &DHCP4Error{Err: err, Src: &c.laddr, Dest: &c.broadcastaddr, IsTimeout: false, IsTemporary: false}
		}
		defer con.Close()

		con.SetReadDeadline(time.Now().Add(timeout))

		readBuffer := make([]byte, MaxDHCPLen)
		_, source, err := con.ReadFrom(readBuffer)
		if err != nil {
			return dhcp4.Packet{}, err
		}

		acknowledgementPacket := dhcp4.Packet(readBuffer)
		acknowledgementPacketOptions := acknowledgementPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		//BUG(d2g): Should Use the Server Identifier Option
		if c.ignoreServer([]net.IP{source.IP}) {
			continue
		}

		if !bytes.Equal(requestPacket.XId(), acknowledgementPacket.XId()) || len(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType]) < 1 || (dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK && dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.NAK) {
			continue
		}

		for _, opt := range opts[dhcp4.Offer] {
			opt.Value = acknowledgementPacketOptions[opt.Code]
		}

		return acknowledgementPacket, nil
	}
}
