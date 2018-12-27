package dhcp4client_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/d2g/dhcp4client"
	"github.com/d2g/dhcp4client/connections"
	"github.com/d2g/dhcp4client/connections/testsocket"
	"github.com/krolaw/dhcp4"
)

//Basic usage example of using the client to complete a DHCP4 request and getting the
//allocated IP.
func ExampleClient() {
	var err error

	m, err := net.ParseMAC("E4-B3-18-64-DC-14")
	if err != nil {
		fmt.Printf("Error: Error Parsing MAC Address:%v\n", err)
		return
	}

	//Create a connection to use
	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m))
	if err != nil {
		fmt.Printf("Error: Error Creating Client:%v\n", err)
		return
	}
	defer exampleClient.Close()

	//Complete a full DHCP4 Cycle, Discover, Request, Offer, Acknowledgement.
	success, acknowledgementpacket, err := exampleClient.Request()

	if err != nil {
		networkError, ok := err.(net.Error)
		if ok && networkError.Timeout() {
			fmt.Printf("Warning: Didn't get a valid respose from a DHCP server in time.")
			return
		}
		fmt.Printf("Error: Network error when making request %v\n", err)
		return
	}

	if !success {
		fmt.Printf("Warning: We didn't sucessfully get a DHCP Lease")
	} else {
		fmt.Printf("IP address received which can be found in acknowledgementpacket.YIAddr()")

		//Print our IP to stderr as the basic example will get any IP so we can't
		//included it in the tested output.
		fmt.Errorf("Info: IP Received:%s\n", acknowledgementpacket.YIAddr())
	}

	//Output:
	//IP address received which can be found in acknowledgementpacket.YIAddr()
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
		IP:   net.IPv4(10, 205, 21, 92),
		Port: 68,
	})

	p := dhcp4.NewPacket(dhcp4.BootRequest)
	p.SetCHAddr(m)
	p.SetYIAddr(net.IPv4(10, 205, 21, 92))
	p.AddOption(dhcp4.OptionServerIdentifier, net.IPv4(10, 210, 31, 214))

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

//First Test With Dummy Socket
func Test_DummySocket(t *testing.T) {
	var err error

	m, err := net.ParseMAC("AA-AA-AA-AA-AA-AA")
	if err != nil {
		t.Fatalf("MAC Error:%v\n", err)
	}

	s := testsocket.TestSocket{}
	tc := connections.Transport{
		Dialer:   s.Dialer(),
		Listener: s.Listener(),
	}

	//Create a dummy connection to use
	c, err := dhcp4client.New(dhcp4client.HardwareAddr(m), dhcp4client.Connection(tc))
	if err != nil {
		t.Fatalf("Error:%v\n", err)
	}

	success, acknowledgementpacket, err := c.Request()
	if err != nil {
		t.Logf("Success:%v\n", success)
		t.Logf("Packet:%v\n", acknowledgementpacket)
		t.Fatalf("Error:%v\n", err)
	}

	//Check we're sucessful
	if !success {
		t.Fatalf("We didn't sucessfully get a DHCP Lease?")
	}

	//Check Our IP
	if !testsocket.CLIENTIP.Equal(acknowledgementpacket.YIAddr()) {
		t.Fatalf("Expected IP %v got %v\n", testsocket.CLIENTIP, acknowledgementpacket.YIAddr())
	}

	//Check Our Server
	acknowledgementOptions := acknowledgementpacket.ParseOptions()
	if !testsocket.SERVERIP.Equal(net.IP(acknowledgementOptions[dhcp4.OptionServerIdentifier])) {
		t.Fatalf("Expected Option54 Server IP %v got %v\n", testsocket.SERVERIP, net.IP(acknowledgementOptions[dhcp4.OptionServerIdentifier]))
	}

	//Renewal
	success, acknowledgementpacket, err = c.Renew(net.UDPAddr{IP: acknowledgementpacket.YIAddr(), Port: 67}, acknowledgementpacket)
	if err != nil {
		t.Logf("Renewal Success:%v\n", success)
		t.Logf("Renewal Packet:%v\n", acknowledgementpacket)
		t.Fatalf("Error:%v\n", err)
	}

	if !success {
		t.Fatalf("We didn't sucessfully get a DHCP Lease on Renewal")
	}

	//Check Our IP
	if !testsocket.CLIENTIP.Equal(acknowledgementpacket.YIAddr()) {
		t.Fatalf("Renewal Expected IP %v got %v\n", testsocket.CLIENTIP, acknowledgementpacket.YIAddr())
	}

}

func Test_ClientWithOptions(t *testing.T) {
	var err error

	m, err := net.ParseMAC("E4-B3-18-64-DC-14")
	if err != nil {
		t.Logf("MAC Error:%v\n", err)
	}

	exampleClient, err := dhcp4client.New(dhcp4client.HardwareAddr(m))
	if err != nil {
		t.Fatalf("Error:%v\n", err)
	}
	defer exampleClient.Close()

	//Build Our Hostname Options
	//My Hostname
	hostname := []byte("D2GClient")
	opts := make(dhcp4client.DHCP4ClientOptions)

	//I want to send the dhcp4.OptionHostName
	opts[dhcp4.Discover] = append(opts[dhcp4.Discover], &dhcp4.Option{Code: dhcp4.OptionHostName, Value: hostname})
	opts[dhcp4.Request] = append(opts[dhcp4.Request], &dhcp4.Option{Code: dhcp4.OptionHostName, Value: hostname})

	//I want to receive the dhcp4.OptionHostName from the reply
	opts[dhcp4.Offer] = append(opts[dhcp4.Offer], &dhcp4.Option{Code: dhcp4.OptionHostName})
	opts[dhcp4.ACK] = append(opts[dhcp4.ACK], &dhcp4.Option{Code: dhcp4.OptionHostName})

	t.Logf("Options:%+v\n", opts)
	success, acknowledgementpacket, err := exampleClient.RequestWithOptions(opts)
	t.Logf("Options:%v\n", opts)

	t.Logf("Success:%v\n", success)
	t.Logf("Packet:%v\n", acknowledgementpacket)

	if err != nil {
		networkError, ok := err.(net.Error)
		if ok && networkError.Timeout() {
			t.Log("Test Skipping as it didn't find a DHCP Server")
			t.SkipNow()
		}
		t.Fatalf("Error:%v\n", err)
	}

	if !success {
		t.Error("We didn't sucessfully get a DHCP Lease?")
	} else {
		t.Logf("IP Received:%v\n", acknowledgementpacket.YIAddr().String())
	}
}
