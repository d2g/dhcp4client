package dhcp4client

import (
	"bytes"
	"errors"
	"net"
	"syscall"
	"time"

	"github.com/krolaw/dhcp4"
)

//Retreive Offer...
//Wait for the offer for a specific Discovery Packet.
// UDP address contain content of option 54.
func (c *Client) GetOffer(discoverPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	return c.GetOfferWithOptions(discoverPacket.XId(), nil)
}

//
func (c *Client) GetOfferWithOptions(xid []byte, opts DHCP4ClientOptions) (dhcp4.Packet, error) {
	start := time.Now()

	for {
		timeout := c.timeout - time.Since(start)
		if timeout <= 0 {
			return dhcp4.Packet{}, &DHCP4Error{Err: errors.New("Timed Out"), Src: &c.laddr, Dest: &c.broadcastaddr, IsTimeout: true, IsTemporary: true}
		}

		con, err := c.connection.Listen(&c.laddr)
		if err != nil {
			return dhcp4.Packet{}, &DHCP4Error{Err: err, Src: &c.laddr, Dest: &c.broadcastaddr}
		}
		defer con.Close()

		con.SetReadDeadline(time.Now().Add(timeout))

		readBuffer := make([]byte, MaxDHCPLen)
		_, _, err = con.ReadFrom(readBuffer)
		if err != nil {
			//Ignore Network Down Errors
			if sc, ok := err.(syscall.Errno); !ok || sc != syscall.ENETDOWN {
				return dhcp4.Packet{}, &DHCP4Error{Err: err, Src: &c.laddr, Dest: &c.broadcastaddr}
			}
		}

		offerPacket := dhcp4.Packet(readBuffer)
		offerPacketOptions := offerPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		if c.ignoreServer([]net.IP{offerPacketOptions[dhcp4.OptionServerIdentifier]}) {
			continue
		}

		if len(offerPacketOptions[dhcp4.OptionDHCPMessageType]) < 1 || dhcp4.MessageType(offerPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.Offer || !bytes.Equal(xid, offerPacket.XId()) {
			continue
		}

		for _, opt := range opts[dhcp4.Offer] {
			opt.Value = offerPacketOptions[opt.Code]
		}

		return offerPacket, nil
	}

}
