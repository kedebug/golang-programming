// Automated testing of LSP support code.
// These test provide some corner conditions to check
// properly handing of disconnecting components & early buffering

package lsp

import (
	"encoding/json"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"math/rand"
	"testing"
	"time"
)

// General parameters
var AuxDefaultVerbosity = 0

// Convert integer to byte array
func auxi2b(i int) []byte {
	b, _ := json.Marshal(i)
	return b
}

// Convert byte array back to integer
func auxb2i(b []byte) int {
	var i int
	json.Unmarshal(b, &i)
	return i
}

// Different test options
const (
	doslowstart = iota
	doclientstop
	doserverstopconns
	doserverstopall
)

var ModeName = map[int]string{
	doslowstart:       "Slow Start",
	doclientstop:      "Client Stop",
	doserverstopconns: "Server Stop Connections",
	doserverstopall:   "Server Stop All",
}

// Test components
type AuxTestSystem struct {
	Server  *LspServer
	Clients []*LspClient
	Params  *LspParams
	Mode    int // One of the modes enumerated above
	Port    int
	// Use to synchronize
	ClientChan chan bool // Clients
	ServerChan chan bool // Server
	TimeChan   chan bool // Overall time limit
	RunFlag    bool
	Nclients   int
	Nmessages  int
	Tester     *testing.T
	Rgen       rand.Rand
}

// Delay for fixed amount of time, specified in terms of epochs
func (ts *AuxTestSystem) auxdelay(epochfraction float64) {
	ms := int(epochfraction * float64(ts.Params.EpochMilliseconds))
	d := time.Duration(ms) * time.Millisecond
	if d.Nanoseconds() > 0 {
		time.Sleep(d)
	}
}

// Generate random port number
func randport(rgen rand.Rand) int {
	return 3000 + rgen.Intn(1000)
}

// Try to create server on designated port.  Return false if fail
func (ts *AuxTestSystem) GenerateServer() bool {
	var err error
	ts.Server, err = NewLspServer(ts.Port, ts.Params)
	return (err == nil)
}

// Try to create next client.  Return false if fail
func (ts *AuxTestSystem) GenerateClient(index int) bool {
	var err error
	hostport := fmt.Sprintf("localhost:%d", ts.Port)
	ts.Clients[index], err = NewLspClient(hostport, ts.Params)
	return (err == nil)
}

func NewAuxTestSystem(t *testing.T, nclients int, mode int, maxepochs int,
	params *LspParams) {
	fmt.Printf("Testing: Mode %s, %d clients\n", ModeName[mode], nclients)
	ts := new(AuxTestSystem)
	ts.Mode = mode
	ts.Params = params
	ts.ClientChan = make(chan bool)
	ts.ServerChan = make(chan bool)
	ts.TimeChan = make(chan bool)
	ts.Clients = make([]*LspClient, nclients)
	ts.Nclients = nclients
	ts.Nmessages = 10
	ts.Rgen = *rand.New(rand.NewSource(time.Now().Unix()))
	ts.Port = randport(ts.Rgen)
	ts.RunFlag = true
	ts.Tester = t
	go ts.BuildServer()
	for i := 0; i < nclients; i++ {
		go ts.BuildClient(i)
	}
	go ts.runtimeout(float64(maxepochs))

	switch ts.Mode {
	case doclientstop:
		// Wait for server or timer to complete
		select {
		case sok := <-ts.ServerChan:
			if !sok {
				ts.Tester.Logf("Server error\n")
				ts.Tester.FailNow()
			}
		case <-ts.TimeChan:
			ts.Tester.Logf("Test timed out waiting for server\n")
			ts.Tester.FailNow()
		}
		lsplog.Vlogf(0, "Server completed\n")
		// Wait for the clients
		for i := 0; i < nclients; i++ {
			select {
			case cok := <-ts.ClientChan:
				if !cok {
					ts.Tester.Logf("Client error\n")
					ts.Tester.FailNow()
				}
			case <-ts.TimeChan:
				ts.Tester.Logf("Test timed out waiting for client\n")
				ts.Tester.FailNow()
			}
		}
		lsplog.Vlogf(0, "Clients completed\n")
	default: // Includes stopserver
		// Wait for the clients
		for i := 0; i < nclients; i++ {
			select {
			case cok := <-ts.ClientChan:
				if !cok {
					ts.Tester.Logf("Client error\n")
					ts.Tester.FailNow()
				}
			case <-ts.TimeChan:
				ts.Tester.Logf("Test timed out waiting for client\n")
				ts.Tester.FailNow()
			}
		}
		// Wait for server or timer to complete
		select {
		case sok := <-ts.ServerChan:
			if !sok {
				ts.Tester.Logf("Server error\n")
				ts.Tester.FailNow()
			}
		case <-ts.TimeChan:
			ts.Tester.Logf("Test timed out waiting for server\n")
			ts.Tester.FailNow()
		}
	}
	lsplog.SetVerbose(AuxDefaultVerbosity)
}

// Create server and and run test

func (ts *AuxTestSystem) BuildServer() {
	if ts.Mode == doslowstart {
		// Delay start so that clients can start first
		sdelay := 3.0
		lsplog.Vlogf(6, "Delaying start by %.2f epochs\n", sdelay)
		ts.auxdelay(sdelay)
	}
	if !ts.GenerateServer() {
		lsplog.Vlogf(0, "Couldn't create server on port %d\n", ts.Port)
		ts.ServerChan <- false
		return
	}
	ndead := 0
	necho := 0
	for ts.RunFlag {
		id, data, err := ts.Server.Read()
		if err != nil {
			lsplog.Vlogf(0, "Detected dead connection %d\n", id)
			ndead++
			if ts.Mode == doclientstop && ndead == ts.Nclients {
				lsplog.Vlogf(0, "Detected termination by all %d clients\n", ndead)
				ts.ServerChan <- true
				// Shut down server after completion
				ts.Server.CloseAll()
				return
			}
		} else {
			err = ts.Server.Write(id, data)
			if err != nil {
				lsplog.Vlogf(0, "Server failed to write to connection %d\n", id)
				ts.ServerChan <- false
				return
			}
			necho++
			if necho == ts.Nclients*ts.Nmessages {
				lsplog.Vlogf(0, "Echoed all %d X %d messsages\n",
					ts.Nclients, ts.Nmessages)
				if ts.Mode == doserverstopall {
					// Tell server to stop
					lsplog.Vlogf(0, "Stopping all connections from server\n")
					ts.Server.CloseAll()
					ts.ServerChan <- true
					return
				}
				if ts.Mode == doserverstopconns {
					for _, cli := range ts.Clients {
						id := cli.ConnId()
						ts.Server.Close(id)
					}
					ts.ServerChan <- true
					return
				}
				if ts.Mode != doclientstop {
					ts.ServerChan <- true
					// Shut down server after completion
					ts.Server.Close(id)
					return
				}
			}
		}
	}
	// Reached time limit
	if ts.Mode == doclientstop {
		lsplog.Vlogf(0, "Server only detected termination of %d clients\n", ndead)
		ts.ServerChan <- false
	} else {
		lsplog.Vlogf(0, "Server only received %d messages.  Expected %d X %d\n",
			necho, ts.Nclients, ts.Nmessages)
		ts.ServerChan <- false
	}
	return
}

// Create client.  Have it generate messages and check that they are echoed.
func (ts *AuxTestSystem) BuildClient(clienti int) {
	if !ts.GenerateClient(clienti) {
		lsplog.Vlogf(0, "Couldn't create client %d on port %d\n", clienti, ts.Port)
		ts.ClientChan <- false
		return
	}
	cli := ts.Clients[clienti]
	var mcount int
	for mcount = 0; ts.RunFlag && mcount < ts.Nmessages; mcount++ {
		tv := mcount*100 + ts.Rgen.Intn(100)
		err := cli.Write(auxi2b(tv))
		if err != nil {
			lsplog.Vlogf(0, "Failed write by client %d\n", clienti)
			ts.ClientChan <- false
		}
		b := cli.Read()
		if b == nil {
			lsplog.Vlogf(0, "Client %d detected server termination after only %d messages\n",
				clienti, mcount)
			ts.ClientChan <- false
			// Shut down client after completion
			cli.Close()
			return
		}
		v := auxb2i(b)
		if v != tv {
			lsplog.Vlogf(0, "Client %d got %d.  Expected %d\n",
				clienti, v, tv)
			ts.ClientChan <- false
			// Shut down client after completion
			cli.Close()
		}
	}
	lsplog.Vlogf(0, "Client #%d completed %d messages\n", clienti, ts.Nmessages)
	if ts.Mode == doclientstop {
		// Shut down client
		lsplog.Vlogf(0, "Stopping client\n")
		cli.Close()
		ts.ClientChan <- true
		return
	}
	if ts.Mode == doserverstopconns || ts.Mode == doserverstopall {
		lsplog.Vlogf(0, "Client %d attempting read\n", clienti)
		b := cli.Read()
		if b == nil {
			lsplog.Vlogf(0, "Client %d detected server termination after %d messages\n",
				clienti, mcount)
			ts.ClientChan <- true
			// Shut down client after completion
			cli.Close()
			return
		} else {
			lsplog.Vlogf(0, "Client %d received unexpected data from serve\n",
				clienti)
			ts.ClientChan <- false
			// Shut down client after completion
			cli.Close()
			return
		}
	}
	cli.Close()
	ts.ClientChan <- true
}

func auxparams(lim, ms int) *LspParams {
	return &LspParams{lim, ms}
}

// Time Out test
func (ts *AuxTestSystem) runtimeout(epochfraction float64) {
	lsplog.Vlogf(6, "Setting test to timeout after %.2f epochs\n", epochfraction)
	ts.auxdelay(epochfraction)
	ts.RunFlag = false
	ts.TimeChan <- true
}

func TestSlowStart1(t *testing.T) {
	lsplog.SetVerbose(1)
	NewAuxTestSystem(t, 1, doslowstart, 5, auxparams(5, 500))
}

func TestSlowStart2(t *testing.T) {
	lsplog.SetVerbose(1)
	NewAuxTestSystem(t, 3, doslowstart, 5, auxparams(5, 500))
}

func TestStopServerAll1(t *testing.T) {
	lsplog.SetVerbose(1)
	NewAuxTestSystem(t, 1, doserverstopall, 10, auxparams(5, 500))
}

func TestStopServerAll2(t *testing.T) {
	lsplog.SetVerbose(1)
	NewAuxTestSystem(t, 3, doserverstopall, 5, auxparams(2, 500))
}

func TestStopServerConns1(t *testing.T) {
	lsplog.SetVerbose(1)
	NewAuxTestSystem(t, 1, doserverstopconns, 10, auxparams(5, 500))
}

func TestStopServerConns2(t *testing.T) {
	lsplog.SetVerbose(1)
	NewAuxTestSystem(t, 3, doserverstopconns, 5, auxparams(2, 500))
}

func TestStopClient1(t *testing.T) {
	lsplog.SetVerbose(1)
	NewAuxTestSystem(t, 2, doclientstop, 10, auxparams(5, 500))
}

func TestStopClient2(t *testing.T) {
	lsplog.SetVerbose(1)
	NewAuxTestSystem(t, 3, doclientstop, 15, auxparams(5, 500))
}
