// This package provides a wrapper around some of the UDP functions
// in the network library.
// They allow selective dropping of packets, as well as monitoring
// of packet traffic

package lspnet

import (
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"math/rand"
	"net"
)

// Useful parameters

var readDropPercent int = 0  // Fraction of packets to drop when reading
var writeDropPercent int = 0 // Fraction of packets to drop when writing

// Special functions to set network parameters

func SetReadDropPercent(p int) {
	if p < 0 || p > 100 {
		readDropPercent = 0
	} else {
		readDropPercent = p
	}
}

func SetWriteDropPercent(p int) {
	if p < 0 || p > 100 {
		writeDropPercent = 0
	} else {
		writeDropPercent = p
	}
}

type UDPAddr net.UDPAddr

// Duplicate features of net.UDPConn data structure, while adding other parameters
type UDPConn struct {
	// Import all fields from net.UDPConn
	ncon *net.UDPConn
}

// Wrappers around standard network functions

func ResolveUDPAddr(ntwk, addr string) (*UDPAddr, error) {
	a, err := net.ResolveUDPAddr(ntwk, addr)
	if err == nil {
		return &UDPAddr{IP: a.IP, Port: a.Port}, err
	}
	return nil, err
}

func (addr *UDPAddr) String() string {
	naddr := &net.UDPAddr{IP: addr.IP, Port: addr.Port}
	return naddr.String()
}

func DialUDP(ntwk string, laddr, raddr *UDPAddr) (*UDPConn, error) {
	var nladdr *net.UDPAddr = nil
	if laddr != nil {
		nladdr = &net.UDPAddr{IP: laddr.IP, Port: laddr.Port}
	}
	var nraddr *net.UDPAddr = nil
	if raddr != nil {
		nraddr = &net.UDPAddr{IP: raddr.IP, Port: raddr.Port}
	}
	ncon, err := net.DialUDP(ntwk, nladdr, nraddr)
	rcon := &UDPConn{ncon}
	return rcon, err
}

func ListenUDP(ntwk string, laddr *UDPAddr) (*UDPConn, error) {
	var nladdr *net.UDPAddr = nil
	if laddr != nil {
		nladdr = &net.UDPAddr{IP: laddr.IP, Port: laddr.Port}
	}
	ncon, err := net.ListenUDP(ntwk, nladdr)
	rcon := &UDPConn{ncon}
	return rcon, err
}

func (con *UDPConn) ReadFromUDP(b []byte) (n int, addr *UDPAddr, err error) {
	var buffer [2000]byte
	ncon := con.ncon
	var naddr *net.UDPAddr
	n, naddr, err = ncon.ReadFromUDP(buffer[0:])
	if dropit(readDropPercent) {
		lsplog.Vlogf(5, "UDP: DROPPING read packet of length %v\n", n)
	} else {
		lsplog.Vlogf(6, "UDP: Read packet of length %v\n", n)
		copy(b, buffer[0:])
	}
	if naddr == nil {
		addr = nil
	} else {
		addr = &UDPAddr{IP: naddr.IP, Port: naddr.Port}
	}
	return n, addr, err
}

func (con *UDPConn) Write(b []byte) (int, error) {
	ncon := con.ncon
	if dropit(writeDropPercent) {
		lsplog.Vlogf(5, "UDP: DROPPING written packet of length %v\n", len(b))
		// Make it look like write was successful
		return len(b), nil
	} else {
		n, err := ncon.Write(b)
		lsplog.Vlogf(5, "UDP: Wrote packet of length %v\n", n)
		return n, err
	}
	return 0, nil
}

func (con *UDPConn) WriteToUDP(b []byte, addr *UDPAddr) (int, error) {
	ncon := con.ncon
	naddr := &net.UDPAddr{IP: addr.IP, Port: addr.Port}
	if dropit(writeDropPercent) {
		lsplog.Vlogf(5, "UDP: DROPPING written packet of length %v\n", len(b))
		// Make it look like write was successful
		return len(b), nil
	} else {
		n, err := ncon.WriteToUDP(b, naddr)
		lsplog.Vlogf(5, "UDP: Wrote packet of length %v", n)
		return n, err
	}
	return 0, nil
}

func (con *UDPConn) Close() error {
	ncon := con.ncon
	return ncon.Close()
}

func dropit(dropPercent int) bool {
	return (rand.Intn(100) < dropPercent)
}
