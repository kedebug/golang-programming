// Implementation of LSP protocal

// NOTE TO STUDENTS
// This file contains just a skeleton of the code you will need to implement
// the live sequence protocol.
// Comments starting with NOTE are intended to provide helpful advice

package lsp

import (
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
)

type LspParams struct {
	EpochLimit        int
	EpochMilliseconds int
}

// API definition

const (
	MsgCONNECT = iota
	MsgDATA
	MsgACK
	MsgINVALID
)

// Network communications
type LspMsg struct {
	Type    byte
	ConnId  uint16 // Connection ID
	SeqNum  byte   // Sequence number
	Payload []byte // Message payload
}

// Client API
// Client information
type LspClient struct {
	client
}

// Client operations

func NewLspClient(hostport string, params *LspParams) (*LspClient, error) {
	cli, err := newLspClient(hostport, params)
	if err != nil {
		lsplog.Vlogf(1, "[client] create failure: %v", err)
		return nil, err
	}
	return cli, nil
}

func (cli *LspClient) ConnId() uint16 {
	return cli.connId()
}

// Client Read.  Returns nil when connection lost
func (cli *LspClient) Read() []byte {
	return cli.read()
}

// Client Write.  Should not send nil
func (cli *LspClient) Write(payload []byte) error {
	return cli.write(payload)
}

// Close connection.
func (cli *LspClient) Close() {
	cli.closeConn()
}

// Server API
type LspServer struct {
	server
}

// Server operations

func NewLspServer(port int, params *LspParams) (*LspServer, error) {
	return newLspServer(port, params)
}

// Server Read.  Returns nil when connection lost
func (srv *LspServer) Read() (uint16, []byte, error) {
	return srv.read()
}

// Server Write.  Should not send nil
func (srv *LspServer) Write(id uint16, payload []byte) error {
	return srv.write(id, payload)
}

// Close connection.
func (srv *LspServer) Close(id uint16) {
	srv.closeConn(id)
}

func (srv *LspServer) CloseAll() {
	srv.closeAll()
}
