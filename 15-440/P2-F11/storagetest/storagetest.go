package main

import (
	"flag"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/cacherpc"
	"github.com/kedebug/golang-programming/15-440/P2-F11/storageproto"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"regexp"
	"time"
)

type StorageTester struct {
	srv         *rpc.Client
	myhostport  string
	recv_revoke map[string]bool // whether we have received a RevokeLease for key x
	comp_revoke map[string]bool // whether we have replied the RevokeLease for key x
	delay       float32         // how long to delay the reply of RevokeLease
}

type TestFunc struct {
	name string
	f    func()
}

var portnum *int = flag.Int("port", 9019, "port # to listen on")
var testType *int = flag.Int("type", 1, "type of test, 1: jtest, 2: btest")
var numServer *int = flag.Int("N", 1, "(jtest only) total # of storage servers")
var myID *int = flag.Int("id", 1, "(jtest only) my id")
var testRegex *string = flag.String("t", "", "test to run")
var output io.Writer
var passCount int
var failCount int
var st *StorageTester

func initStorageTester(server string, myhostport string) *StorageTester {
	tester := &StorageTester{}
	tester.myhostport = myhostport
	tester.recv_revoke = make(map[string]bool)
	tester.comp_revoke = make(map[string]bool)
	// Create RPC connection to storage server
	srv, err := rpc.DialHTTP("tcp", server)
	if err != nil {
		fmt.Printf("Could not connect to server %s, returning nil\n", server)
		return nil
	}

	// Listen cacherpc
	rpc.Register(cacherpc.NewCacheRPC(tester))
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", fmt.Sprintf(":%d", *portnum))
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)

	tester.srv = srv
	return tester
}

func (st *StorageTester) ResetDelay() {
	st.delay = 0
}

func (st *StorageTester) SetDelay(f float32) {
	st.delay = f * (storageproto.LEASE_SECONDS + storageproto.LEASE_GUARD_SECONDS)
}

// Make StorageTester CacheRPC-able
func (st *StorageTester) RevokeLease(args *storageproto.RevokeLeaseArgs, reply *storageproto.RevokeLeaseReply) error {
	//fmt.Printf("Revoke Msg received %s\n", args.Key)
	st.recv_revoke[args.Key] = true
	st.comp_revoke[args.Key] = false
	time.Sleep(time.Duration(st.delay*1000) * time.Millisecond)
	st.comp_revoke[args.Key] = true
	reply.Status = storageproto.OK
	return nil
}

// Helper functions to test a single storage server
func (st *StorageTester) GetServers() (*storageproto.RegisterReply, error) {
	args := &storageproto.GetServersArgs{}
	var reply storageproto.RegisterReply
	err := st.srv.Call("StorageRPC.GetServers", args, &reply)
	return &reply, err
}

func (st *StorageTester) RegisterServer() (*storageproto.RegisterReply, error) {
	node := storageproto.Node{st.myhostport, uint32(*myID)}
	args := &storageproto.RegisterArgs{node}
	var reply storageproto.RegisterReply
	err := st.srv.Call("StorageRPC.Register", args, &reply)
	return &reply, err
}

func (st *StorageTester) Put(key, value string) (*storageproto.PutReply, error) {
	args := &storageproto.PutArgs{key, value}
	var reply storageproto.PutReply
	err := st.srv.Call("StorageRPC.Put", args, &reply)
	return &reply, err
}

func (st *StorageTester) Get(key string, wantlease bool) (*storageproto.GetReply, error) {
	args := &storageproto.GetArgs{key, wantlease, st.myhostport}
	var reply storageproto.GetReply
	err := st.srv.Call("StorageRPC.Get", args, &reply)
	return &reply, err
}

func (st *StorageTester) GetList(key string, wantlease bool) (*storageproto.GetListReply, error) {
	args := &storageproto.GetArgs{key, wantlease, st.myhostport}
	var reply storageproto.GetListReply
	err := st.srv.Call("StorageRPC.GetList", args, &reply)
	return &reply, err
}

func (st *StorageTester) RemoveFromList(key, removeitem string) (*storageproto.PutReply, error) {
	args := &storageproto.PutArgs{key, removeitem}
	var reply storageproto.PutReply
	err := st.srv.Call("StorageRPC.RemoveFromList", args, &reply)
	return &reply, err
}

func (st *StorageTester) AppendToList(key, newitem string) (*storageproto.PutReply, error) {
	args := &storageproto.PutArgs{key, newitem}
	var reply storageproto.PutReply
	err := st.srv.Call("StorageRPC.AppendToList", args, &reply)
	return &reply, err
}

// Check error and status
func checkErrorStatus(err error, status int, expectedStatus int) bool {
	if err != nil {
		fmt.Fprintln(output, "FAIL: unexpected error returned")
		failCount++
		return true
	}
	if status != expectedStatus {
		fmt.Fprintf(output, "FAIL: incorrect status %d, expected status %d\n", status, expectedStatus)
		failCount++
		return true
	}
	return false
}

// Check error
func checkError(err error, expectError bool) bool {
	if expectError {
		if err == nil {
			fmt.Fprintln(output, "FAIL: error should be returned")
			failCount++
			return true
		}
	} else {
		if err != nil {
			fmt.Fprintln(output, "FAIL: unexpected error returned (%s)", err)
			failCount++
			return true
		}
	}
	return false
}

// Check list
func checkList(list []string, expectedList []string) bool {
	if len(list) != len(expectedList) {
		fmt.Fprintf(output, "FAIL: incorrect list %v, expected list %v\n", list, expectedList)
		failCount++
		return true
	}
	m := make(map[string]bool)
	for _, s := range list {
		m[s] = true
	}
	for _, s := range expectedList {
		if m[s] == false {
			fmt.Fprintf(output, "FAIL: incorrect list %v, expected list %v\n", list, expectedList)
			failCount++
			return true
		}
	}
	return false
}

// We treat a RPC call finihsed in 0.5 seconds as OK
func isTimeOK(d time.Duration) bool {
	return d < 500*time.Millisecond
}

// Cache a key
func cacheKey(key string) bool {
	replyP, err := st.Put(key, "old-value")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return true
	}

	// get and cache key
	replyG, err := st.Get(key, true)
	if checkErrorStatus(err, replyG.Status, storageproto.OK) {
		return true
	}
	if !replyG.Lease.Granted {
		fmt.Fprintln(output, "FAIL: Failed to get lease")
		failCount++
		return true
	}
	return false
}

// Cache a list key
func cacheKeyList(key string) bool {
	replyP, err := st.AppendToList(key, "old-value")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return true
	}

	// get and cache key
	replyL, err := st.GetList(key, true)
	if checkErrorStatus(err, replyL.Status, storageproto.OK) {
		return true
	}
	if !replyL.Lease.Granted {
		fmt.Fprintln(output, "FAIL: Failed to get lease")
		failCount++
		return true
	}
	return false
}

/////////////////////////////////////////////
//  test storage server initialization
/////////////////////////////////////////////

// make sure to run N-1 servers in shell before entering this function
func testInitStorageServers() {
	// test get server
	replyR, err := st.GetServers()
	if checkError(err, false) {
		return
	}
	if replyR.Ready {
		fmt.Fprintln(output, "FAIL: storage system should not be ready", err)
		failCount++
		return
	}

	// test register
	replyR, err = st.RegisterServer()
	if checkError(err, false) {
		return
	}
	if !replyR.Ready || replyR.Servers == nil {
		fmt.Fprintln(output, "FAIL: storage system should be ready", err)
		failCount++
		return
	}
	if len(replyR.Servers) != (*numServer) {
		fmt.Printf("numServer=%d\n", *numServer)
		for _, s := range replyR.Servers {
			fmt.Printf("server: %s, id=%d\n", s.HostPort, s.NodeID)
		}
		fmt.Fprintln(output, "FAIL: storage seystem returned wrong serverlist", err)
		failCount++
		return
	}

	// test key range
	replyG, err := st.Get("wrongkey:1", false)
	if checkErrorStatus(err, replyG.Status, storageproto.EWRONGSERVER) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

/////////////////////////////////////////////
//  test basic storage operations
/////////////////////////////////////////////

// Get keys without and with wantlease
func testPutGet() {
	// get an invalid key
	replyG, err := st.Get("nullkey:1", false)
	if checkErrorStatus(err, replyG.Status, storageproto.EKEYNOTFOUND) {
		return
	}

	replyP, err := st.Put("keyputget:1", "value")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// without asking for a lease
	replyG, err = st.Get("keyputget:1", false)
	if checkErrorStatus(err, replyG.Status, storageproto.OK) {
		return
	}
	if replyG.Value != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if replyG.Lease.Granted {
		fmt.Fprintln(output, "FAIL: did not apply for lease")
		failCount++
		return
	}

	// now I want a lease this time
	replyG, err = st.Get("keyputget:1", true)
	if checkErrorStatus(err, replyG.Status, storageproto.OK) {
		return
	}
	if replyG.Value != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if !replyG.Lease.Granted {
		fmt.Fprintln(output, "FAIL: did not get lease")
		failCount++
		return
	}

	fmt.Fprintln(output, "PASS")
	passCount++
}

// list related operations
func testAppendGetRemoveList() {
	// test AppendToList
	replyP, err := st.AppendToList("keylist:1", "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// test GetList
	replyL, err := st.GetList("keylist:1", false)
	if checkErrorStatus(err, replyL.Status, storageproto.OK) {
		return
	}
	if len(replyL.Value) != 1 || replyL.Value[0] != "value1" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}

	// test AppendToList for a duplicated item
	replyP, err = st.AppendToList("keylist:1", "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.EITEMEXISTS) {
		return
	}

	// test AppendToList for a different item
	replyP, err = st.AppendToList("keylist:1", "value2")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// test RemoveFromList for the first item
	replyP, err = st.RemoveFromList("keylist:1", "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// test RemoveFromList for removed item
	replyP, err = st.RemoveFromList("keylist:1", "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.EITEMNOTFOUND) {
		return
	}

	// test GetList after RemoveFromList
	replyL, err = st.GetList("keylist:1", false)
	if checkErrorStatus(err, replyL.Status, storageproto.OK) {
		return
	}
	if len(replyL.Value) != 1 || replyL.Value[0] != "value2" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}

	fmt.Fprintln(output, "PASS")
	passCount++
}

/////////////////////////////////////////////
//  test revoke related
/////////////////////////////////////////////

// Without leasing, we should not expect revoke
func testUpdateWithoutLease() {
	key := "revokekey:0"

	replyP, err := st.Put(key, "value")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// get without caching this item
	replyG, err := st.Get(key, false)
	if checkErrorStatus(err, replyG.Status, storageproto.OK) {
		return
	}

	// update this key
	replyP, err = st.Put(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// get without caching this item
	replyG, err = st.Get(key, false)
	if checkErrorStatus(err, replyG.Status, storageproto.OK) {
		return
	}

	if st.recv_revoke[key] {
		fmt.Fprintln(output, "FAIL: expect no revoke")
		failCount++
		return
	}

	fmt.Fprintln(output, "PASS")
	passCount++
}

// updating a key before its lease expires
// expect a revoke msg from storage server
func testUpdateBeforeLeaseExpire() {
	key := "revokekey:1"

	if cacheKey(key) {
		return
	}

	// update this key
	replyP, err := st.Put(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// read it back
	replyG, err := st.Get(key, false)
	if checkErrorStatus(err, replyG.Status, storageproto.OK) {
		return
	}
	if replyG.Value != "value1" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}

	// expect a revoke msg, check if we receive it
	if !st.recv_revoke[key] {
		fmt.Fprintln(output, "FAIL: did not receive revoke")
		failCount++
		return
	}

	fmt.Fprintln(output, "PASS")
	passCount++
}

// updating a key after its lease expires
// expect no revoke msg received from storage server
func testUpdateAfterLeaseExpire() {
	key := "revokekey:2"

	if cacheKey(key) {
		return
	}

	// sleep until lease expires
	time.Sleep((storageproto.LEASE_SECONDS + storageproto.LEASE_GUARD_SECONDS + 1) * time.Second)

	// update this key
	replyP, err := st.Put(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// read back this item
	replyG, err := st.Get(key, false)
	if checkErrorStatus(err, replyG.Status, storageproto.OK) {
		return
	}
	if replyG.Value != "value1" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}

	// expect no revoke msg, check if we receive any
	if st.recv_revoke[key] {
		fmt.Fprintln(output, "FAIL: should not receive revoke")
		failCount++
		return
	}

	fmt.Fprintln(output, "PASS")
	passCount++
}

// helper function for delayed revoke tests
func delayedRevoke(key string, f func() bool) bool {
	if cacheKey(key) {
		return true
	}

	// trigger a delayed revocation in background
	var replyP *storageproto.PutReply
	var err error
	putCh := make(chan bool)
	doneCh := make(chan bool)
	go func() {
		// put key1 again to trigger a revoke
		replyP, err = st.Put(key, "new-value")
		putCh <- true
	}()
	// ensure Put has gotten to server
	time.Sleep(100 * time.Millisecond)

	// run rest of function in go routine to allow for timeouts
	go func() {
		// run rest of test function
		ret := f()
		// wait for put to complete
		<-putCh
		// check for failures
		if ret {
			doneCh <- true
			return
		}
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			doneCh <- true
			return
		}
		doneCh <- false
	}()

	// wait for test completion or timeout
	select {
	case ret := <-doneCh:
		return ret
	case <-time.After((storageproto.LEASE_SECONDS + storageproto.LEASE_GUARD_SECONDS + 1) * time.Second):
		break
	}
	fmt.Fprintln(output, "FAIL: timeout, may erroneously increase test count")
	failCount++
	return true
}

// when revoking leases for key "x",
// storage server should not block queries for other keys
func testDelayedRevokeWithoutBlocking() {
	st.SetDelay(0.5)
	defer st.ResetDelay()

	key1 := "revokekey:3"
	key2 := "revokekey:4"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// put key2, this should not block
		replyP, err := st.Put(key2, "value")
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			fmt.Fprintln(output, "FAIL: concurrent Put got blocked")
			failCount++
			return true
		}

		ts = time.Now()
		// get key2, this should not block
		replyG, err := st.Get(key2, false)
		if checkErrorStatus(err, replyG.Status, storageproto.OK) {
			return true
		}
		if replyG.Value != "value" {
			fmt.Fprintln(output, "FAIL: get got wrong value")
			failCount++
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			fmt.Fprintln(output, "FAIL: concurrent Get got blocked")
			failCount++
			return true
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should stop leasing for "x"
// before revoking completes or old lease expires.
// this function tests the former case
func testDelayedRevokeWithLeaseRequest1() {
	st.SetDelay(0.5) // Revoke finishes before lease expires
	defer st.ResetDelay()

	key1 := "revokekey:5"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// get key1 and want a lease
		replyG, err := st.Get(key1, true)
		if checkErrorStatus(err, replyG.Status, storageproto.OK) {
			return true
		}
		if isTimeOK(time.Since(ts)) {
			// in this case, server should reply old value and refuse lease
			if replyG.Lease.Granted || replyG.Value != "old-value" {
				fmt.Fprintln(output, "FAIL: server should return old value and not grant lease")
				failCount++
				return true
			}
		} else {
			if !st.comp_revoke[key1] || (!replyG.Lease.Granted || replyG.Value != "new-value") {
				fmt.Fprintln(output, "FAIL: server should return new value and grant lease")
				failCount++
				return true
			}
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should stop leasing for "x"
// before revoking completes or old lease expires.
// this function tests the latter case
// The diff from the previous test is
// st.comp_revoke[key1] in the else case
func testDelayedRevokeWithLeaseRequest2() {
	st.SetDelay(2) // Lease expires before revoking finishes
	defer st.ResetDelay()

	key1 := "revokekey:15"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// get key1 and want a lease
		replyG, err := st.Get(key1, true)
		if checkErrorStatus(err, replyG.Status, storageproto.OK) {
			return true
		}
		if isTimeOK(time.Since(ts)) {
			// in this case, server should reply old value and refuse lease
			if replyG.Lease.Granted || replyG.Value != "old-value" {
				fmt.Fprintln(output, "FAIL: server should return old value and not grant lease")
				failCount++
				return true
			}
		} else {
			if st.comp_revoke[key1] || (!replyG.Lease.Granted || replyG.Value != "new-value") {
				fmt.Fprintln(output, "FAIL: server should return new value and grant lease")
				failCount++
				return true
			}
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should hold upcoming updates for "x",
// until either all revocations complete or the lease expires
// this function tests the former case
func testDelayedRevokeWithUpdate1() {
	st.SetDelay(0.5) // revocation takes longer, but still completes before lease expires
	defer st.ResetDelay()

	key1 := "revokekey:6"

	// function called during revoke of key1
	f := func() bool {
		// put key1, this should block
		replyP, err := st.Put(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			return true
		}
		if !st.comp_revoke[key1] {
			fmt.Fprintln(output, "FAIL: storage server should hold modification to key x during finishing revocating all lease holders of x")
			failCount++
			return true
		}
		replyG, err := st.Get(key1, false)
		if checkErrorStatus(err, replyG.Status, storageproto.OK) {
			return true
		}
		if replyG.Value != "newnew-value" {
			fmt.Fprintln(output, "FAIL: got wrong value")
			failCount++
			return true
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should hold upcoming updates for "x",
// until either all revocations complete or the lease expires
// this function tests the latter case
func testDelayedRevokeWithUpdate2() {
	st.SetDelay(2) // lease expires before revocation completes
	defer st.ResetDelay()

	key1 := "revokekey:7"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// put key1, this should block
		replyP, err := st.Put(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			return true
		}
		d := time.Since(ts)
		if d < (storageproto.LEASE_SECONDS+storageproto.LEASE_GUARD_SECONDS-1)*time.Second {
			fmt.Fprintln(output, "FAIL: storage server should hold this Put until leases expires key1")
			failCount++
			return true
		}
		if st.comp_revoke[key1] {
			fmt.Fprintln(output, "FAIL: storage server should not block this Put till the lease revoke of key1")
			failCount++
			return true
		}
		replyG, err := st.Get(key1, false)
		if checkErrorStatus(err, replyG.Status, storageproto.OK) {
			return true
		}
		if replyG.Value != "newnew-value" {
			fmt.Fprintln(output, "FAIL: got wrong value")
			failCount++
			return true
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// remote libstores may not even reply all RevokeLease RPC calls.
// in this case, service should continue after lease expires
func testDelayedRevokeWithUpdate3() {
	st.SetDelay(2) // lease expires before revocation completes
	defer st.ResetDelay()

	key1 := "revokekey:8"

	// function called during revoke of key1
	f := func() bool {
		// sleep here until lease expires on the remote server
		time.Sleep((storageproto.LEASE_SECONDS + storageproto.LEASE_GUARD_SECONDS) * time.Second)

		// put key1, this should not block
		ts := time.Now()
		replyP, err := st.Put(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			fmt.Fprintln(output, "FAIL: storage server should not block this Put")
			failCount++
			return true
		}
		// get key1 and want lease, this should not block
		ts = time.Now()
		replyG, err := st.Get(key1, true)
		if checkErrorStatus(err, replyG.Status, storageproto.OK) {
			return true
		}
		if replyG.Value != "newnew-value" {
			fmt.Fprintln(output, "FAIL: got wrong value")
			failCount++
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			fmt.Fprintln(output, "FAIL: storage server should not block this Get")
			failCount++
			return true
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Without leasing, we should not expect revoke
func testUpdateListWithoutLease() {
	key := "revokelistkey:0"

	replyP, err := st.AppendToList(key, "value")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// get without caching this item
	replyL, err := st.GetList(key, false)
	if checkErrorStatus(err, replyL.Status, storageproto.OK) {
		return
	}

	// update this key
	replyP, err = st.AppendToList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}
	replyP, err = st.RemoveFromList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// get without caching this item
	replyL, err = st.GetList(key, false)
	if checkErrorStatus(err, replyL.Status, storageproto.OK) {
		return
	}

	if st.recv_revoke[key] {
		fmt.Fprintln(output, "FAIL: expect no revoke")
		failCount++
		return
	}

	fmt.Fprintln(output, "PASS")
	passCount++
}

// updating a key before its lease expires
// expect a revoke msg from storage server
func testUpdateListBeforeLeaseExpire() {
	key := "revokelistkey:1"

	if cacheKeyList(key) {
		return
	}

	// update this key
	replyP, err := st.AppendToList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}
	replyP, err = st.RemoveFromList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// read it back
	replyL, err := st.GetList(key, false)
	if checkErrorStatus(err, replyL.Status, storageproto.OK) {
		return
	}
	if len(replyL.Value) != 1 || replyL.Value[0] != "old-value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}

	// expect a revoke msg, check if we receive it
	if !st.recv_revoke[key] {
		fmt.Fprintln(output, "FAIL: did not receive revoke")
		failCount++
		return
	}

	fmt.Fprintln(output, "PASS")
	passCount++
}

// updating a key after its lease expires
// expect no revoke msg received from storage server
func testUpdateListAfterLeaseExpire() {
	key := "revokelistkey:2"

	if cacheKeyList(key) {
		return
	}

	// sleep until lease expires
	time.Sleep((storageproto.LEASE_SECONDS + storageproto.LEASE_GUARD_SECONDS + 1) * time.Second)

	// update this key
	replyP, err := st.AppendToList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}
	replyP, err = st.RemoveFromList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storageproto.OK) {
		return
	}

	// read back this item
	replyL, err := st.GetList(key, false)
	if checkErrorStatus(err, replyL.Status, storageproto.OK) {
		return
	}
	if len(replyL.Value) != 1 || replyL.Value[0] != "old-value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}

	// expect no revoke msg, check if we receive any
	if st.recv_revoke[key] {
		fmt.Fprintln(output, "FAIL: should not receive revoke")
		failCount++
		return
	}

	fmt.Fprintln(output, "PASS")
	passCount++
}

// helper function for delayed revoke tests
func delayedRevokeList(key string, f func() bool) bool {
	if cacheKeyList(key) {
		return true
	}

	// trigger a delayed revocation in background
	var replyP *storageproto.PutReply
	var err error
	appendCh := make(chan bool)
	doneCh := make(chan bool)
	go func() {
		// append key to trigger a revoke
		replyP, err = st.AppendToList(key, "new-value")
		appendCh <- true
	}()
	// ensure Put has gotten to server
	time.Sleep(100 * time.Millisecond)

	// run rest of function in go routine to allow for timeouts
	go func() {
		// run rest of test function
		ret := f()
		// wait for append to complete
		<-appendCh
		// check for failures
		if ret {
			doneCh <- true
			return
		}
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			doneCh <- true
			return
		}
		doneCh <- false
	}()

	// wait for test completion or timeout
	select {
	case ret := <-doneCh:
		return ret
	case <-time.After((storageproto.LEASE_SECONDS + storageproto.LEASE_GUARD_SECONDS + 1) * time.Second):
		break
	}
	fmt.Fprintln(output, "FAIL: timeout, may erroneously increase test count")
	failCount++
	return true
}

// when revoking leases for key "x",
// storage server should not block queries for other keys
func testDelayedRevokeListWithoutBlocking() {
	st.SetDelay(0.5)
	defer st.ResetDelay()

	key1 := "revokelistkey:3"
	key2 := "revokelistkey:4"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// put key2, this should not block
		replyP, err := st.AppendToList(key2, "value")
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			fmt.Fprintln(output, "FAIL: concurrent Append got blocked")
			failCount++
			return true
		}

		ts = time.Now()
		// get key2, this should not block
		replyL, err := st.GetList(key2, false)
		if checkErrorStatus(err, replyL.Status, storageproto.OK) {
			return true
		}
		if len(replyL.Value) != 1 || replyL.Value[0] != "value" {
			fmt.Fprintln(output, "FAIL: GetList got wrong value")
			failCount++
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			fmt.Fprintln(output, "FAIL: concurrent GetList got blocked")
			failCount++
			return true
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should stop leasing for "x"
// before revoking completes or old lease expires.
// this function tests the former case
func testDelayedRevokeListWithLeaseRequest1() {
	st.SetDelay(0.5) // Revoke finishes before lease expires
	defer st.ResetDelay()

	key1 := "revokelistkey:5"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// get key1 and want a lease
		replyL, err := st.GetList(key1, true)
		if checkErrorStatus(err, replyL.Status, storageproto.OK) {
			return true
		}
		if isTimeOK(time.Since(ts)) {
			// in this case, server should reply old value and refuse lease
			if replyL.Lease.Granted || len(replyL.Value) != 1 || replyL.Value[0] != "old-value" {
				fmt.Fprintln(output, "FAIL: server should return old value and not grant lease")
				failCount++
				return true
			}
		} else {
			if checkList(replyL.Value, []string{"old-value", "new-value"}) {
				return true
			}
			if !st.comp_revoke[key1] || !replyL.Lease.Granted {
				fmt.Fprintln(output, "FAIL: server should grant lease in this case")
				failCount++
				return true
			}
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should stop leasing for "x"
// before revoking completes or old lease expires.
// this function tests the latter case
// The diff from the previous test is
// st.comp_revoke[key1] in the else case
func testDelayedRevokeListWithLeaseRequest2() {
	st.SetDelay(2) // Lease expires before revoking finishes
	defer st.ResetDelay()

	key1 := "revokelistkey:15"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// get key1 and want a lease
		replyL, err := st.GetList(key1, true)
		if checkErrorStatus(err, replyL.Status, storageproto.OK) {
			return true
		}
		if isTimeOK(time.Since(ts)) {
			// in this case, server should reply old value and refuse lease
			if replyL.Lease.Granted || len(replyL.Value) != 1 || replyL.Value[0] != "old-value" {
				fmt.Fprintln(output, "FAIL: server should return old value and not grant lease")
				failCount++
				return true
			}
		} else {
			if checkList(replyL.Value, []string{"old-value", "new-value"}) {
				return true
			}
			if st.comp_revoke[key1] || !replyL.Lease.Granted {
				fmt.Fprintln(output, "FAIL: server should grant lease in this case")
				failCount++
				return true
			}
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should hold upcoming updates for "x",
// until either all revocations complete or the lease expires
// this function tests the former case
func testDelayedRevokeListWithUpdate1() {
	st.SetDelay(0.5) // revocation takes longer, but still completes before lease expires
	defer st.ResetDelay()

	key1 := "revokelistkey:6"

	// function called during revoke of key1
	f := func() bool {
		// put key1, this should block
		replyP, err := st.AppendToList(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			return true
		}
		if !st.comp_revoke[key1] {
			fmt.Fprintln(output, "FAIL: storage server should hold modification to key x during finishing revocating all lease holders of x")
			failCount++
			return true
		}
		replyL, err := st.GetList(key1, false)
		if checkErrorStatus(err, replyL.Status, storageproto.OK) {
			return true
		}
		if checkList(replyL.Value, []string{"old-value", "new-value", "newnew-value"}) {
			return true
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should hold upcoming updates for "x",
// until either all revocations complete or the lease expires
// this function tests the latter case
func testDelayedRevokeListWithUpdate2() {
	st.SetDelay(2) // lease expires before revocation completes
	defer st.ResetDelay()

	key1 := "revokelistkey:7"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// put key1, this should block
		replyP, err := st.AppendToList(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			return true
		}
		d := time.Since(ts)
		if d < (storageproto.LEASE_SECONDS+storageproto.LEASE_GUARD_SECONDS-1)*time.Second {
			fmt.Fprintln(output, "FAIL: storage server should hold this Put until leases expires key1")
			failCount++
			return true
		}
		if st.comp_revoke[key1] {
			fmt.Fprintln(output, "FAIL: storage server should not block this Put till the lease revoke of key1")
			failCount++
			return true
		}
		replyL, err := st.GetList(key1, false)
		if checkErrorStatus(err, replyL.Status, storageproto.OK) {
			return true
		}
		if checkList(replyL.Value, []string{"old-value", "new-value", "newnew-value"}) {
			return true
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// remote libstores may not even reply all RevokeLease RPC calls.
// in this case, service should continue after lease expires
func testDelayedRevokeListWithUpdate3() {
	st.SetDelay(2) // lease expires before revocation completes
	defer st.ResetDelay()

	key1 := "revokelistkey:8"

	// function called during revoke of key1
	f := func() bool {
		// sleep here until lease expires on the remote server
		time.Sleep((storageproto.LEASE_SECONDS + storageproto.LEASE_GUARD_SECONDS) * time.Second)

		// put key1, this should not block
		ts := time.Now()
		replyP, err := st.AppendToList(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storageproto.OK) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			fmt.Fprintln(output, "FAIL: storage server should not block this Put")
			failCount++
			return true
		}
		// get key1 and want lease, this should not block
		ts = time.Now()
		replyL, err := st.GetList(key1, true)
		if checkErrorStatus(err, replyL.Status, storageproto.OK) {
			return true
		}
		if checkList(replyL.Value, []string{"old-value", "new-value", "newnew-value"}) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			fmt.Fprintln(output, "FAIL: storage server should not block this Get")
			failCount++
			return true
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

func main() {
	output = os.Stderr
	passCount = 0
	failCount = 0

	jtests := []TestFunc{
		// initialization tests
		TestFunc{"testInitStorageServers", testInitStorageServers}}

	btests := []TestFunc{
		// node ID stuff
		// basic tests
		TestFunc{"testPutGet", testPutGet},
		TestFunc{"testAppendGetRemoveList", testAppendGetRemoveList},
		// lease related tests
		TestFunc{"testUpdateWithoutLease", testUpdateWithoutLease},
		TestFunc{"testUpdateBeforeLeaseExpire", testUpdateBeforeLeaseExpire},
		TestFunc{"testUpdateAfterLeaseExpire", testUpdateAfterLeaseExpire},
		TestFunc{"testDelayedRevokeWithoutBlocking", testDelayedRevokeWithoutBlocking},
		TestFunc{"testDelayedRevokeWithLeaseRequest1", testDelayedRevokeWithLeaseRequest1},
		TestFunc{"testDelayedRevokeWithLeaseRequest2", testDelayedRevokeWithLeaseRequest2},
		TestFunc{"testDelayedRevokeWithUpdate1", testDelayedRevokeWithUpdate1},
		TestFunc{"testDelayedRevokeWithUpdate2", testDelayedRevokeWithUpdate2},
		TestFunc{"testDelayedRevokeWithUpdate3", testDelayedRevokeWithUpdate3},
		TestFunc{"testUpdateListWithoutLease", testUpdateListWithoutLease},
		TestFunc{"testUpdateListBeforeLeaseExpire", testUpdateListBeforeLeaseExpire},
		TestFunc{"testUpdateListAfterLeaseExpire", testUpdateListAfterLeaseExpire},
		TestFunc{"testDelayedRevokeListWithoutBlocking", testDelayedRevokeListWithoutBlocking},
		TestFunc{"testDelayedRevokeListWithLeaseRequest1", testDelayedRevokeListWithLeaseRequest1},
		TestFunc{"testDelayedRevokeListWithLeaseRequest2", testDelayedRevokeListWithLeaseRequest2},
		TestFunc{"testDelayedRevokeListWithUpdate1", testDelayedRevokeListWithUpdate1},
		TestFunc{"testDelayedRevokeListWithUpdate2", testDelayedRevokeListWithUpdate2},
		TestFunc{"testDelayedRevokeListWithUpdate3", testDelayedRevokeListWithUpdate3}}

	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal("usage: storagetest <storage master>")
	}

	// Run the tests with a single tester
	st = initStorageTester(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum))
	if st == nil {
		fmt.Println("Failed to init StorageTester")
		return
	}

	// Run btests
	switch *testType {
	case 1:
		for _, t := range jtests {
			if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
				fmt.Println("Starting " + t.name + ":")
				t.f()
			}
		}
	case 2:
		for _, t := range btests {
			if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
				fmt.Println("Starting " + t.name + ":")
				t.f()
			}
		}
	}

	fmt.Printf("Passed (%d/%d) tests\n", passCount, passCount+failCount)
}
