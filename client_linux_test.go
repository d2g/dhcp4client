package dhcp4client_test

import (
	"net"
	"testing"

	"github.com/d2g/dhcp4client"
	"github.com/d2g/dhcp4client/connections"
	"github.com/d2g/dhcp4client/connections/pktsocket"
	"github.com/krolaw/dhcp4"
)

//Example Client
func Test_ExampleLinuxClient(test *testing.T) {
	var err error

	m, err := net.ParseMAC("08-00-27-DF-83-61")
	if err != nil {
		test.Logf("MAC Error:%v\n", err)
	}

	//Create a connection to use
	c, err := pktsocket.NewPacketSock(2)
	if err != nil {
		test.Fatalf("Client Connection Generation:%s\n", err.Error())
	}
	defer c.Close()

	t := connections.Transport{
		Dialer:   c.Dialer(),
		Listener: c.Listener(),
	}

	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m), dhcp4client.Connection(t))
	if err != nil {
		test.Fatalf("Error:%v\n", err)
	}
	defer exampleClient.Close()

	success := false

	discoveryPacket, err := exampleClient.SendDiscoverPacket()
	if err != nil {
		test.Fatalf("Discovery Error:%v\n", err)
	}

	offerPacket, err := exampleClient.GetOffer(&discoveryPacket)
	if err != nil {
		test.Fatalf("Offer Error:%v\n", err)
	}

	requestPacket, err := exampleClient.SendRequestFromOfferPacket(&offerPacket)
	if err != nil {
		test.Fatalf("Send Offer Error:%v\n", err)
	}

	acknowledgementpacket, err := exampleClient.GetAcknowledgement(&requestPacket)
	if err != nil {
		test.Fatalf("Get Ack Error:%v\n", err)
	}

	acknowledgementOptions := acknowledgementpacket.ParseOptions()
	if dhcp4.MessageType(acknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		test.Fatalf("Not Acknowledged")
	} else {
		success = true
	}

	if !success {
		test.Error("We didn't sucessfully get a DHCP Lease?")
	} else {
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
		test.Logf("Bootstrap or DHCP Server:%v\n", acknowledgementpacket.SIAddr().String())
		test.Logf("Hardware Addr is:%v\n", acknowledgementpacket.CHAddr())
	}

	exampleClient.SetLaddr(&net.UDPAddr{
		IP:   acknowledgementpacket.YIAddr(),
		Port: 68,
	})

	test.Log("Start Renewing Lease")
	success, renewpacket, err := exampleClient.Renew(net.UDPAddr{IP: acknowledgementpacket.YIAddr(), Port: 67}, acknowledgementpacket)
	if err != nil {
		networkError, ok := err.(*net.OpError)
		if ok && networkError.Timeout() {
			test.Log("Renewal Failed! Because it didn't find the DHCP server very Strange")
			test.Errorf("Error" + err.Error())
		}
		test.Fatalf("Error:%v\n", err)
	}

	if !success {
		test.Error("We didn't sucessfully Renew a DHCP Lease?")
	} else {
		test.Logf("IP Received:%v\n", renewpacket.YIAddr().String())
	}
}

func Test_ExampleLinuxClient_Renew(test *testing.T) {

	p := dhcp4.NewPacket(dhcp4.BootRequest)

	m, err := net.ParseMAC("08-00-27-DF-83-61")
	if err != nil {
		test.Logf("MAC Error:%v\n", err)
	}

	//Create a connection to use
	c, err := pktsocket.NewPacketSock(2)
	if err != nil {
		test.Error("Client Connection Generation:" + err.Error())
	}
	defer c.Close()

	t := connections.Transport{
		Dialer:   c.Dialer(),
		Listener: c.Listener(),
	}

	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m), dhcp4client.Connection(t))

	p.SetCHAddr(m)
	p.SetSIAddr(net.IPv4(10, 0, 2, 2))
	p.SetYIAddr(net.IPv4(10, 0, 2, 16))
	p.SetCIAddr(net.IPv4(10, 0, 2, 16))

	test.Log("Start Renewing Lease")
	success, acknowledgementpacket, err := exampleClient.Renew(net.UDPAddr{IP: net.IPv4(10, 0, 2, 2), Port: 68}, p)
	if err != nil {
		networkError, ok := err.(*net.OpError)
		if ok && networkError.Timeout() {
			test.Log("Renewal Failed! Because it didn't find the DHCP server very Strange")
			test.Errorf("Error" + err.Error())
		}
		test.Fatalf("Error:%v\n", err)
	}

	if !success {
		test.Logf("Packet::%v\n", acknowledgementpacket)
		acknowledgementpacketOptions := acknowledgementpacket.ParseOptions()
		test.Logf("ResponseCode::%v\n", acknowledgementpacketOptions[dhcp4.OptionDHCPMessageType][0])
		test.Logf("OpCode::%v\n", acknowledgementpacket.OpCode())

		test.Error("We didn't sucessfully Renew a DHCP Lease?")
	} else {
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}

	success, acknowledgementpacket, err = exampleClient.Renew(net.UDPAddr{IP: net.IPv4(10, 0, 2, 2), Port: 68}, p)
	if err != nil {
		networkError, ok := err.(*net.OpError)
		if ok && networkError.Timeout() {
			test.Log("Renewal Failed! Because it didn't find the DHCP server very Strange")
			test.Errorf("Error" + err.Error())
		}
		test.Fatalf("Error:%v\n", err)
	}

	if !success {
		test.Logf("Packet::%v\n", acknowledgementpacket)
		acknowledgementpacketOptions := acknowledgementpacket.ParseOptions()
		test.Logf("ResponseCode::%v\n", acknowledgementpacketOptions[dhcp4.OptionDHCPMessageType][0])
		test.Logf("OpCode::%v\n", acknowledgementpacket.OpCode())

		test.Error("We didn't sucessfully Renew a DHCP Lease?")
	} else {
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}

}

//Legacy Test
func Test_LegacyLinuxClient(test *testing.T) {
	var err error

	m, err := net.ParseMAC("08-00-27-DF-83-61")
	if err != nil {
		test.Logf("MAC Error:%v\n", err)
	}

	//Create a connection to use
	c, err := dhcp4client.NewPacketSock(2)
	if err != nil {
		test.Fatalf("Client Connection Generation:%s\n", err.Error())
	}
	defer c.Close()

	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m), dhcp4client.Connection(c))
	if err != nil {
		test.Fatalf("Error:%v\n", err)
	}
	defer exampleClient.Close()

	success := false

	discoveryPacket, err := exampleClient.SendDiscoverPacket()
	if err != nil {
		test.Fatalf("Discovery Error:%v\n", err)
	}

	offerPacket, err := exampleClient.GetOffer(&discoveryPacket)
	if err != nil {
		test.Fatalf("Offer Error:%v\n", err)
	}

	requestPacket, err := exampleClient.SendRequestFromOfferPacket(&offerPacket)
	if err != nil {
		test.Fatalf("Send Offer Error:%v\n", err)
	}

	acknowledgementpacket, err := exampleClient.GetAcknowledgement(&requestPacket)
	if err != nil {
		test.Fatalf("Get Ack Error:%v\n", err)
	}

	acknowledgementOptions := acknowledgementpacket.ParseOptions()
	if dhcp4.MessageType(acknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		test.Fatalf("Not Acknowledged")
	} else {
		success = true
	}

	if !success {
		test.Error("We didn't sucessfully get a DHCP Lease?")
	} else {
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
		test.Logf("Bootstrap or DHCP Server:%v\n", acknowledgementpacket.SIAddr().String())
		test.Logf("Hardware Addr is:%v\n", acknowledgementpacket.CHAddr())
	}

	exampleClient.SetLaddr(&net.UDPAddr{
		IP:   acknowledgementpacket.YIAddr(),
		Port: 68,
	})

	test.Log("Start Renewing Lease")
	success, renewpacket, err := exampleClient.Renew(net.UDPAddr{IP: acknowledgementpacket.YIAddr(), Port: 67}, acknowledgementpacket)
	if err != nil {
		networkError, ok := err.(*net.OpError)
		if ok && networkError.Timeout() {
			test.Log("Renewal Failed! Because it didn't find the DHCP server very Strange")
			test.Errorf("Error" + err.Error())
		}
		test.Fatalf("Error:%v\n", err)
	}

	if !success {
		test.Error("We didn't sucessfully Renew a DHCP Lease?")
	} else {
		test.Logf("IP Received:%v\n", renewpacket.YIAddr().String())
	}
}
