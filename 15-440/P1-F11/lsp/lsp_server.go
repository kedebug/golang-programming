package lsp

import (
	"encoding/json"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
	"time"
)

type server struct {
	nextConnId      uint16
	connMap         map[string]*lspConn
	params          *LspParams
	udpConn         *lspnet.UDPConn
	udpAddr         *lspnet.UDPAddr
	netReadChan     chan *udpPacket
	appWriteChan    chan *LspMsg
	closeChan       chan uint16
	epochNotifyChan chan int
	epochQuitChan   chan int
	readQuitChan    chan int
	serveQuitChan   chan int
}

type udpPacket struct {
	msg  *LspMsg
	addr *lspnet.UDPAddr
}

func newLspServer(port int, params *LspParams) (*LspServer, error) {
	if params == nil {
		params = &LspParams{5, 2000}
	}
	hostport := fmt.Sprintf(":%v", port)
	addr, err := lspnet.ResolveUDPAddr("udp", hostport)
	if lsplog.CheckReport(1, err) {
		return nil, err
	}
	udpconn, err := lspnet.ListenUDP("udp", addr)
	if lsplog.CheckReport(1, err) {
		return nil, err
	}
	srv := &LspServer{
		server{
			nextConnId: 1,
			params:     params,
			connMap:    make(map[string]*lspConn),
			udpConn:    udpconn,
			udpAddr:    addr,
		},
	}
	go srv.loopServe()
	go srv.loopRead()
	go epochTrigger(params.EpochMilliseconds, srv.epochNotifyChan, srv.epochQuitChan)
	return srv, nil
}

func (srv *LspServer) loopServe() {
	for {
		select {
		case p := <-srv.netReadChan:
			srv.handleUdpPacket(p)
		case msg := <-srv.appWriteChan:
			conn := srv.getConnById(msg.ConnId)
			conn.sendChan <- msg
		case id := <-srv.closeChan:
			conn := srv.getConnById(id)
			conn.closeChan <- 0
		}
	}
}

func (srv *LspServer) getConnById(id uint16) *lspConn {
	for _, v := range srv.connMap {
		if v.connId == id {
			return v
		}
	}
	return nil
}

func (srv *LspServer) loopRead() {
	conn := srv.udpConn
	var buf [2000]byte
	for {
		n, addr, err := conn.ReadFromUDP(buf[0:])
		if err != nil {
			lsplog.Vlogf(3, "ReadFromUDP error: %s\n", err.Error())
			continue
		}
		var msg LspMsg
		err = json.Unmarshal(buf[0:], &msg)
		if err != nil {
			lsplog.Vlogf(3, "Unmarshal error: %s\n", err.Error())
			continue
		}
		packet := &udpPacket{
			msg:  &msg,
			addr: addr,
		}
		select {
		case <-srv.readQuitChan:
			lsplog.Vlogf(1, "server loop read quit\n")
		case srv.netReadChan <- packet:
			lsplog.Vlogf(5, "recieved udp packet\n")
		}
	}
}

func (srv *LspServer) handleUdpPacket(p *udpPacket) {
	msg := p.msg
	addr := p.addr
	switch msg.Type {
	case MsgCONNECT:
		hostport := addr.String()
		conn := srv.connMap[hostport]
		if conn != nil {
			lsplog.Vlogf(5, "Duplicate connect request from: %s\n", hostport)
			conn.sendChan <- conn.lastAck
		}
		id := srv.nextConnId
		srv.nextConnId++
		conn = newLspConn(srv, addr, id)
		conn.lastAck = genAckMsg(id, 0)
		srv.connMap[hostport] = conn
		go conn.serve()
		conn.sendChan <- conn.lastAck
	case MsgACK:
	case MsgDATA:
	}
}

func (srv *LspServer) read() (uint16, []byte, error) {
	return 0, nil, nil
}

func (srv *LspServer) write(id uint16, payload []byte) error {
	msg := genDataMsg(id, 0, payload)
	srv.appWriteChan <- msg
	return nil
}

func (srv *LspServer) closeConn(id uint16) {
	srv.closeChan <- id
}

func epochTrigger(delay int64, notify chan<- int, quit <-chan int) {
	for {
		timeout := time.After(time.Duration(delay) * time.Millisecond)
		select {
		case <-timeout:
			notify <- 0
		case <-quit:
			return
		}
	}
}
