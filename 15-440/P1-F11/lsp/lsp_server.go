package lsp

import (
	"encoding/json"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
)

type server struct {
	nextConnId     uint16
	connMap        map[string]*lspConn
	params         *LspParams
	udpConn        *lspnet.UDPConn
	udpAddr        *lspnet.UDPAddr
	netReadChan    chan *udpPacket
	appWriteChan   chan *LspMsg
	closeChan      chan uint16
	closeAllChan   chan error
	stop           bool
	removeConnChan chan uint16
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
			nextConnId:     1,
			params:         params,
			connMap:        make(map[string]*lspConn),
			udpConn:        udpconn,
			udpAddr:        addr,
			netReadChan:    make(chan *udpPacket),
			appWriteChan:   make(chan *LspMsg),
			closeChan:      make(chan uint16),
			closeAllChan:   make(chan error),
			removeConnChan: make(chan uint16),
		},
	}
	go srv.loopServe()
	go srv.loopRead()
	return srv, nil
}

func (srv *LspServer) loopServe() {
	for {
		select {
		case p := <-srv.netReadChan:
			srv.handleUdpPacket(p)
		case msg := <-srv.appWriteChan:
			if conn := srv.getConnById(msg.ConnId); conn != nil {
				conn.sendChan <- msg
			}
		case id := <-srv.closeChan:
			if conn := srv.getConnById(id); conn != nil {
				conn.closeChan <- nil
			}
		case <-srv.closeAllChan:
			srv.stop = true
			for _, v := range srv.connMap {
				v.closeChan <- nil
			}
		case id := <-srv.removeConnChan:
			if conn := srv.getConnById(id); conn != nil {
				delete(srv.connMap, conn.addr.String())
				lsplog.Vlogf(2, "remove connection: %v\n", conn.addr.String())
			}
			if srv.stop && len(srv.connMap) == 0 {
				lsplog.Vlogf(1, "serve stop running\n")
				return
			}
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
		err = json.Unmarshal(buf[0:n], &msg)
		if err != nil {
			lsplog.Vlogf(3, "Unmarshal error: %s\n", err.Error())
			continue
		}
		packet := &udpPacket{
			msg:  &msg,
			addr: addr,
		}
		srv.netReadChan <- packet
		lsplog.Vlogf(5, "recieved udp packet\n")
	}
}

func (srv *LspServer) handleUdpPacket(p *udpPacket) {
	msg := p.msg
	addr := p.addr
	hostport := addr.String()
	conn := srv.connMap[hostport]
	switch msg.Type {
	case MsgCONNECT:
		if conn != nil {
			lsplog.Vlogf(5, "Duplicate connect request from: %s\n", hostport)
			conn.sendChan <- conn.lastAck
		}
		conn = newLspConn(srv.params, srv.udpConn, addr, srv.nextConnId, srv.removeConnChan)
		srv.nextConnId++
		srv.connMap[hostport] = conn
		conn.sendChan <- conn.lastAck
		go conn.serve()
	case MsgDATA:
		conn.recvChan <- msg
	case MsgACK:
		conn.recvChan <- msg
	default:
		lsplog.Vlogf(5, "Invalid packet, hostport=%v\n", hostport)
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

func (srv *LspServer) closeAll() {
	srv.closeAllChan <- nil
}
