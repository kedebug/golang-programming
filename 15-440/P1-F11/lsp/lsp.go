// Implementation of LSP protocal

// NOTE TO STUDENTS
// This file contains just a skeleton of the code you will need to implement
// the live sequence protocol.
// Comments starting with NOTE are intended to provide helpful advice

package lsp

type LspParams struct {
	EpochLimit        int
	EpochMilliseconds int64
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
	return nil, nil
}

func (cli *LspClient) ConnId() uint16 {
	return 0
}

// Client Read.  Returns nil when connection lost
func (cli *LspClient) Read() []byte {
	return nil
}

// Client Write.  Should not send nil
func (cli *LspClient) Write(payload []byte) {
}

// Close connection.
func (cli *LspClient) Close() {
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
	return 0, nil, nil
}

// Server Write.  Should not send nil
func (srv *LspServer) Write(id uint16, payload []byte) {
}

// Close connection.
func (srv *LspServer) Close(id int) {
}

func (srv *LspServer) CloseAll() {
}
