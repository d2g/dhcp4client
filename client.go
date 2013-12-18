package dhcp4client

import (
	"bytes"
	"encoding/binary"
	"github.com/d2g/dhcp4"
	//"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Client struct {
	MACAddress    net.HardwareAddr //The MACAddress to send in the request.
	IgnoreServers []net.IP         //List of Servers to Ignore requests from.
	Timeout       time.Duration    //Time before we timeout.

	connection      *net.UDPConn
	connectionMutex sync.Mutex //This is to stop us renewing as we're trying to get a normal
}

/*
 * Connect Setup Connections to be used by other functions :D
 */
func (this *Client) Connect() error {
	var err error

	if this.connection == nil {
		address := net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 68}
		this.connection, err = net.ListenUDP("udp4", &address)

		if err != nil {
			return err
		}
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

	readBuffer := make([]byte, 576)

	for {
		this.connection.SetReadDeadline(time.Now().Add(this.Timeout))
		_, source, err := this.connection.ReadFromUDP(readBuffer)
		if err != nil {
			return dhcp4.Packet{}, err
		}

		offerPacket := dhcp4.Packet(readBuffer)
		offerPacketOptions := offerPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		for _, ignoreServer := range this.IgnoreServers {
			if source.IP.Equal(ignoreServer) {
				continue
			}

			if offerPacket.SIAddr().Equal(ignoreServer) {
				continue
			}
		}

		if dhcp4.MessageType(offerPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.Offer || !bytes.Equal(discoverPacket.XId(), offerPacket.XId()) {
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
	readBuffer := make([]byte, 576)

	for {
		this.connection.SetReadDeadline(time.Now().Add(this.Timeout))
		_, source, err := this.connection.ReadFromUDP(readBuffer)
		if err != nil {
			return dhcp4.Packet{}, err
		}

		acknowledgementPacket := dhcp4.Packet(readBuffer)
		acknowledgementPacketOptions := acknowledgementPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		for _, ignoreServer := range this.IgnoreServers {
			if source.IP.Equal(ignoreServer) {
				continue
			}

			if acknowledgementPacket.SIAddr().Equal(ignoreServer) {
				continue
			}
		}

		if dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK || !bytes.Equal(requestPacket.XId(), acknowledgementPacket.XId()) {
			continue
		}

		return acknowledgementPacket, nil
	}
}

/*
 * Send a DHCP Packet.
 */
func (this *Client) SendPacket(packet dhcp4.Packet) error {
	address := net.UDPAddr{IP: net.IPv4bcast, Port: 67}

	_, err := this.connection.WriteToUDP(packet, &address)
	//I Keep experencing what seems to be random "invalid argument" errors
	//if err != nil {
	//	log.Printf("Error:%v\n", err)
	//}
	return err
}

/*
 * Create Discover Packet
 */
func (this *Client) DiscoverPacket() dhcp4.Packet {
	rand.Seed(time.Now().Unix())
	messageid := make([]byte, 4)
	binary.LittleEndian.PutUint32(messageid, rand.Uint32())

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(this.MACAddress)
	packet.SetXId(messageid)
	packet.SetBroadcast(true)

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

	packet.SetBroadcast(true)
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
	rand.Seed(time.Now().Unix())
	messageid := make([]byte, 4)
	binary.LittleEndian.PutUint32(messageid, rand.Uint32())

	acknowledgementOptions := acknowledgement.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(acknowledgement.CHAddr())

	packet.SetXId(messageid)
	packet.SetCIAddr(acknowledgement.YIAddr())
	packet.SetSIAddr(acknowledgement.SIAddr())

	packet.SetBroadcast(true)
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
