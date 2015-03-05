package dhcp4client

import (
	"bytes"
	"crypto/rand"
	"net"
	"sync"
	"time"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/d2g/dhcp4"
)

type Client struct {
	MACAddress    net.HardwareAddr //The MACAddress to send in the request.
	IgnoreServers []net.IP         //List of Servers to Ignore requests from.
	Timeout       time.Duration    //Time before we timeout.
	NoBcastFlag   bool             //Don't set the Bcast flag in BOOTP Flags

	connection      sock
	connectionMutex sync.Mutex //This is to stop us renewing as we're trying to get a normal
}

/*
 * Abstracts the type of underlying socket used
 */
type sock interface {
	Close() error
	Send(packet []byte) error
	RecvFrom() ([]byte, net.IP, error)
	SetReadTimeout(t time.Duration) error
}

/*
 * Connect Setup Connections to be used by other functions :D
 */
func (this *Client) Connect() error {
	if this.connection == nil {
		c, err := NewInetSock()

		if err != nil {
			return err
		}

		this.connection = c
	}
	return nil
}

/*
 * ConnectPacket is like Connect but uses AF_PACKET socket
 */
func (this *Client) ConnectPacket(ifindex int) error {
	if this.connection == nil {
		c, err := NewPacketSock(ifindex)
		if err != nil {
			return err
		}

		this.connection = c
	}

	return nil
}

/*
 * Close Connections
 */
func (this *Client) Close() error {
	if this.connection != nil {
		return this.connection.Close()
	}
	return nil
}

/*
 * Send the Discovery Packet to the Broadcast Channel
 */
func (this *Client) SendDiscoverPacket() (dhcp4.Packet, error) {
	discoveryPacket := this.DiscoverPacket()
	discoveryPacket.PadToMinSize()

	return discoveryPacket, this.SendPacket(discoveryPacket)
}

/*
 * Retreive Offer...
 * Wait for the offer for a specific Discovery Packet.
 */
func (this *Client) GetOffer(discoverPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	for {
		this.connection.SetReadTimeout(this.Timeout)
		readBuffer, source, err := this.connection.RecvFrom()
		if err != nil {
			return dhcp4.Packet{}, err
		}

		offerPacket := dhcp4.Packet(readBuffer)
		offerPacketOptions := offerPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		for _, ignoreServer := range this.IgnoreServers {
			if source.Equal(ignoreServer) {
				continue
			}

			if offerPacket.SIAddr().Equal(ignoreServer) {
				continue
			}
		}

		if len(offerPacketOptions[dhcp4.OptionDHCPMessageType]) < 1 || dhcp4.MessageType(offerPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.Offer || !bytes.Equal(discoverPacket.XId(), offerPacket.XId()) {
			continue
		}

		return offerPacket, nil
	}

}

/*
 * Send Request Based On the offer Received.
 */
func (this *Client) SendRequest(offerPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	requestPacket := this.RequestPacket(offerPacket)
	requestPacket.PadToMinSize()

	return requestPacket, this.SendPacket(requestPacket)
}

/*
 * Retreive Acknowledgement
 * Wait for the offer for a specific Request Packet.
 */
func (this *Client) GetAcknowledgement(requestPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	for {
		this.connection.SetReadTimeout(this.Timeout)
		readBuffer, source, err := this.connection.RecvFrom()
		if err != nil {
			return dhcp4.Packet{}, err
		}

		acknowledgementPacket := dhcp4.Packet(readBuffer)
		acknowledgementPacketOptions := acknowledgementPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		for _, ignoreServer := range this.IgnoreServers {
			if source.Equal(ignoreServer) {
				continue
			}

			if acknowledgementPacket.SIAddr().Equal(ignoreServer) {
				continue
			}
		}

		if !bytes.Equal(requestPacket.XId(), acknowledgementPacket.XId()) || len(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType]) < 1 || (dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK && dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.NAK) {
			continue
		}

		return acknowledgementPacket, nil
	}
}

/*
 * Send a DHCP Packet.
 */
func (this *Client) SendPacket(packet dhcp4.Packet) error {
	return this.connection.Send(packet)
}

/*
 * Create Discover Packet
 */
func (this *Client) DiscoverPacket() dhcp4.Packet {
	messageid := make([]byte, 4)
	if _, err := rand.Read(messageid); err != nil {
		panic(err)
	}

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(this.MACAddress)
	packet.SetXId(messageid)
	packet.SetBroadcast(!this.NoBcastFlag)

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Discover)})
	//packet.PadToMinSize()
	return packet
}

/*
 * Create Request Packet
 */
func (this *Client) RequestPacket(offerPacket *dhcp4.Packet) dhcp4.Packet {
	offerOptions := offerPacket.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(this.MACAddress)

	packet.SetXId(offerPacket.XId())
	packet.SetCIAddr(offerPacket.CIAddr())
	packet.SetSIAddr(offerPacket.SIAddr())

	packet.SetBroadcast(!this.NoBcastFlag)
	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})
	packet.AddOption(dhcp4.OptionRequestedIPAddress, (offerPacket.YIAddr()).To4())
	packet.AddOption(dhcp4.OptionServerIdentifier, offerOptions[dhcp4.OptionServerIdentifier])

	//packet.PadToMinSize()
	return packet
}

/*
 * Create Request Packet For a Renew
 */
func (this *Client) RenewalRequestPacket(acknowledgement *dhcp4.Packet) dhcp4.Packet {
	messageid := make([]byte, 4)
	if _, err := rand.Read(messageid); err != nil {
		panic(err)
	}

	acknowledgementOptions := acknowledgement.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(acknowledgement.CHAddr())

	packet.SetXId(messageid)
	packet.SetCIAddr(acknowledgement.YIAddr())
	packet.SetSIAddr(acknowledgement.SIAddr())

	packet.SetBroadcast(!this.NoBcastFlag)
	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})
	packet.AddOption(dhcp4.OptionRequestedIPAddress, (acknowledgement.YIAddr()).To4())
	packet.AddOption(dhcp4.OptionServerIdentifier, acknowledgementOptions[dhcp4.OptionServerIdentifier])

	//packet.PadToMinSize()
	return packet
}

/*
 * Lets do a Full DHCP Request.
 */
func (this *Client) Request() (bool, dhcp4.Packet, error) {
	discoveryPacket, err := this.SendDiscoverPacket()
	if err != nil {
		return false, discoveryPacket, err
	}

	offerPacket, err := this.GetOffer(&discoveryPacket)
	if err != nil {
		return false, offerPacket, err
	}

	requestPacket, err := this.SendRequest(&offerPacket)
	if err != nil {
		return false, requestPacket, err
	}

	acknowledgement, err := this.GetAcknowledgement(&requestPacket)
	if err != nil {
		return false, acknowledgement, err
	}

	acknowledgementOptions := acknowledgement.ParseOptions()
	if dhcp4.MessageType(acknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		return false, acknowledgement, nil
	}

	return true, acknowledgement, nil
}

/*
 * Renew a lease backed on the Acknowledgement Packet.
 * Returns Sucessfull, The AcknoledgementPacket, Any Errors
 */
func (this *Client) Renew(acknowledgement dhcp4.Packet) (bool, dhcp4.Packet, error) {
	renewRequest := this.RenewalRequestPacket(&acknowledgement)
	renewRequest.PadToMinSize()

	err := this.SendPacket(renewRequest)
	if err != nil {
		return false, renewRequest, err
	}

	newAcknowledgement, err := this.GetAcknowledgement(&acknowledgement)
	if err != nil {
		return false, newAcknowledgement, err
	}

	newAcknowledgementOptions := newAcknowledgement.ParseOptions()
	if dhcp4.MessageType(newAcknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		return false, newAcknowledgement, nil
	}

	return true, newAcknowledgement, nil
}
