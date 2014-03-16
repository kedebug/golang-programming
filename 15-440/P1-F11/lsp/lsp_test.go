// Automated testing of LSP support code

package lsp

import (
	"encoding/json"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
	"math/rand"
	"testing"
	"time"
)

// General parameters
var DefaultVerbosity = 0

// Convert integer to byte array
func i2b(i int) []byte {
	b, _ := json.Marshal(i)
	return b
}

// Convert byte array back to integer
func b2i(b []byte) int {
	var i int
	json.Unmarshal(b, &i)
	return i
}

// Use channel for synchronization
type CommChan chan int

// Test components
type TestSystem struct {
	Server               *LspServer
	Clients              []*LspClient
	RunFlag              bool     // Set to false to get clients and server to stop
	CChan                CommChan // Use to synchronize
	NClients             int
	NMessages            int
	MaxSleepMilliseconds int
	Tester               *testing.T
	Description          string
	DropPercent          int
	Rgen                 rand.Rand
}

func (ts *TestSystem) SetMaxSleepMilliseconds(ms int) {
	ts.MaxSleepMilliseconds = ms
}

func (ts *TestSystem) SetNMessages(n int) {
	ts.NMessages = n
}

func (ts *TestSystem) SetDescription(t string) {
	ts.Description = t
}

func (ts *TestSystem) SetDropPercent(pct int) {
	ts.DropPercent = pct
}

// Delay for fixed amount of time (milliseconds)
func delay(ms int) {
	d := time.Duration(ms) * time.Millisecond
	if d.Nanoseconds() > 0 {
		time.Sleep(d)
	}
}

// Delay for a random amount of time up to limit
func rdelay(maxms int) {
	rms := int(float64(maxms) * rand.Float64())
	delay(rms)
}

func NewTestSystem(t *testing.T, nclients int, params *LspParams) *TestSystem {
	var err error
	t.Logf("Testing: %d clients", nclients)
	ts := new(TestSystem)
	ts.Rgen = *rand.New(rand.NewSource(time.Now().Unix()))

	// Try to create server for different port numbers
	ntries := 5
	port := 0
	var errmsg string = "Failed to create server on ports"
	var i int
	for i = 0; i < ntries && ts.Server == nil; i++ {
		port = 2000 + ts.Rgen.Intn(1000)
		ts.Server, err = NewLspServer(port, params)
		errmsg = fmt.Sprintf("%s %d", errmsg, port)
	}
	if err != nil {
		t.Logf(errmsg)
		return nil
	}
	// Create clients
	ts.Clients = make([]*LspClient, nclients)
	for i = 0; i < nclients; i++ {
		hostport := fmt.Sprintf("localhost:%v", port)
		ts.Clients[i], err = NewLspClient(hostport, params)
		if err != nil {
			t.Logf("Couldn't open client on port %d.  Error message'%s'",
				port, err.Error())
			return nil
		}
	}
	ts.RunFlag = true
	ts.CChan = make(CommChan, nclients+1)
	ts.NClients = nclients
	ts.NMessages = 50
	ts.MaxSleepMilliseconds = 0
	ts.Tester = t
	ts.Description = ""
	return ts
}

// Set up server as an echo.

func (ts *TestSystem) runserver() {
	ndead := 0
	for ts.RunFlag {
		id, data, err := ts.Server.Read()
		if err != nil {
			ndead++
		} else {
			rdelay(ts.MaxSleepMilliseconds)
			ts.Server.Write(id, data)
		}
	}
	ts.Server.CloseAll()
}

// Have client generate n messages and check that they are echoed.
func (ts *TestSystem) runclient(clienti int) {
	if clienti > ts.NClients {
		ts.Tester.Logf("Invalid client number %d\n", clienti)
		ts.Tester.FailNow()
	}
	cli := ts.Clients[clienti]
	for i := 0; i < ts.NMessages && ts.RunFlag; i++ {
		wt := ts.Rgen.Intn(100)
		werr := cli.Write(i2b(i + wt))
		if werr != nil {
			ts.Tester.Logf("Client write got error '%s'\n",
				werr.Error())
			ts.RunFlag = false
			ts.Tester.FailNow()
		}

		b := cli.Read()
		if b == nil {
			ts.Tester.Logf("Client read got error\n")
			ts.RunFlag = false
			ts.Tester.FailNow()
		}
		v := b2i(b)
		if v != wt+i {
			ts.Tester.Logf("Client got %d.  Expected %d\n",
				v, i+wt)
			ts.RunFlag = false
			ts.Tester.FailNow()
		}
	}
	cli.Close()
	lsplog.Vlogf(0, "Client #%d completed %d messages\n", clienti, ts.NMessages)
	ts.CChan <- 0
}

// Time Out test by sending -1 to CChan
func (ts *TestSystem) runtimeout(ms int) {
	// Delay for 1 second
	delay(ms)
	ts.CChan <- -1
}

func (ts *TestSystem) runtest(timeoutms int) {
	lspnet.SetWriteDropPercent(ts.DropPercent)
	if ts.Description != "" {
		fmt.Printf("Testing: %s\n", ts.Description)
	}
	go ts.runserver()
	for i := 0; i < ts.NClients; i++ {
		go ts.runclient(i)
	}
	go ts.runtimeout(timeoutms)
	for i := 0; i < ts.NClients; i++ {
		v := <-ts.CChan
		if v < 0 {
			ts.RunFlag = false
			ts.Tester.Logf("Test timed out after %f secs\n",
				float64(timeoutms)/1000.0)
			ts.Tester.FailNow()
		}
	}
	ts.RunFlag = false
	lsplog.Vlogf(0, "Passed: %d clients, %d messages/client, %.2f maxsleep, %.2f drop rate\n",
		ts.NClients, ts.NMessages,
		float64(ts.MaxSleepMilliseconds)/1000.0,
		float64(ts.DropPercent)/100.0)
	lsplog.SetVerbose(DefaultVerbosity)
	lspnet.SetWriteDropPercent(0)
}

func params(lim, ms int) *LspParams {
	return &LspParams{lim, ms}
}

func TestBasic1(t *testing.T) {
	lsplog.SetVerbose(2)
	ts := NewTestSystem(t, 1, params(5, 2000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("Short client/server interaction")
	ts.SetNMessages(3)
	ts.runtest(1000)
}

func TestBasic2(t *testing.T) {
	lsplog.SetVerbose(2)
	ts := NewTestSystem(t, 1, params(5, 2000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("Long client/server interaction")
	ts.SetNMessages(50)
	ts.runtest(1000)
}

func TestBasic3(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 2, params(5, 2000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("Two client/one server interaction")
	ts.SetNMessages(50)
	ts.runtest(1000)
}

func TestBasic4(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 10, params(5, 2000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("10 clients, long interaction")
	ts.SetNMessages(50)
	ts.runtest(2000)
}

func TestBasic5(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 4, params(5, 2000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("Random delays by clients & server")
	ts.SetNMessages(10)
	ts.SetMaxSleepMilliseconds(100)
	ts.runtest(15000)
}

func TestBasic6(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 2, params(5, 2000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("2 clients, test wraparound of sequence numnbers")
	ts.SetNMessages(550)
	ts.runtest(2000)
}

func TestSendReceive1(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 1, params(5, 5000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("No epochs.  Single client")
	ts.SetNMessages(6)
	ts.runtest(5000)
}

func TestSendReceive2(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 4, params(5, 5000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("No epochs.  Multiple clients")
	ts.SetNMessages(6)
	ts.runtest(5000)
}

func TestSendReceive3(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 4, params(5, 10000))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("No epochs.  Random delays inserted")
	ts.SetNMessages(6)
	ts.SetMaxSleepMilliseconds(100)
	ts.runtest(10000)
}

func TestRobust1(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 1, params(20, 50))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("Single client.  Some packet dropping")
	ts.SetDropPercent(20)
	ts.SetNMessages(10)
	ts.runtest(10000)
}

func TestRobust2(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 3, params(20, 50))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("Three clients.  Some packet dropping")
	ts.SetDropPercent(20)
	ts.SetNMessages(10)
	ts.runtest(15000)
}

func TestRobust3(t *testing.T) {
	lsplog.SetVerbose(DefaultVerbosity)
	ts := NewTestSystem(t, 4, params(20, 50))
	if ts == nil {
		t.FailNow()
	}
	ts.SetDescription("Four clients.  Some packet dropping")
	ts.SetDropPercent(20)
	ts.SetNMessages(8)
	ts.runtest(15000)
}
