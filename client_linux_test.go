package dhcp4client_test

import (
	"net"
	"testing"

	"github.com/d2g/dhcp4"
	"github.com/d2g/dhcp4client"
	"github.com/d2g/dhcp4client/connections/pktsocket"
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
		test.Error("Client Connection Generation:" + err.Error())
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

	offerPacket, src, err := exampleClient.GetOffer(&discoveryPacket)
	if err != nil {
		test.Fatalf("Offer Error:%v\n", err)
	}

	// As the client I select the DHCP server to respond to.
	// The Server itself stops telling us it's address so we need to remember it for renewals.
	// SIAddr isn't the DHCP SERVER it's the next server in the boot process.
	test.Logf("DHCP Server:%v\n", src.String())

	requestPacket, err := exampleClient.SendRequest(&offerPacket)
	if err != nil {
		test.Fatalf("Send Offer Error:%v\n", err)
	}

	acknowledgementpacket, _, err := exampleClient.GetAcknowledgement(&requestPacket)
	if err != nil {
		test.Fatalf("Get Ack Error:%v\n", err)
	}

	acknowledgementOptions := acknowledgementpacket.ParseOptions()
	if dhcp4.MessageType(acknowledgementOptions[dhcp4.OptionDHCPMessageType][0]) != dhcp4.ACK {
		test.Fatalf("Not Acknoledged")
	} else {
		success = true
	}

	if !success {
		test.Error("We didn't sucessfully get a DHCP Lease?")
	} else {
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
		test.Logf("Bootstrap or DHCP Server:%v\n", acknowledgementpacket.SIAddr().String())
		// Modify the ackpackage to set the correct server address.
		acknowledgementpacket.SetSIAddr(src)
		test.Logf("Hardware Addr is:%v\n", acknowledgementpacket.CHAddr())
	}

	test.Log("Start Renewing Lease")
	success, renewpacket, err := exampleClient.Renew(src, acknowledgementpacket)
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

	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m), dhcp4client.Connection(c))

	p.SetCHAddr(m)
	p.SetSIAddr(net.IPv4(10, 0, 2, 2))
	p.SetYIAddr(net.IPv4(10, 0, 2, 16))

	test.Log("Start Renewing Lease")
	success, acknowledgementpacket, err := exampleClient.Renew(net.IPv4(10, 0, 2, 2), p)
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
