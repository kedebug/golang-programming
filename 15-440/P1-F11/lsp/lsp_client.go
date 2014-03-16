package lsp

import (
	"encoding/json"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
)

type client struct {
	udpConn        *lspnet.UDPConn
	addr           *lspnet.UDPAddr
	conn           *lspConn
	netReadChan    chan *LspMsg
	appWriteChan   chan *LspMsg
	appReadChan    chan *LspMsg
	connIdChan     chan uint16
	removeConnChan chan uint16
	closeChan      chan error
}

func newLspClient(hostport string, params *LspParams) (*LspClient, error) {
	if params == nil {
		params = &LspParams{5, 2000}
	}
	addr, err := lspnet.ResolveUDPAddr("udp", hostport)
	if lsplog.CheckReport(1, err) {
		return nil, err
	}
	udpConn, err := lspnet.DialUDP("udp", nil, addr)
	if err != nil {
		lsplog.Vlogf(1, "[client] connect to %v failed: %v\n", addr.String(), err)
		return nil, err
	} else {
		lsplog.Vlogf(1, "[client] connected to %v\n", addr.String())
	}
	removeChan := make(chan uint16)
	appReadChan := make(chan *LspMsg)
	conn := newLspConn(params, udpConn, addr, 0, appReadChan, removeChan)
	cli := &LspClient{
		client{
			udpConn:        udpConn,
			addr:           addr,
			conn:           conn,
			netReadChan:    make(chan *LspMsg),
			appWriteChan:   make(chan *LspMsg),
			appReadChan:    appReadChan,
			connIdChan:     make(chan uint16, 1),
			removeConnChan: removeChan,
			closeChan:      make(chan error),
		},
	}
	go cli.loopServe()
	go cli.loopRead()
	return cli, nil
}

func (cli *LspClient) loopServe() {
	var connid uint16 = 0
	for {
		select {
		case msg := <-cli.netReadChan:
			cli.conn.recvChan <- msg
			if connid == 0 {
				connid = msg.ConnId
				cli.connIdChan <- connid
			}
		case msg := <-cli.appWriteChan:
			cli.conn.sendChan <- msg
		case <-cli.removeConnChan:
			lsplog.Vlogf(1, "[client] exit\n")
			return
		case <-cli.closeChan:
			cli.closeChan <- nil
		}
	}
}

func (cli *LspClient) loopRead() {
	conn := cli.udpConn
	var buf [2000]byte
	for {
		n, _, err := conn.ReadFromUDP(buf[0:])
		if err != nil {
			lsplog.Vlogf(3, "[client] ReadFromUDP error: %s\n", err.Error())
			continue
		}
		var msg LspMsg
		err = json.Unmarshal(buf[0:n], &msg)
		if err != nil {
			lsplog.Vlogf(3, "[client] Unmarshal error: %s\n", err.Error())
			continue
		}
		cli.netReadChan <- &msg
		lsplog.Vlogf(5, "[client] received udp packet\n")
	}
}

func (cli *LspClient) connId() uint16 {
	if cli.conn.connId == 0 {
		id := <-cli.connIdChan
		cli.connIdChan <- id
		return id
	}
	return cli.conn.connId
}

func (cli *LspClient) read() []byte {
	msg := <-cli.appReadChan
	if msg.Type == MsgDATA {
		return msg.Payload
	}
	return nil

}

func (cli *LspClient) write(payload []byte) error {
	msg := genDataMsg(cli.conn.connId, 0, payload)
	cli.appWriteChan <- msg
	return nil
}

func (cli *LspClient) closeConn() {
	cli.closeChan <- nil
}
