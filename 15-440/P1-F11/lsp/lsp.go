// Implementation of LSP protocal

// NOTE TO STUDENTS
// This file contains just a skeleton of the code you will need to implement
// the live sequence protocol.
// Comments starting with NOTE are intended to provide helpful advice

package lsp

import (
	"log"
	"os"
	// NOTE: You will want to import more packages here
)

// Global Parameters.  These parameters control the operation of both
// the server and the clients.

// Length of epoch in seconds
var epochlength float32 = 2.0

// Set length of epoch (in seconds)
func SetEpochLength(v float32) {
	epochlength = v
}

// Number of epochs before giving up on connection
var epochlimit int = 5

func SetEpochCount(n int) {
	epochlimit = n
}

// Fraction of packets that should be dropped as a robustness test.
// The same values apply for sending & receiving by both clients and server
var droprate float32 = 0.0

func SetDropRate(rate float32) {
	if rate < 0.0 || rate > 1.0 {
		Vlogf(1, "Invalid drop rate %.2f\n", rate)
		return
	}
	droprate = rate
}

// API definition

// Network communications
type Packet struct {
	Connid  int    // Connection ID
	Seqnum  int    // Sequence number
	Payload []byte // Message payload
}

// Client API
// Client information
type LspClient struct {
	// NOTE: You will need to define the fields of this structure
}

// Client operations

// Client Read.  Returns nil when connection lost
func (cli *LspClient) Read() []byte {
	// NOTE: You must implement this function
}

// Client Write.  Should not send nil
func (cli *LspClient) Write(payload []byte) {
	// NOTE: You must implement this function
}

// Close connection.
func (cli *LspClient) Close() {
	// NOTE: You must implement this function
}

// Server API
type LspServer struct {
	// NOTE: You must define the fields of this structure
}

// Server operations

// Server Read.  Returns nil when connection lost
func (srv *LspServer) Read() (int, []byte) {
	// NOTE: You must define this function
}

// Server Write.  Should not send nil
func (srv *LspServer) Write(id int, payload []byte) {
	// NOTE: You must define this function
}

// Close connection.
func (srv *LspServer) Close(id int) {
	// NOTE: You must define this function
}

// Debugging support
//NOTE: You might the following code useful

// NOTE: You might find it useful to include logging statements
// that are activated according to different levels of verbosity.
// That way, you can set the verbosity level to 0 to eliminate any
// logging, but set to higher levels for debugging.

var verbosity int = 0

func SetVerbose(level int) {
	verbosity = level
}

// NOTE: Here are the levels of verbosity we built into our LSP implementation
// General Rules for Verbosity
// 0: Silent
// 1: New connections by clients and servers
// 2: Application-level jobs
// 3: Low-level tasks
// 4: Data communications
// 5: All communications
// 6: Everything

// Log result if verbosity level high enough
func Vlogf(level int, format string, v ...interface{}) {
	if level <= verbosity {
		log.Printf(format, v...)
	}
}

// Handle errors
func checkreport(level int, err os.Error) bool {
	if err == nil {
		return false
	}
	Vlogf(level, "Error: %s", err.String())
	return true
}

func checkfatal(err os.Error) {
	if err == nil {
		return
	}
	log.Fatalf("Fatal: %s", err.String())
}
