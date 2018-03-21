package dhcp4client_test

import (
	"log"
	"net"
	"testing"

	"github.com/d2g/dhcp4client"
	"github.com/d2g/dhcp4client/pktsocket"
)

//Example Client
func Test_ExampleLinuxClient(test *testing.T) {
	var err error

	m, err := net.ParseMAC("08-00-27-00-A8-E8")
	if err != nil {
		log.Printf("MAC Error:%v\n", err)
	}

	//Create a connection to use
	c, err := pktsocket.NewPacketSock(1)
	if err != nil {
		test.Error("Client Connection Generation:" + err.Error())
	}
	defer c.Close()

	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m), dhcp4client.Connection(c))
	if err != nil {
		test.Fatalf("Error:%v\n", err)
	}
	defer exampleClient.Close()

	success, acknowledgementpacket, err := exampleClient.Request()

	test.Logf("Success:%v\n", success)
	test.Logf("Packet:%v\n", acknowledgementpacket)

	if err != nil {
		networkError, ok := err.(net.Error)
		if ok && networkError.Timeout() {
			test.Log("Test Skipping as it didn't find a DHCP Server")
			test.SkipNow()
		}
		test.Fatalf("Error:%v\n", err)
	}

	if !success {
		test.Error("We didn't sucessfully get a DHCP Lease?")
	} else {
		log.Printf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}

	test.Log("Start Renewing Lease")
	success, acknowledgementpacket, err = exampleClient.Renew(acknowledgementpacket)
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
		log.Printf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}

}
