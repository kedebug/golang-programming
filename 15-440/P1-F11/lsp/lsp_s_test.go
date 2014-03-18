// Automated testing of LSP support code.  These tests use
// synchronization between the clients, the servers, and the
// network to test the handling of buffering, and the close operations

package lsp

import (
	"encoding/json"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// General parameters
var SynchDefaultVerbosity = 0

// Convert integer to byte array
func synchi2b(i int) []byte {
	b, _ := json.Marshal(i)
	return b
}

// Convert byte array back to integer
func synchb2i(b []byte) int {
	var i int
	json.Unmarshal(b, &i)
	return i
}

// Different test options
const (
	doroundtrip = iota
	doserverfastclose
	doservertoclient
	doclienttoserver
)

var SynchModeName = map[int]string{
	doroundtrip:       "Buffering in client and server",
	doserverfastclose: "Fast close of server",
	doservertoclient:  "Stream from server to client",
	doclienttoserver:  "Stream from client to server",
}

// Generate array of random data for transmission
func genData(n int, rgen rand.Rand) []int {
	result := make([]int, n)
	for i := 0; i < n; i++ {
		result[i] = rgen.Int()
	}
	return result
}

// Test components
type SynchTestSystem struct {
	Server    *LspServer
	Clients   []*LspClient
	ClientMap map[uint16]int // From connection id to client number
	MapLock   sync.Mutex     // To protect the client map
	Params    *LspParams
	Mode      int // One of the modes enumerated above
	Port      int
	// Use to synchronize
	C2MChan   chan bool // For client to signal when its done
	S2MChan   chan bool // For server to signal when its done
	N2MChan   chan bool // For network to signal when its done
	TChan     chan int  // For signaling timeouts
	M2CChan   chan bool
	M2SChan   chan bool
	M2NChan   chan bool
	Data      [][]int
	Nclients  int
	Nmessages int
	Rgen      rand.Rand
	Tester    *testing.T
	RunFlag   bool
}

// Test components
func NewSynchTestSystem(t *testing.T, nclients int, nmessages int, mode int, maxepochs int, params *LspParams) {
	fmt.Printf("Testing: Mode %s, %d clients\n", SynchModeName[mode], nclients)
	ts := new(SynchTestSystem)
	ts.Mode = mode
	ts.Params = params
	ts.C2MChan = make(chan bool)
	ts.S2MChan = make(chan bool)
	ts.N2MChan = make(chan bool)
	ts.TChan = make(chan int)
	ts.M2CChan = make(chan bool)
	ts.M2SChan = make(chan bool)
	ts.M2NChan = make(chan bool)
	ts.Nclients = nclients
	ts.Nmessages = nmessages
	ts.Clients = make([]*LspClient, nclients)
	ts.ClientMap = make(map[uint16]int, nclients)
	ts.Data = make([][]int, nclients)
	ts.Rgen = *rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < nclients; i++ {
		ts.Data[i] = genData(nmessages, ts.Rgen)
	}
	ts.Nclients = nclients
	ts.Port = synchrandport(ts.Rgen)
	ts.RunFlag = true
	ts.Tester = t
	go ts.RunNetwork()
	go ts.RunServer()
	// Set up clients
	for i := 0; i < nclients; i++ {
		go ts.RunClient(i)
	}
	go ts.RunTimeout(float64(maxepochs))
	ts.Master()
	ts.RunFlag = false
	lsplog.SetVerbose(SynchDefaultVerbosity)
	lspnet.SetWriteDropPercent(0)
}

// Create server and and run test
func (ts *SynchTestSystem) RunServer() {
	var err error
	ts.Server, err = NewLspServer(ts.Port, ts.Params)
	if err != nil {
		ts.Tester.Logf("Couldn't create server on port %d\n", ts.Port)
		ts.Tester.FailNow()
	}
	lsplog.Vlogf(1, "Server created on port %d\n", ts.Port)
	ts.S2MChan <- true
	// Read
	if ts.Mode != doservertoclient {
		lsplog.Vlogf(4, "Server waiting to read\n")
		<-ts.M2SChan
		lsplog.Vlogf(3, "Server reading messages\n")
		// Receive messages from client
		// Track number received from each client
		rcvdCount := make([]int, ts.Nclients)
		n := ts.Nmessages * ts.Nclients
		for m := 0; m < n; m++ {
			id, b, err := ts.Server.Read()
			if err != nil {
				ts.Tester.Log("Server failed to read\n")
				ts.Tester.Fail()
			}
			v := synchb2i(b)
			ts.MapLock.Lock()
			clienti, found := ts.ClientMap[id]
			ts.MapLock.Unlock()
			if !found || clienti >= ts.Nclients {
				ts.Tester.Logf("Server received message from unknown client #%d\n", id)
				ts.Tester.FailNow()
			}
			rc := rcvdCount[clienti]
			if rc >= ts.Nmessages {
				ts.Tester.Logf("Server has received too many messages from client #%d\n", id)
				ts.Tester.FailNow()
			}
			ev := ts.Data[clienti][rc]
			if v != ev {
				ts.Tester.Logf("Server received element #%v, value %v from connection %v.  Expected %v\n",
					rc, v, id, ev)
				ts.Tester.Fail()
			}
			lsplog.Vlogf(6, "Server received element #%v, value %v from connection %v.  Expected %v\n",
				rc, v, id, ev)
			rcvdCount[clienti] = rc + 1
		}
		lsplog.Vlogf(3, "Server read all %v messages from clients\n", n)
		ts.S2MChan <- true
	}
	// Write
	if ts.Mode != doclienttoserver {
		lsplog.Vlogf(4, "Server waiting to write\n")
		<-ts.M2SChan
		lsplog.Vlogf(3, "Server writing messages\n")
		// Track number sent to each client
		sentCount := make([]int, ts.Nclients)
		n := ts.Nmessages * ts.Nclients
		for nsent := 0; nsent < n; {
			var clienti int
			// Choose random client
			found := false
			for !found {
				clienti = ts.Rgen.Intn(ts.Nclients)
				found = sentCount[clienti] < ts.Nmessages
			}
			id := ts.Clients[clienti].ConnId()
			sc := sentCount[clienti]
			v := ts.Data[clienti][sc]
			err := ts.Server.Write(id, synchi2b(v))
			if err != nil {
				ts.Tester.Logf("Server could not send value %v  (#%d) to client %d (ID %v)\n",
					v, sc, clienti, id)
				ts.Tester.Fail()
			}
			sentCount[clienti] = sc + 1
			nsent++
		}
		lsplog.Vlogf(3, "Server wrote all %v messages to clients\n", n)
		ts.S2MChan <- true
	}
	lsplog.Vlogf(4, "Server waiting to close\n")
	<-ts.M2SChan
	lsplog.Vlogf(4, "Server closing\n")
	ts.Server.CloseAll()
	lsplog.Vlogf(4, "Server closed\n")
	ts.S2MChan <- false
}

// Create client and and run test
func (ts *SynchTestSystem) RunClient(clienti int) {
	hostport := fmt.Sprintf("localhost:%d", ts.Port)
	cli, err := NewLspClient(hostport, ts.Params)
	if err != nil {
		ts.Tester.Logf("Couldn't create client %d to server at %s\n",
			clienti, hostport)
		ts.Tester.FailNow()
	}
	ts.Clients[clienti] = cli
	id := cli.ConnId()
	ts.MapLock.Lock()
	ts.ClientMap[id] = clienti
	ts.MapLock.Unlock()
	lsplog.Vlogf(1, "Client %d created with id %d to server at %s\n",
		clienti, id, hostport)
	ts.C2MChan <- true

	// Write
	if ts.Mode != doservertoclient {
		lsplog.Vlogf(4, "Client %d (id %d) waiting to write\n", clienti, id)
		<-ts.M2CChan
		lsplog.Vlogf(3, "Client %d (id %d) writing messages\n", clienti, id)
		for nsent := 0; nsent < ts.Nmessages; nsent++ {
			v := ts.Data[clienti][nsent]
			cli.Write(synchi2b(v))
			if err != nil {
				ts.Tester.Logf("Client %d (id %d) could not send value %v\n",
					clienti, id, v)
				ts.Tester.Fail()
			}
		}
		lsplog.Vlogf(3, "Client %d (id %d) wrote all %v messages to server\n",
			clienti, id, ts.Nmessages)
		ts.C2MChan <- true
	}

	// Read
	if ts.Mode != doclienttoserver {
		lsplog.Vlogf(4, "Client %d (id %d) waiting to read\n", clienti, id)
		<-ts.M2CChan
		lsplog.Vlogf(4, "Client %d (id %d) reading messages\n", clienti, id)
		// Receive messages from server
		for nrcvd := 0; nrcvd < ts.Nmessages; nrcvd++ {
			b := cli.Read()
			if b == nil {
				ts.Tester.Logf("Client %d (id %d) failed to read value #%d\n",
					clienti, id, nrcvd)
				ts.Tester.Fail()
			}
			v := synchb2i(b)
			ev := ts.Data[clienti][nrcvd]
			if v != ev {
				ts.Tester.Logf("Client %d (id %d) received element #%v, value %v.  Expected %v\n",
					clienti, id, nrcvd, v, ev)
				ts.Tester.Fail()
			}
			lsplog.Vlogf(6, "Client %d (id %d) received element #%v, value %v.  Expected %v\n",
				clienti, id, nrcvd, v, ev)
		}
		lsplog.Vlogf(3, "Client %d (id %d) read all %v messages from servers\n",
			clienti, id, ts.Nmessages)
		ts.C2MChan <- true
	}
	lsplog.Vlogf(4, "Client %d (id %d) waiting to close\n", clienti, id)
	<-ts.M2CChan
	lsplog.Vlogf(4, "Client %d (id %d) closing\n", clienti, id)
	cli.Close()
	lsplog.Vlogf(4, "Client %d (id %d) done\n", clienti, id)
	ts.C2MChan <- false
}

// Turn network off and on
func (ts *SynchTestSystem) RunNetwork() {
	// Network initially on
	lspnet.SetWriteDropPercent(0)
	for ts.RunFlag {
		lsplog.Vlogf(4, "Network running.  Waiting for master\n")
		<-ts.M2NChan
		lsplog.Vlogf(4, "Turning off network\n")
		lspnet.SetWriteDropPercent(100)
		ts.N2MChan <- true

		lsplog.Vlogf(4, "Network off.  Waiting for master\n")
		<-ts.M2NChan
		lsplog.Vlogf(4, "Turning network on and delaying\n")
		lspnet.SetWriteDropPercent(0)
		ts.N2MChan <- true
		ts.synchdelay(2.0)
	}
	lsplog.Vlogf(4, "Network handler exiting\n")
}

// Enable network & then Wait until it signals that it is done
func (ts *SynchTestSystem) SynchNetwork() {
	lsplog.Vlogf(6, "Enabling Network\n")
	ts.M2NChan <- true
	lsplog.Vlogf(6, "Waiting for network\n")
	select {
	case d := <-ts.TChan:
		ts.Tester.Logf("Test reached time limit of %d waiting for network", d)
		ts.Tester.FailNow()
	case <-ts.N2MChan:
	}
	lsplog.Vlogf(6, "Got signal from network\n")
}

// Wait until server has signaled it is done
func (ts *SynchTestSystem) WaitServer() {
	lsplog.Vlogf(6, "Waiting for server\n")
	select {
	case d := <-ts.TChan:
		ts.Tester.Logf("Test reached time limit of %d waiting for server", d)
		ts.Tester.FailNow()
	case <-ts.S2MChan:
	}
	lsplog.Vlogf(6, "Got signal from server\n")
}

// Wait until clients have that they're done
func (ts *SynchTestSystem) WaitClients() {
	lsplog.Vlogf(6, "Waiting for clients\n")
	for i := 0; i < ts.Nclients; i++ {
		select {
		case d := <-ts.TChan:
			ts.Tester.Logf("Test reached time limit of %d waiting for client", d)
			ts.Tester.FailNow()
		case <-ts.C2MChan:
		}
	}

	lsplog.Vlogf(6, "Got signals from all clients\n")
}

// Let server proceed
func (ts *SynchTestSystem) ReadyServer() {
	lsplog.Vlogf(6, "Enabling server\n")
	ts.M2SChan <- true
}

// Let clients proceed
func (ts *SynchTestSystem) ReadyClients() {
	lsplog.Vlogf(6, "Enabling clients\n")
	for i := 0; i < ts.Nclients; i++ {
		ts.M2CChan <- true
	}
}

// Function to coordinate tests
func (ts *SynchTestSystem) Master() {
	// Wait until server and all clients are ready
	ts.WaitServer()
	ts.WaitClients()
	lsplog.Vlogf(4, "Server + all clients started.  Shut off network\n")
	ts.SynchNetwork()
	// Network Off
	// Enable client writes
	if ts.Mode != doservertoclient {
		ts.ReadyClients()
		ts.WaitClients()
	}
	// Do fast close of client.  The calls will not be able to complete
	if ts.Mode == doclienttoserver {
		ts.ReadyClients()
	}
	if ts.Mode != doservertoclient {
		// Turn on network and delay
		ts.SynchNetwork()
		if ts.Mode == doclienttoserver {
			ts.WaitClients()
		}
		ts.SynchNetwork()
		// Network off
		// Enable server reads
		ts.ReadyServer()
		ts.WaitServer()
	}
	// Do server writes
	if ts.Mode != doclienttoserver {
		ts.ReadyServer()
		ts.WaitServer()
	}
	// Do fast close of server.  The calls will not be able to complete
	if ts.Mode != doroundtrip {
		ts.ReadyServer()
	}
	if ts.Mode != doclienttoserver {
		// Turn on network
		ts.SynchNetwork()
		if ts.Mode != doroundtrip {
			// If did quick close, should get responses from server
			ts.WaitServer()
		}
		// Network off again
		ts.SynchNetwork()
		// Enable client reads
		ts.ReadyClients()
		ts.WaitClients()
		// Enable client closes
		ts.ReadyClients()
		ts.WaitClients()
	}
	// Final close by server
	if ts.Mode == doroundtrip {
		ts.ReadyServer()
		ts.WaitServer()
	}
	// Made it!
}

// Delay for fixed amount of time, specified in terms of epochs
func (ts *SynchTestSystem) synchdelay(epochfraction float64) {
	ms := int(epochfraction * float64(ts.Params.EpochMilliseconds))
	d := time.Duration(ms) * time.Millisecond
	if d.Nanoseconds() > 0 {
		time.Sleep(d)
	}
}

// Generate random port number
func synchrandport(rgen rand.Rand) int {
	return 3000 + rgen.Intn(50000)
}

func synchparams(lim, ms int) *LspParams {
	return &LspParams{lim, ms}
}

// Time Out test
func (ts *SynchTestSystem) RunTimeout(epochfraction float64) {
	lsplog.Vlogf(6, "Setting test to timeout after %.2f epochs\n", epochfraction)
	ts.synchdelay(epochfraction)
	ts.RunFlag = false
	ts.TChan <- int(epochfraction)
}

func TestRoundTrip1(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 1, 10, doroundtrip, 12, synchparams(5, 500))
}

func TestRoundTrip2(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 3, 10, doroundtrip, 12, synchparams(5, 500))
}

func TestRoundTrip3(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 5, 500, doroundtrip, 20, synchparams(5, 2000))
}

func TestServerFastClose1(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 1, 10, doserverfastclose, 12, synchparams(5, 500))
}

func TestServerFastClose2(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 3, 10, doserverfastclose, 12, synchparams(5, 500))
}

func TestServerFastClose3(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 5, 500, doserverfastclose, 20, synchparams(5, 2000))
}

func TestServerToClient1(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 1, 10, doservertoclient, 12, synchparams(5, 500))
}

func TestServerToClient2(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 3, 10, doservertoclient, 12, synchparams(5, 500))
}

func TestServerToClient3(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 5, 500, doservertoclient, 20, synchparams(5, 2000))
}

func TestClientToServer1(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 1, 10, doclienttoserver, 12, synchparams(5, 500))
}

func TestClientToServer2(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 3, 10, doclienttoserver, 12, synchparams(5, 500))
}

func TestClientToServer3(t *testing.T) {
	lsplog.SetVerbose(3)
	NewSynchTestSystem(t, 5, 500, doclienttoserver, 20, synchparams(5, 2000))
}
