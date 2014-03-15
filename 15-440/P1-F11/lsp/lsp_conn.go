package lsp

import (
	"encoding/json"
	"github.com/kedebug/golang-programming/15-440/P1-F11/bufi"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
	"time"
)

const (
	ClientSide = iota
	ServerSide
)

type lspConn struct {
	params         *LspParams
	udpConn        *lspnet.UDPConn
	addr           *lspnet.UDPAddr
	connId         uint16
	removeChan     chan<- uint16 // chan from server
	sendBuf        *bufi.Buf
	recvBuf        *bufi.Buf
	sendChan       chan *LspMsg
	recvChan       chan *LspMsg
	readChan       chan<- *LspMsg
	closeChan      chan error
	writeDone      chan error
	closed         bool
	nextRecvSeqNum byte
	nextSendSeqNum byte
	lastAck        *LspMsg
	lastTime       time.Time
	whichSide      byte
}

func newLspConn(params *LspParams, udpConn *lspnet.UDPConn, addr *lspnet.UDPAddr,
	id uint16, readChan chan<- *LspMsg, removeChan chan<- uint16) *lspConn {
	conn := &lspConn{
		params:         params,
		udpConn:        udpConn,
		addr:           addr,
		connId:         id,
		removeChan:     removeChan,
		sendBuf:        bufi.NewBuf(),
		recvBuf:        bufi.NewBuf(),
		sendChan:       make(chan *LspMsg, 1),
		recvChan:       make(chan *LspMsg, 1),
		readChan:       readChan,
		closeChan:      make(chan error),
		writeDone:      make(chan error),
		nextRecvSeqNum: 1,
		nextSendSeqNum: 1,
		lastTime:       time.Now(),
		lastAck:        genAckMsg(id, 0),
	}
	if id == 0 {
		conn.sendChan <- genConnMsg(0)
		conn.whichSide = ClientSide
	} else {
		conn.sendChan <- conn.lastAck
		conn.whichSide = ServerSide
	}
	go conn.serve()
	return conn
}

func (conn *lspConn) serve() {
	params := conn.params
	interval := time.Duration(params.EpochMilliseconds) * time.Millisecond
	timeout := time.After(interval)
	for {
		var first *LspMsg
		var readChan chan<- *LspMsg
		if !conn.recvBuf.Empty() {
			b, _ := conn.recvBuf.Front()
			first = b.(*LspMsg)
			readChan = conn.readChan
		}
		select {
		case msg := <-conn.sendChan:
			if !conn.closed {
				conn.send(msg)
			} else {
				lsplog.Vlogf(2, "connection already closed, ignore send msg")
			}
		case msg := <-conn.recvChan:
			conn.lastTime = time.Now()
			conn.receive(msg)
		case readChan <- first:
			conn.recvBuf.Remove()
		case <-conn.closeChan:
			conn.closed = true
		case <-timeout:
			if conn.epochTrigger() {
				timeout = time.After(interval)
			} else {
				return
			}
		case <-conn.writeDone:
			conn.removeChan <- conn.connId
			return
		}
	}
}

func (conn *lspConn) send(msg *LspMsg) {
	lsplog.Vlogf(3, "[conn] send data, connId=%v, seqnum=%v, payload=%s\n",
		conn.connId, msg.SeqNum, msg.Payload)
	switch msg.Type {
	case MsgCONNECT:
		conn.sendBuf.Insert(msg)
	case MsgACK:
	case MsgDATA:
		msg.SeqNum = conn.nextSendSeqNum
		conn.nextSendSeqNum++
		conn.sendBuf.Insert(msg)
	}
	conn.udpWrite(msg)
}

func (conn *lspConn) receive(msg *LspMsg) {
	lsplog.Vlogf(3, "[conn] recv data, connId=%v, seqnum=%v, payload=%s\n",
		conn.connId, msg.SeqNum, msg.Payload)
	switch msg.Type {
	case MsgDATA:
		if msg.SeqNum == conn.nextRecvSeqNum {
			conn.recvBuf.Insert(msg)
			conn.nextRecvSeqNum++
			conn.lastAck = genAckMsg(conn.connId, msg.SeqNum)
			conn.send(conn.lastAck)
		} else {
			lsplog.Vlogf(5, "[conn] ignore data, connId=%v, seqnum=%v, expected=%v\n",
				conn.connId, msg.SeqNum, conn.nextRecvSeqNum)
		}
	case MsgACK:
		if conn.sendBuf.Empty() {
			lsplog.Vlogf(5, "[conn] ignore ack, nothing to send\n")
			return
		}
		b, _ := conn.sendBuf.Front()
		expect := b.(*LspMsg)
		if msg.SeqNum == expect.SeqNum {
			conn.sendBuf.Remove()
			if msg.SeqNum == 0 {
				conn.connId = msg.ConnId
				lsplog.Vlogf(2, "[conn] connection confirmed, ConnId=%v\n", msg.ConnId)
			}
		} else {
			lsplog.Vlogf(5, "[conn] ignore ack, ConnId=%v, seqnum=%v, expected=%v\n",
				conn.connId, msg.SeqNum, expect.SeqNum)
		}
		if conn.sendBuf.Empty() && conn.closed {
			conn.writeDone <- nil
		}
	}
}

func (conn *lspConn) epochTrigger() bool {
	if !conn.sendBuf.Empty() {
		b, _ := conn.sendBuf.Front()
		msg := b.(*LspMsg)
		if msg.SeqNum != 0 {
			// rewrite message
			conn.udpWrite(msg)
		}
	}
	if conn.lastAck != nil {
		conn.udpWrite(conn.lastAck)
	}
	params := conn.params
	delay := time.Now().Sub(conn.lastTime)
	allowed := time.Duration(params.EpochMilliseconds*params.EpochLimit) * time.Millisecond
	if delay >= allowed {
		// remove connection
		conn.removeChan <- conn.connId
		return false
	}
	return true
}

func (conn *lspConn) udpWrite(msg *LspMsg) {
	result, err := json.Marshal(msg)
	if err != nil {
		lsplog.Vlogf(3, "[conn] Marshal failed: %s\n", err.Error())
		return
	}
	switch conn.whichSide {
	case ClientSide:
		_, err = conn.udpConn.Write(result)
	case ServerSide:
		_, err = conn.udpConn.WriteToUDP(result, conn.addr)
	}
	if err != nil {
		lsplog.Vlogf(3, "[conn] udpWrite failed: %s\n", err.Error())
	}
}
