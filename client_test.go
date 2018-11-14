package dhcp4client_test

import (
	"net"
	"testing"

	"github.com/d2g/dhcp4client"
	"github.com/krolaw/dhcp4"
)

//Example Client
func Test_ExampleClient(test *testing.T) {
	var err error

	m, err := net.ParseMAC("E4-B3-18-64-DC-14")
	if err != nil {
		test.Logf("MAC Error:%v\n", err)
	}

	//Create a connection to use
	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m))
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
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}

	acknowledgementOptions := acknowledgementpacket.ParseOptions()
	test.Logf("Option 54:%v\n", acknowledgementOptions[dhcp4.OptionServerIdentifier])

	test.Log("Start Renewing Lease")
	exampleClient.SetLaddr(&net.UDPAddr{
		IP:   acknowledgementpacket.YIAddr(),
		Port: 68,
	})

	//success, acknowledgementpacket, err = exampleClient.Renew(*srv, acknowledgementpacket)
	//if err != nil {
	//	networkError, ok := err.(*net.OpError)
	//		if ok && networkError.Timeout() {
	//			test.Log("Renewal Failed! Because it didn't find the DHCP server very Strange")
	//			test.Errorf("Error" + err.Error())
	//		}
	//		test.Fatalf("Error:%v\n", err)
	//	}

	if !success {
		test.Error("We didn't sucessfully Renew a DHCP Lease?")
	} else {
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}

}

func Test_Renew(test *testing.T) {
	var err error

	m, err := net.ParseMAC("E4-B3-18-64-DC-14")
	if err != nil {
		test.Logf("MAC Error:%v\n", err)
	}

	//Create a connection to use
	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m))
	if err != nil {
		test.Fatalf("Error:%v\n", err)
	}
	defer exampleClient.Close()

	test.Log("Start Renewing Lease")
	exampleClient.SetLaddr(&net.UDPAddr{
		IP:   net.IPv4(10, 162, 222, 167),
		Port: 68,
	})

	p := dhcp4.NewPacket(dhcp4.BootRequest)
	p.SetCHAddr(m)
	p.SetYIAddr(net.IPv4(10, 162, 222, 167))
	p.AddOption(dhcp4.OptionServerIdentifier, net.IPv4(10, 128, 128, 128))

	success, acknowledgementpacket, err := exampleClient.Renew(net.UDPAddr{IP: net.IPv4(10, 128, 128, 128), Port: 67}, p)
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
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}

}

//Example Client (With MathGID)
func Test_ExampleClientWithMathGenerateXID(test *testing.T) {
	var err error

	m, err := net.ParseMAC("E4-B3-18-64-DC-14")
	if err != nil {
		test.Logf("MAC Error:%v\n", err)
	}

	//Create a connection to use
	//We need to set the connection ports to 1068 and 1067 so we don't need root access

	// If you ar using MathGenerateXID then you are responsible for seeding math/rand
	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m), dhcp4client.GenerateXID(dhcp4client.MathGenerateXID))
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
		test.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}
}
