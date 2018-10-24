package dhcp4client

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/d2g/dhcp4"
	"github.com/d2g/dhcp4client/connections"
	"github.com/d2g/dhcp4client/connections/inetsocket"
)

const (
	MaxDHCPLen = 576
)

type MultiError []error

func (m MultiError) Error() string {
	var s bytes.Buffer

	for _, e := range m {
		if e != nil {
			if s.Len() > 0 {
				s.WriteString(" & ")
			}
			s.WriteString(e.Error())
		}
	}

	return s.String()
}

type DHCP4Error struct {
	Err         error    // Child Error
	OpCode      int      // The DHCP option code of the action
	Src         net.Addr // connection source
	Dest        net.Addr // destination source
	IsTimeout   bool     // if true, timed out; not all timeouts set this
	IsTemporary bool     // if true, error is temporary; not all errors set this
}

func (e *DHCP4Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	var s string
	if e.OpCode > 0 {
		s += "Oppcode " + strconv.Itoa(e.OpCode)
	}
	if e.Src != nil {
		s += " From " + e.Src.String()
	}
	if e.Dest != nil {
		s += " To " + e.Dest.String()
	}
	if len(s) > 0 {
		s += ": "
	}
	s += e.Err.Error()
	return s
}

func (e *DHCP4Error) Timeout() bool {
	if e.IsTimeout {
		return true
	}

	//Check The Child Error For Timeout
	netErr, ok := e.Err.(net.Error)
	if ok {
		e.IsTimeout = netErr.Timeout()
	}
	return e.IsTimeout
}

func (e *DHCP4Error) Temporary() bool {
	if e.IsTemporary {
		return true
	}

	//Check The Child Error For Temporary error
	netErr, ok := e.Err.(net.Error)
	if ok {
		e.IsTemporary = netErr.Temporary()
	}

	return e.IsTemporary
}

type Client struct {
	hardwareAddr  net.HardwareAddr //The HardwareAddr to send in the request.
	ignoreServers []net.IP         //List of Servers to Ignore requests from.
	timeout       time.Duration    //Time before we timeout.

	connection connections.Transport //Where connections are sourced

	laddr         net.UDPAddr //Local Clients Address
	broadcastaddr net.UDPAddr //Remote Broadcast Address

	generateXID func([]byte) //Function Used to Generate a XID
}

func New(options ...func(*Client) error) (*Client, error) {
	d := inetsocket.InetSock{}

	c := Client{
		timeout:     time.Second * 10,
		generateXID: CryptoGenerateXID,
		connection: connections.Transport{
			Dial:   d.Dialer(),
			Listen: d.Listener(),
		},
		laddr:         net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 68},
		broadcastaddr: net.UDPAddr{IP: net.IPv4bcast, Port: 67},
	}

	err := c.SetOption(options...)
	if err != nil {
		return nil, err
	}

	//if connection hasn't been set as an option create the default.
	return &c, nil
}

func (c *Client) SetOption(options ...func(*Client) error) error {
	for _, opt := range options {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

func Timeout(t time.Duration) func(*Client) error {
	return func(c *Client) error {
		c.timeout = t
		return nil
	}
}

func IgnoreServers(s []net.IP) func(*Client) error {
	return func(c *Client) error {
		c.ignoreServers = s
		return nil
	}
}

func HardwareAddr(h net.HardwareAddr) func(*Client) error {
	return func(c *Client) error {
		c.hardwareAddr = h
		return nil
	}
}

func Connection(co connections.Transport) func(*Client) error {
	return func(c *Client) error {
		c.connection = co
		return nil
	}
}

func GenerateXID(g func([]byte)) func(*Client) error {
	return func(c *Client) error {
		c.generateXID = g
		return nil
	}
}

// SetLocalAddrs
func (c *Client) SetLaddr(l *net.UDPAddr) {
	c.laddr = *l
}

//Close Connections
func (c *Client) Close() error {
	//For later use for informing the connections.transport to close any pooling.
	return nil
}

//Returns true if any of the addresses supplied are in the ignore list
func (c *Client) ignoreServer(srcs []net.IP) bool {
	for _, src := range srcs {
		// Ignore Servers in my Ignore list
		for _, ignoreServer := range c.ignoreServers {
			if src.Equal(ignoreServer) {
				return true
			}
		}
	}
	return false
}

//Send the Discovery Packet to the Broadcast Channel
func (c *Client) SendDiscoverPacket() (dhcp4.Packet, error) {
	discoveryPacket := c.DiscoverPacket()
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

//Retreive Offer...
//Wait for the offer for a specific Discovery Packet.
func (c *Client) GetOffer(discoverPacket *dhcp4.Packet) (dhcp4.Packet, *net.UDPAddr, error) {
	start := time.Now()

	for {
		timeout := c.timeout - time.Since(start)
		if timeout <= 0 {
			return dhcp4.Packet{}, nil, &DHCP4Error{Err: errors.New("Timed Out"), Src: &c.laddr, Dest: &c.broadcastaddr, IsTimeout: true, IsTemporary: true}
		}

		con, err := c.connection.Listen(&c.laddr)
		if err != nil {
			return dhcp4.Packet{}, nil, &DHCP4Error{Err: err, Src: &c.laddr, Dest: &c.broadcastaddr}
		}
		defer con.Close()

		con.SetReadDeadline(time.Now().Add(timeout))

		readBuffer := make([]byte, MaxDHCPLen)
		_, source, err := con.ReadFrom(readBuffer)
		if err != nil {
			//Ignore Network Down Errors
			if sc, ok := err.(syscall.Errno); !ok || sc != syscall.ENETDOWN {
				return dhcp4.Packet{}, source, &DHCP4Error{Err: err, Src: &c.laddr, Dest: &c.broadcastaddr}
			}
		}

		offerPacket := dhcp4.Packet(readBuffer)
		offerPacketOptions := offerPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		if c.ignoreServer([]net.IP{source.IP}) {
			continue
		}

		if len(offerPacketOptions[dhcp4.OptionDHCPMessageType]) < 1 || dhcp4.MessageType(offerPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.Offer || !bytes.Equal(discoverPacket.XId(), offerPacket.XId()) {
			continue
		}

		return offerPacket, source, nil
	}

}

//Send Request Based On the offer Received.
func (c *Client) SendRequest(offerPacket *dhcp4.Packet) (requestPacket dhcp4.Packet, err error) {
	requestPacket = c.RequestPacket(offerPacket)
	requestPacket.PadToMinSize()

	_, err = c.BroadcastPacket(requestPacket)
	return
}

// Retrieve Acknowledgement
// Wait for the offer for a specific Request Packet.
func (c *Client) GetAcknowledgement(requestPacket *dhcp4.Packet) (dhcp4.Packet, *net.UDPAddr, error) {
	start := time.Now()

	for {
		timeout := c.timeout - time.Since(start)
		if timeout <= 0 {
			return dhcp4.Packet{}, nil, &DHCP4Error{Err: errors.New("Timed Out"), Src: &c.laddr, Dest: &c.broadcastaddr, IsTimeout: true, IsTemporary: true}
		}

		con, err := c.connection.Listen(&c.laddr)
		if err != nil {
			return dhcp4.Packet{}, nil, &DHCP4Error{Err: err, Src: &c.laddr, Dest: &c.broadcastaddr, IsTimeout: false, IsTemporary: false}
		}
		defer con.Close()

		con.SetReadDeadline(time.Now().Add(timeout))

		readBuffer := make([]byte, MaxDHCPLen)
		_, source, err := con.ReadFrom(readBuffer)
		if err != nil {
			return dhcp4.Packet{}, source, err
		}

		acknowledgementPacket := dhcp4.Packet(readBuffer)
		acknowledgementPacketOptions := acknowledgementPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		if c.ignoreServer([]net.IP{source.IP}) {
			continue
		}

		if !bytes.Equal(requestPacket.XId(), acknowledgementPacket.XId()) || len(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType]) < 1 || (dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK && dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.NAK) {
			continue
		}

		return acknowledgementPacket, source, nil
	}
}

// Send Decline to the received acknowledgement.
func (c *Client) SendDecline(acknowledgementPacket *dhcp4.Packet) (declinePacket dhcp4.Packet, err error) {
	declinePacket = c.DeclinePacket(acknowledgementPacket)
	declinePacket.PadToMinSize()

	_, err = c.BroadcastPacket(declinePacket)
	return
}

func (c *Client) BroadcastPacket(packet dhcp4.Packet) (i int, err error) {

	con, err := c.connection.Dial(&c.laddr, &c.broadcastaddr)
	if err != nil {
		return
	}
	defer con.Close()

	con.SetWriteDeadline(time.Now().Add(c.timeout))

	i, err = con.Write(packet)
	return
}

func (c *Client) UnicastPacket(dhcpAddr net.UDPAddr, packet dhcp4.Packet) (i int, err error) {
	con, err := c.connection.Dial(&c.laddr, &dhcpAddr)
	if err != nil {
		return
	}
	defer con.Close()

	con.SetWriteDeadline(time.Now().Add(c.timeout))

	i, err = con.Write(packet)
	return
}

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

//Create Request Packet
func (c *Client) RequestPacket(offerPacket *dhcp4.Packet) dhcp4.Packet {
	offerOptions := offerPacket.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(c.hardwareAddr)

	packet.SetXId(offerPacket.XId())
	packet.SetBroadcast(true)

	packet.SetCIAddr(offerPacket.CIAddr())
	packet.SetSIAddr(offerPacket.SIAddr())

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})
	packet.AddOption(dhcp4.OptionRequestedIPAddress, (offerPacket.YIAddr()).To4())
	packet.AddOption(dhcp4.OptionServerIdentifier, offerOptions[dhcp4.OptionServerIdentifier])

	return packet
}

//Create Request Packet For a Renew
func (c *Client) RenewalRequestPacket(acknowledgement *dhcp4.Packet) dhcp4.Packet {
	messageid := make([]byte, 4)
	c.generateXID(messageid)

	acknowledgementOptions := acknowledgement.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(acknowledgement.CHAddr())

	packet.SetXId(messageid)
	packet.SetCIAddr(acknowledgement.YIAddr())

	packet.SetBroadcast(false)
	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})
	packet.AddOption(dhcp4.OptionServerIdentifier, acknowledgementOptions[dhcp4.OptionServerIdentifier])

	return packet
}

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

//Create Decline Packet
func (c *Client) DeclinePacket(acknowledgement *dhcp4.Packet) dhcp4.Packet {
	messageid := make([]byte, 4)
	c.generateXID(messageid)

	acknowledgementOptions := acknowledgement.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(acknowledgement.CHAddr())
	packet.SetXId(messageid)
	packet.SetBroadcast(true)

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Decline)})
	packet.AddOption(dhcp4.OptionRequestedIPAddress, (acknowledgement.YIAddr()).To4())
	packet.AddOption(dhcp4.OptionServerIdentifier, acknowledgementOptions[dhcp4.OptionServerIdentifier])

	return packet
}

//Lets do a Full DHCP Request.
func (c *Client) Request() (bool, *net.UDPAddr, dhcp4.Packet, error) {
	discoveryPacket, err := c.SendDiscoverPacket()
	if err != nil {
		return false, nil, discoveryPacket, err
	}

	offerPacket, src, err := c.GetOffer(&discoveryPacket)
	if err != nil {
		return false, src, offerPacket, err
	}

	requestPacket, err := c.SendRequest(&offerPacket)
	if err != nil {
		return false, src, requestPacket, err
	}

	acknowledgement, _, err := c.GetAcknowledgement(&requestPacket)
	if err != nil {
		return false, src, acknowledgement, err
	}

	acknowledgementOptions := acknowledgement.ParseOptions()
	if dhcp4.MessageType(acknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		return false, src, acknowledgement, nil
	}

	return true, src, acknowledgement, nil
}

//Renew a lease backed on the Acknowledgement Packet.
//Returns Sucessfull, The AcknoledgementPacket, Any Errors
//The ack packet doesn't include the correct details for the DHCP server (Needs reconsidering)
func (c *Client) Renew(dhcpserver net.UDPAddr, acknowledgement dhcp4.Packet) (bool, dhcp4.Packet, error) {
	renewRequest := c.RenewalRequestPacket(&acknowledgement)
	renewRequest.PadToMinSize()

	_, err := c.UnicastPacket(dhcpserver, renewRequest)
	if err != nil {
		return false, renewRequest, err
	}

	newAcknowledgement, _, err := c.GetAcknowledgement(&renewRequest)
	if err != nil {
		return false, newAcknowledgement, err
	}

	newAcknowledgementOptions := newAcknowledgement.ParseOptions()
	if dhcp4.MessageType(newAcknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		return false, newAcknowledgement, nil
	}

	return true, newAcknowledgement, nil
}

//Release a lease backed on the Acknowledgement Packet.
//Returns Any Errors
func (c *Client) Release(dhcpaddr net.UDPAddr, acknowledgement dhcp4.Packet) error {
	release := c.ReleasePacket(&acknowledgement)
	release.PadToMinSize()

	_, err := c.UnicastPacket(dhcpaddr, release)
	return err
}
