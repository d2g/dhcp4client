package pktsocket

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"net"
	"time"

	"github.com/d2g/dhcp4client/connections"
	"golang.org/x/sys/unix"
)

const (
	minIPHdrLen = 20
	maxIPHdrLen = 60
	udpHdrLen   = 8
	ip4Ver      = 0x40
	ttl         = 16
)

var (
	bcastMAC = []byte{255, 255, 255, 255, 255, 255}
)

// abstracts AF_PACKET
type PacketSock struct {
	fd int

	ifindex int
	laddr   *net.UDPAddr
	raddr   *net.UDPAddr

	randFunc func(p []byte) (n int, err error)
}

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

//ifindex int
func NewPacketSock(ifindex int, options ...func(*PacketSock) error) (*PacketSock, error) {

	c := &PacketSock{
		laddr:    &net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 68},
		randFunc: rand.Read,
		ifindex:  ifindex,
	}

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM, int(swap16(unix.ETH_P_IP)))
	if err != nil {
		return nil, err
	}

	c.fd = fd

	//Functional Options?
	err = c.setOption(options...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *PacketSock) setOption(options ...func(*PacketSock) error) error {
	for _, opt := range options {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

func RandFunc(f func(p []byte) (n int, err error)) func(*PacketSock) error {
	return func(ps *PacketSock) error {
		ps.randFunc = f
		return nil
	}
}

func (pc *PacketSock) LocalAddr() *net.UDPAddr {
	return pc.laddr
}

func (pc *PacketSock) RemoteAddr() *net.UDPAddr {
	return pc.raddr
}

func (pc *PacketSock) Close() error {
	return unix.Close(pc.fd)
}

// Unix.SendTo returns an error when the network is down. Which it obviously isn't an error because we're bring the network up.
func (pc *PacketSock) Write(packet []byte) (int, error) {
	lladdr := unix.SockaddrLinklayer{
		Ifindex:  pc.ifindex,
		Protocol: swap16(unix.ETH_P_IP),
		Halen:    uint8(len(bcastMAC)),
	}
	copy(lladdr.Addr[:], bcastMAC)

	pkt := make([]byte, minIPHdrLen+udpHdrLen+len(packet))

	pc.fillIPHdr(pkt[0:minIPHdrLen], udpHdrLen+uint16(len(packet)))
	pc.fillUDPHdr(pkt[minIPHdrLen:minIPHdrLen+udpHdrLen], uint16(len(packet)))

	// payload
	copy(pkt[minIPHdrLen+udpHdrLen:], packet)

	// TODO Look at how to return the correct length written.
	return 0, unix.Sendto(pc.fd, pkt, 0, &lladdr)
}

func (pc *PacketSock) ReadFrom(b []byte) (int, *net.UDPAddr, error) {
	pkt := make([]byte, maxIPHdrLen+udpHdrLen+len(b))
	n, _, err := unix.Recvfrom(pc.fd, pkt, 0)
	if err != nil {
		return 0, nil, err
	}

	// IP hdr len
	ihl := int(pkt[0]&0x0F) * 4

	// Source IP address
	// TODO: Add Source PORT
	src := net.UDPAddr{
		IP: net.IPv4(pkt[12], pkt[13], pkt[14], pkt[15]),
	}

	// TODO is there a better way of doing this without a copy?
	copy(b, pkt[ihl+udpHdrLen:n:len(b)])

	return (n - (ihl + udpHdrLen)), &src, nil
}

func (pc *PacketSock) SetDeadline(t time.Time) error {
	var err MultiError
	err = append(err, pc.SetReadDeadline(t))
	err = append(err, pc.SetWriteDeadline(t))
	return err
}

func (pc *PacketSock) SetReadDeadline(t time.Time) error {
	remain := t.Sub(time.Now())
	tv := unix.NsecToTimeval(remain.Nanoseconds())
	return unix.SetsockoptTimeval(pc.fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
}

func (pc *PacketSock) SetWriteDeadline(t time.Time) error {
	remain := t.Sub(time.Now())
	tv := unix.NsecToTimeval(remain.Nanoseconds())
	return unix.SetsockoptTimeval(pc.fd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &tv)
}

func (pc *PacketSock) fillIPHdr(hdr []byte, payloadLen uint16) {
	// version + IHL
	hdr[0] = ip4Ver | (minIPHdrLen / 4)
	// total length
	binary.BigEndian.PutUint16(hdr[2:4], uint16(len(hdr))+payloadLen)
	// identification
	if _, err := pc.randFunc(hdr[4:5]); err != nil {
		panic(err)
	}
	// TTL
	hdr[8] = 16
	// Protocol
	hdr[9] = unix.IPPROTO_UDP
	// src IP
	copy(hdr[12:16], pc.laddr.IP.To4())
	// dst IP
	copy(hdr[16:20], pc.raddr.IP.To4())
	// compute IP hdr checksum
	chksum(hdr[0:], hdr[10:12])
}

func (pc *PacketSock) fillUDPHdr(hdr []byte, payloadLen uint16) {
	// src port
	binary.BigEndian.PutUint16(hdr[0:2], uint16(pc.laddr.Port))
	// dest port
	binary.BigEndian.PutUint16(hdr[2:4], uint16(pc.raddr.Port))
	// length
	binary.BigEndian.PutUint16(hdr[4:6], udpHdrLen+payloadLen)
}

func (pc *PacketSock) Dialer() func(*net.UDPAddr, *net.UDPAddr) (connections.UDPConn, error) {
	return func(l *net.UDPAddr, r *net.UDPAddr) (connections.UDPConn, error) {
		//Build a new packet socket with the current connection and return it?
		npc := PacketSock{
			fd:       pc.fd,
			ifindex:  pc.ifindex,
			laddr:    l,
			raddr:    r,
			randFunc: pc.randFunc,
		}

		return &npc, nil
	}
}

func (pc *PacketSock) Listener() func(*net.UDPAddr) (connections.UDPConn, error) {
	return func(l *net.UDPAddr) (connections.UDPConn, error) {

		npc := PacketSock{
			fd:       pc.fd,
			ifindex:  pc.ifindex,
			laddr:    l,
			raddr:    &net.UDPAddr{IP: net.IPv4bcast, Port: 67},
			randFunc: pc.randFunc,
		}

		addr := unix.SockaddrLinklayer{
			Ifindex:  npc.ifindex,
			Protocol: swap16(unix.ETH_P_IP),
		}

		if err := unix.Bind(npc.fd, &addr); err != nil {
			return nil, err
		}

		return &npc, nil
	}
}

// compute's 1's complement checksum
func chksum(p []byte, csum []byte) {
	cklen := len(p)
	s := uint32(0)
	for i := 0; i < (cklen - 1); i += 2 {
		s += uint32(p[i+1])<<8 | uint32(p[i])
	}
	if cklen&1 == 1 {
		s += uint32(p[cklen-1])
	}
	s = (s >> 16) + (s & 0xffff)
	s = s + (s >> 16)
	s = ^s

	csum[0] = uint8(s & 0xff)
	csum[1] = uint8(s >> 8)
}

func swap16(x uint16) uint16 {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], x)
	return binary.LittleEndian.Uint16(b[:])
}
