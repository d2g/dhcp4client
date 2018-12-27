package dhcp4client

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/d2g/dhcp4client/connections"
	"github.com/d2g/dhcp4client/connections/inetsocket"
	"github.com/krolaw/dhcp4"
)

const (
	//The Maximum size of a DHCP4 response. Used to initialise read buffers.
	MaxDHCPLen = 576
)

// An Error Type with aditoin DHCP4 information.
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

// DHCP4ClientOptions is used to GET & SET custom DHCP Options in the requests
// and response packets. The dhcp4.Options is passed in with the values you want
// to set and those that you want to retreive. The reason that the value is a map
// is so that all options can be GET at any stage of the request lifecycle in a
// single call to the RequestWithOptions().
type DHCP4ClientOptions map[dhcp4.MessageType][]*dhcp4.Option

//TODO(d2g): Tidy Up
func (o DHCP4ClientOptions) String() string {

	//	Discover
	//	Offer
	//	Request
	//	Decline
	//	ACK
	//	NAK
	//	Release
	//	Inform
	output := ""

	output += "Discover["
	for _, v := range o[dhcp4.Discover] {
		output += fmt.Sprintf("%+v,", v)
	}
	output += "],"

	output += "Offer["
	for _, v := range o[dhcp4.Offer] {
		output += fmt.Sprintf("%+v,", v)
	}
	output += "],"

	output += "Request["
	for _, v := range o[dhcp4.Request] {
		output += fmt.Sprintf("%+v,", v)
	}
	output += "],"

	output += "Decline["
	for _, v := range o[dhcp4.Decline] {
		output += fmt.Sprintf("%+v,", v)
	}
	output += "],"

	output += "ACK["
	for _, v := range o[dhcp4.ACK] {
		output += fmt.Sprintf("%+v,", v)
	}
	output += "],"

	output += "NAK["
	for _, v := range o[dhcp4.NAK] {
		output += fmt.Sprintf("%+v,", v)
	}
	output += "],"

	output += "Release["
	for _, v := range o[dhcp4.Release] {
		output += fmt.Sprintf("%+v,", v)
	}
	output += "],"

	output += "Inform["
	for _, v := range o[dhcp4.Inform] {
		output += fmt.Sprintf("%+v,", v)
	}
	output += "],"

	return output
}

// The Main DHCP Control Client
type Client struct {
	hardwareAddr net.HardwareAddr //The HardwareAddr to send in the request.

	// List of Servers to Ignore requests from. This is compared against the
	// dhcp4.OptionServerIdentifier (Code 54) passed back in the response
	ignoreServers []net.IP

	timeout    time.Duration         //Time before we timeout.
	connection connections.Transport //Where connections are sourced

	// Local Clients UDP Address (IP, PORT), this doesn't change automatically during the request.
	// For example if you want to renew you should update the clients laddr
	// prior to making the renew request.
	laddr net.UDPAddr

	//Remote Broadcast Address. This is used in braodcasts but not unicast messages.
	broadcastaddr net.UDPAddr

	generateXID func([]byte) //Function Used to Generate a XID
}

// Creates a DHCP4Client instance using functional options to configure settings.
// More information about function options [https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis]
func New(options ...func(*Client) error) (*Client, error) {
	d := inetsocket.InetSock{}

	c := Client{
		timeout: time.Second * 10,
		connection: connections.Transport{
			Dialer:   d.Dialer(),
			Listener: d.Listener(),
		},
		laddr:         net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 68},
		broadcastaddr: net.UDPAddr{IP: net.IPv4bcast, Port: 67},
	}

	err := c.SetOption(options...)
	if err != nil {
		return nil, err
	}

	if c.generateXID == nil {
		// https://tools.ietf.org/html/rfc2131#section-4.1 explains:
		//
		// A DHCP client MUST choose 'xid's in such a way as to minimize the chance
		// of using an 'xid' identical to one used by another client.
		//
		// Hence, seed a random number generator with the current time and hardware
		// address.
		h := fnv.New64()
		h.Write(c.hardwareAddr)
		seed := int64(h.Sum64()) + time.Now().Unix()
		rnd := rand.New(rand.NewSource(seed))
		var rndMu sync.Mutex
		c.generateXID = func(b []byte) {
			rndMu.Lock()
			defer rndMu.Unlock()
			rnd.Read(b)
		}
	}

	//if connection hasn't been set as an option create the default.
	return &c, nil
}

// SetOptions allows you to configure the client. Generally you should be setting
// the functional options on the creation of the instance.
func (c *Client) SetOption(options ...func(*Client) error) error {
	for _, opt := range options {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

// Timout is a functional options that lets you set all network timeouts on the client.
func Timeout(t time.Duration) func(*Client) error {
	return func(c *Client) error {
		c.timeout = t
		return nil
	}
}

// IgnoreServers is a function option that allow you to pass an array s of IP's
// which the client will ignore when the DHCP Server respond with a
// dhcp4.OptionServerIdentifier (Code 54) in this IP array.
func IgnoreServers(s []net.IP) func(*Client) error {
	return func(c *Client) error {
		c.ignoreServers = s
		return nil
	}
}

// HardwareAddr is a function option to set the Hardware(MAC) address of the client.
func HardwareAddr(h net.HardwareAddr) func(*Client) error {
	return func(c *Client) error {
		c.hardwareAddr = h
		return nil
	}
}

// Connection  is a function option that allows you to configure which
// connection(s) type to utilise.
func Connection(co connections.Transport) func(*Client) error {
	return func(c *Client) error {
		c.connection = co
		return nil
	}
}

// GenerateXID is a function option that allows you to set the function
// responsible for producing a random XID.
func GenerateXID(g func([]byte)) func(*Client) error {
	return func(c *Client) error {
		c.generateXID = g
		return nil
	}
}

// SetLaddr allows you to change the Local Address used by the Client to make request.
// This is normally used after you've obtained an IP and brought your interface up to
// make renewal requests to the DHCP Server.
func (c *Client) SetLaddr(l *net.UDPAddr) {
	c.laddr = *l
}

// BroadcastPacket Allows you to Broadcast the DHCP4 packets over the UDP Connection.
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

// UnicastPacket Allows you to Unicast Packets to the server passed in the
// dhcp4.OptionServerIdentifier DHCP4 Packet option.
// BUG(d2g): Hardcoded to contect the server on port 67.
func (c *Client) UnicastPacket(p dhcp4.Packet) (i int, err error) {

	opt := p.ParseOptions()
	dhcpAddr := net.UDPAddr{
		IP:   opt[dhcp4.OptionServerIdentifier],
		Port: 67,
	}

	con, err := c.connection.Dial(&c.laddr, &dhcpAddr)
	if err != nil {
		return
	}
	defer con.Close()

	con.SetWriteDeadline(time.Now().Add(c.timeout))

	i, err = con.Write(p)
	return
}

// Close function closes the Client.
func (c *Client) Close() error {
	//TODO: For later use for informing the connections.transport to close any pooling.
	return nil
}

// ignoreServer takes an array of IPs and returns true if any of them are in the
// array of ignoreServers.
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

//Renew a lease backed on the Acknowledgement Packet.
//Returns Sucessfull, The AcknoledgementPacket, Any Errors
//BUG(d2g): Which directly contradicts [RFC 2131 p28]
//BUG(d2g): ACK packet doesn't have to contain the server identifier. https://tools.ietf.org/html/rfc2132#section-9.7
func (c *Client) Renew(dhcpaddr net.UDPAddr, acknowledgement dhcp4.Packet) (bool, dhcp4.Packet, error) {
	renewRequest := c.RenewalRequestPacketFromAcknowledgment(&acknowledgement)
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

///////////////////////////////////////////////////

// Lets do a Full DHCP Request.
// The Returned UDP Address is the content of Option 54 of the Offer packet.
// TODO:
func (c *Client) Request() (bool, dhcp4.Packet, error) {
	discoveryPacket, err := c.SendDiscoverPacket()
	if err != nil {
		return false, discoveryPacket, err
	}

	offerPacket, err := c.GetOffer(&discoveryPacket)
	if err != nil {
		return false, offerPacket, err
	}

	requestPacket, err := c.SendRequestFromOfferPacket(&offerPacket)
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

// Full DCHP Call With Client Options
// The Options Should Contain the requested values and the responded values
func (c *Client) RequestWithOptions(opts DHCP4ClientOptions) (bool, dhcp4.Packet, error) {
	discoveryPacket, err := c.SendDiscoverPacketWithOptions(opts)
	if err != nil {
		return false, discoveryPacket, err
	}

	offerPacket, err := c.GetOfferWithOptions(discoveryPacket.XId(), opts)
	if err != nil {
		return false, offerPacket, err
	}

	requestPacket, err := c.SendRequestFromOfferPacketWithOptions(&offerPacket, opts)
	if err != nil {
		return false, requestPacket, err
	}

	acknowledgement, err := c.GetAcknowledgementWithOptions(&requestPacket, opts)
	if err != nil {
		return false, acknowledgement, err
	}

	acknowledgementOptions := acknowledgement.ParseOptions()
	if dhcp4.MessageType(acknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		return false, acknowledgement, nil
	}

	return true, acknowledgement, nil
}
