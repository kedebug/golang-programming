package lsp

import (
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
	recvBuf        *bufi.Buf
	sendChan       chan *LspMsg
	recvChan       chan *LspMsg
	closeChan      chan chan error
	nextRecvSeqNum byte
	nextSendSeqNum byte
	lastAck        *LspMsg
	lastTime       time.Duration
}

func newLspConn(srv *LspServer, addr *lspnet.UDPAddr, id uint16) *lspConn {
	conn := &lspConn{
		srv:            srv,
		addr:           addr,
		connId:         id,
		sendBuf:        bufi.NewBuf(),
		recvBuf:        bufi.NewBuf(),
		sendChan:       make(chan *LspMsg),
		recvChan:       make(chan *LspMsg),
		closeChan:      make(chan chan error),
		nextRecvSeqNum: 1,
		nextSendSeqNum: 1,
	}
	conn.lastAck = genAckMsg(id, 0)
	conn.sendChan <- conn.lastAck
	return conn
}

func (conn *lspConn) serve() {
	params := conn.srv.params
	interval := time.Duration(params.EpochMilliseconds) * time.Millisecond
	timeout := time.After(interval)
	for {
		select {
		case msg := <-conn.sendChan:
			conn.send(msg)
		case msg := <-conn.recvChan:
			conn.recieve(msg)
		case err := <-conn.closeChan:
			conn.closeConn(err)
			return
		case <-timeout:
			conn.epochTrigger()
			timeout = time.After(interval)
		}
	}
}

func (conn *lspConn) send(msg *LspMsg) {
	switch msg.Type {
	case MsgACK:
	case MsgDATA:
		conn.sendBuf.Insert(msg)
		b, _ := conn.sendBuf.Front()
		msg = b.(*LspMsg)
		msg.SeqNum = conn.nextSendSeqNum
		conn.nextSendSeqNum++
	}
	conn.srv.writeToUDP(conn, msg)
}

func (conn *lspConn) recieve(msg *LspMsg) {
	switch msg.Type {
	case MsgDATA:
		if msg.SeqNum == conn.nextRecvSeqNum {
			conn.recvBuf.Insert(msg)
			conn.nextRecvSeqNum++
			conn.lastAck = genAckMsg(conn.connId, msg.SeqNum)
			conn.send(conn.lastAck)
		} else {
			lsplog.Vlogf(5, "ignore data, connId=%v, seqnum=%v, expected=%v\n",
				conn.connId, msg.SeqNum, conn.nextRecvSeqNum)
		}
	case MsgACK:
		if conn.sendBuf.Empty() {
			lsplog.Vlogf(5, "ignore ack, nothing to send\n")
		}
		b, _ := conn.sendBuf.Front()
		expect := b.(*LspMsg)
		if msg.SeqNum == expect.SeqNum {
			conn.sendBuf.Remove()
		} else {
			lsplog.Vlogf(5, "ignore ack, ConnId=%v, seqnum=%v, expected=%v\n",
				conn.connId, msg.SeqNum, expect.SeqNum)
		}
	}
}

func (conn *lspConn) epochTrigger() {
	if !conn.sendBuf.Empty() {
		b, _ := conn.sendBuf.Front()
		msg := b.(*LspMsg)
		if msg.SeqNum != 0 {
			// resend message
			conn.srv.writeToUDP(conn, msg)
		}
	}
	if conn.lastAck != nil {
		conn.srv.writeToUDP(conn, conn.lastAck)
	}
}

func (conn *lspConn) closeConn(err chan error) {
}
