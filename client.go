package dhcp4client

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/d2g/dhcp4"
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
	broadcast     bool             //Set the Bcast flag in BOOTP Flags

	connections struct {
		broadcast Conn // Broadcast connection
		unicast   Conn // Unicast connection
	}

	unicast     UnicastFactory //Function Used to obtain a unicast connection. Source, Destination.
	generateXID func([]byte)   //Function Used to Generate a XID
}

// Abstracts the type of underlying socket used
// Altered to more closly represent the net.Conn
// Unable to use net.Conn as only ReadFrom returns the senders address
type Conn interface {
	ReadFrom(b []byte) (int, net.Addr, error)
	Write(b []byte) (int, error)

	Close() error

	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type UnicastFactory func(src, dest net.IP) (Conn, error)

// The connection type can generate a Unicast connection from the existing
// broadcast connection settings etc. In most instances unicast connections can
// flow over the default inetsock connection as the client now has an IP.
type UnicastSwitcher interface {
	UnicastConn() UnicastFactory
}

func New(options ...func(*Client) error) (*Client, error) {
	c := Client{
		timeout:     time.Second * 10,
		broadcast:   true,
		generateXID: CryptoGenerateXID,
	}

	err := c.SetOption(options...)
	if err != nil {
		return nil, err
	}

	//if connection hasn't been set as an option create the default.
	if c.connections.broadcast == nil {
		conn, err := NewInetSock()
		if err != nil {
			return nil, err
		}
		c.connections.broadcast = conn
	}

	if us, ok := c.connections.broadcast.(UnicastSwitcher); ok {
		c.unicast = us.UnicastConn()
	} else {
		//We can't create a new inetsocket as it will start listening?

	}

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

func Broadcast(b bool) func(*Client) error {
	return func(c *Client) error {
		c.broadcast = b
		return nil
	}
}

func Connection(co Conn) func(*Client) error {
	return func(c *Client) error {
		c.connections.broadcast = co
		return nil
	}
}

func GenerateXID(g func([]byte)) func(*Client) error {
	return func(c *Client) error {
		c.generateXID = g
		return nil
	}
}

func Unicast(u UnicastFactory) func(*Client) error {
	return func(c *Client) error {
		c.unicast = u
		return nil
	}
}

//Close Connections
func (c *Client) Close() error {
	var err MultiError

	if c.connections.broadcast != nil {
		err = append(err, c.connections.broadcast.Close())
	}
	if c.connections.unicast != nil {
		err = append(err, c.connections.unicast.Close())
	}

	return err
}

//Returns true if any of the addresses supplied are in the ignore list
func (c *Client) ignoreServer(srcs []net.Addr) bool {
	for _, src := range srcs {
		s, ok := src.(*net.IPAddr)

		// Ignore Servers in my Ignore list
		for _, ignoreServer := range c.ignoreServers {
			if ok && s.IP.Equal(ignoreServer) {
				return true
			}
		}
	}
	return false
}

//Send the Discovery Packet to the Broadcast Channel
func (c *Client) SendDiscoverPacket() (discoveryPacket dhcp4.Packet, err error) {
	discoveryPacket = c.DiscoverPacket()
	discoveryPacket.PadToMinSize()

	_, err = c.BroadcastPacket(discoveryPacket)
	return
}

//Retreive Offer...
//Wait for the offer for a specific Discovery Packet.
func (c *Client) GetOffer(discoverPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	start := time.Now()

	for {
		timeout := c.timeout - time.Since(start)
		if timeout <= 0 {
			return dhcp4.Packet{}, &DHCP4Error{Err: errors.New("Timed Out"), Src: c.connections.broadcast.LocalAddr(), Dest: c.connections.broadcast.RemoteAddr(), IsTimeout: true, IsTemporary: true}
		}

		c.connections.broadcast.SetReadDeadline(time.Now().Add(timeout))

		readBuffer := make([]byte, MaxDHCPLen)
		_, source, err := c.connections.broadcast.ReadFrom(readBuffer)
		if err != nil {
			return dhcp4.Packet{}, &DHCP4Error{Err: err, Src: c.connections.broadcast.LocalAddr(), Dest: c.connections.broadcast.RemoteAddr()}
		}

		offerPacket := dhcp4.Packet(readBuffer)
		offerPacketOptions := offerPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		if c.ignoreServer([]net.Addr{source, &net.IPAddr{IP: offerPacket.SIAddr()}}) {
			continue
		}

		if len(offerPacketOptions[dhcp4.OptionDHCPMessageType]) < 1 || dhcp4.MessageType(offerPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.Offer || !bytes.Equal(discoverPacket.XId(), offerPacket.XId()) {
			continue
		}

		return offerPacket, nil
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
func (c *Client) GetAcknowledgement(requestPacket *dhcp4.Packet) (dhcp4.Packet, error) {
	start := time.Now()

	for {
		timeout := c.timeout - time.Since(start)
		if timeout <= 0 {
			return dhcp4.Packet{}, &DHCP4Error{Err: errors.New("Timed Out"), Src: c.connections.broadcast.LocalAddr(), Dest: c.connections.broadcast.RemoteAddr(), IsTimeout: true, IsTemporary: true}
		}

		err := c.connections.broadcast.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return dhcp4.Packet{}, err
		}

		readBuffer := make([]byte, MaxDHCPLen)
		_, source, err := c.connections.broadcast.ReadFrom(readBuffer)
		if err != nil {
			return dhcp4.Packet{}, err
		}

		acknowledgementPacket := dhcp4.Packet(readBuffer)
		acknowledgementPacketOptions := acknowledgementPacket.ParseOptions()

		// Ignore Servers in my Ignore list
		if c.ignoreServer([]net.Addr{source, &net.IPAddr{IP: acknowledgementPacket.SIAddr()}}) {
			continue
		}

		if !bytes.Equal(requestPacket.XId(), acknowledgementPacket.XId()) || len(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType]) < 1 || (dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK && dhcp4.MessageType(acknowledgementPacketOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.NAK) {
			continue
		}

		return acknowledgementPacket, nil
	}
}

// Send Decline to the received acknowledgement.
func (c *Client) SendDecline(acknowledgementPacket *dhcp4.Packet) (declinePacket dhcp4.Packet, err error) {
	declinePacket = c.DeclinePacket(acknowledgementPacket)
	declinePacket.PadToMinSize()

	_, err = c.BroadcastPacket(declinePacket)
	return
}

// Deprecated, Use BroadcastPacket - Sends a DHCP Packet via the broadcast.
func (c *Client) SendPacket(packet dhcp4.Packet) (err error) {
	_, err = c.connections.broadcast.Write(packet)
	return
}

func (c *Client) BroadcastPacket(packet dhcp4.Packet) (i int, err error) {

	err = c.connections.broadcast.SetWriteDeadline(time.Now().Add(c.timeout))
	if err != nil {
		return
	}

	i, err = c.connections.broadcast.Write(packet)
	return
}

func (c *Client) UnicastPacket(packet dhcp4.Packet) (i int, err error) {
	ncr := true

	if c.connections.unicast != nil {
		laddr, lok := c.connections.unicast.LocalAddr().(*net.IPAddr)
		raddr, rok := c.connections.unicast.RemoteAddr().(*net.IPAddr)

		if lok && rok && laddr.IP.Equal(packet.CIAddr()) && raddr.IP.Equal(packet.SIAddr()) {
			ncr = false
		}

		if ncr {
			c.connections.unicast.Close()
		}
	}

	if ncr {
		c.connections.unicast, err = c.unicast(packet.CIAddr(), packet.SIAddr())
		if err != nil {
			return 0, err
		}
	}

	err = c.connections.unicast.SetWriteDeadline(time.Now().Add(c.timeout))
	if err != nil {
		return
	}

	i, err = c.connections.unicast.Write(packet)
	return
}

//Create Discover Packet
func (c *Client) DiscoverPacket() dhcp4.Packet {
	messageid := make([]byte, 4)
	c.generateXID(messageid)

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(c.hardwareAddr)
	packet.SetXId(messageid)
	packet.SetBroadcast(c.broadcast)

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Discover)})
	return packet
}

//Create Request Packet
func (c *Client) RequestPacket(offerPacket *dhcp4.Packet) dhcp4.Packet {
	offerOptions := offerPacket.ParseOptions()

	packet := dhcp4.NewPacket(dhcp4.BootRequest)
	packet.SetCHAddr(c.hardwareAddr)

	packet.SetXId(offerPacket.XId())
	packet.SetCIAddr(offerPacket.CIAddr())
	packet.SetSIAddr(offerPacket.SIAddr())

	packet.SetBroadcast(c.broadcast)
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
	packet.SetSIAddr(acknowledgement.SIAddr())

	packet.SetBroadcast(c.broadcast)
	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Request)})
	packet.AddOption(dhcp4.OptionRequestedIPAddress, (acknowledgement.YIAddr()).To4())
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

	packet.AddOption(dhcp4.OptionDHCPMessageType, []byte{byte(dhcp4.Decline)})
	packet.AddOption(dhcp4.OptionRequestedIPAddress, (acknowledgement.YIAddr()).To4())
	packet.AddOption(dhcp4.OptionServerIdentifier, acknowledgementOptions[dhcp4.OptionServerIdentifier])

	return packet
}

//Lets do a Full DHCP Request.
func (c *Client) Request() (bool, dhcp4.Packet, error) {
	discoveryPacket, err := c.SendDiscoverPacket()
	if err != nil {
		return false, discoveryPacket, err
	}

	offerPacket, err := c.GetOffer(&discoveryPacket)
	if err != nil {
		return false, offerPacket, err
	}

	requestPacket, err := c.SendRequest(&offerPacket)
	if err != nil {
		return false, requestPacket, err
	}

	acknowledgement, err := c.GetAcknowledgement(&requestPacket)
	if err != nil {
		return false, acknowledgement, err
	}

	acknowledgementOptions := acknowledgement.ParseOptions()
	if dhcp4.MessageType(acknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		return false, acknowledgement, nil
	}

	return true, acknowledgement, nil
}

//Renew a lease backed on the Acknowledgement Packet.
//Returns Sucessfull, The AcknoledgementPacket, Any Errors
func (c *Client) Renew(acknowledgement dhcp4.Packet) (bool, dhcp4.Packet, error) {
	renewRequest := c.RenewalRequestPacket(&acknowledgement)
	renewRequest.PadToMinSize()

	_, err := c.UnicastPacket(renewRequest)
	if err != nil {
		return false, renewRequest, err
	}

	newAcknowledgement, err := c.GetAcknowledgement(&renewRequest)
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
func (c *Client) Release(acknowledgement dhcp4.Packet) error {
	release := c.ReleasePacket(&acknowledgement)
	release.PadToMinSize()

	_, err := c.UnicastPacket(release)
	return err
}
