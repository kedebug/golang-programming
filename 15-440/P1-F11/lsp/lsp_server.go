package lsp

import (
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
	"time"
)

type server struct {
	nextConnId  uint16
	connMap     map[uint16]*lspConn
	params      *LspParams
	udpConn     *lspnet.UDPConn
	netReadChan chan *udpPacket
	appReadChan chan *LspMsg
}

type lspConn struct {
	nextRecvSeqNum int
	nextSendSeqNum int
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
			connMap:    make(map[uint16]*lspConn),
			udpConn:    udpconn,
		},
	}
	go srv.loopServe()
	go srv.loopRead()
	return srv, nil
}

func (srv *LspServer) loopServe() {
}

func (srv *LspServer) loopRead() {

}

func (srv *LspServer) read() (uint16, []byte, error) {
	return 0, nil, nil
}

func (srv *LspServer) write() {
}

func (srv *LspServer) closeHandler() {
}

func epochTrigger(delay time.Duration, notify chan<- int, stop <-chan int) {
	for {
		timeout := time.After(delay * time.Millisecond)
		select {
		case <-timeout:
			notify <- 0
		case <-stop:
			return
		}
	}
}
