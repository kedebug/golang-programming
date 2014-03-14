package lsp

import (
	"encoding/json"
	"github.com/kedebug/golang-programming/15-440/P1-F11/bufi"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
	"time"
)

type lspConn struct {
	srv            *LspServer
	addr           *lspnet.UDPAddr
	connId         uint16
	sendBuf        *bufi.Buf
	sendChan       chan *LspMsg
	recvChan       chan *LspMsg
	closeChan      chan int
	nextRecvSeqNum byte
	nextSendSeqNum byte
	lastAck        *LspMsg
	lastTime       time.Duration
}

func newLspConn(srv *LspServer, addr *lspnet.UDPAddr, id uint16) *lspConn {
	return &lspConn{
		srv:       srv,
		addr:      addr,
		connId:    id,
		sendBuf:   bufi.NewBuf(),
		sendChan:  make(chan *LspMsg),
		recvChan:  make(chan *LspMsg),
		closeChan: make(chan int),
	}
}

func (conn *lspConn) serve() {
	for {
		select {
		case msg := <-conn.sendChan:
			conn.write(msg)
		case msg := <-conn.recvChan:
		}
	}
}

func (conn *lspConn) write(msg *LspMsg) {
	if msg.Type == MsgDATA {
		msg.SeqNum = conn.nextSendSeqNum
		conn.nextSendSeqNum++
	}
	result, err := json.Marshal(msg)
	if err != nil {
		lsplog.Vlogf(3, "Marshal failed: %s\n", err.Error())
		return
	}
	_, err = conn.srv.udpConn.WriteToUDP(result, conn.addr)
	if err != nil {
		lsplog.Vlogf(3, "WriteToUDP failed: %s\n", err.Error())
	}
}
