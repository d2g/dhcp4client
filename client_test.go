package dhcp4client

import (
	"log"
	"net"
	"testing"
	"time"
)

/*
 * Example Client
 */
func Test_ExampleClient(test *testing.T) {
	var err error
	exampleClient := Client{}
	exampleClient.IgnoreServers = []net.IP{}
	exampleClient.MACAddress, err = net.ParseMAC("08-00-27-00-A8-E8")
	if err != nil {
		log.Printf("MAC Error:%v\n", err)
	}
	exampleClient.Timeout = (time.Duration(10) * time.Second)
	exampleClient.Connect()
	success, acknowledgementpacket, err := exampleClient.Request()

	test.Logf("Success:%v\n", success)
	test.Logf("Packet:%v\n", acknowledgementpacket)

	if err != nil {
		networkError, ok := err.(*net.OpError)
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
