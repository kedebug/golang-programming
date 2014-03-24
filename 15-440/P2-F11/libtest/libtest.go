package main

import (
	"flag"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/libstore"
	"github.com/kedebug/golang-programming/15-440/P2-F11/proxycounter"
	"github.com/kedebug/golang-programming/15-440/P2-F11/storageproto"
	"github.com/kedebug/golang-programming/15-440/P2-F11/storagerpc"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type TestFunc struct {
	name string
	f    func()
}

var portnum *int = flag.Int("port", 9010, "port # to listen on")
var testRegex *string = flag.String("t", "", "test to run")
var output io.Writer
var passCount int
var failCount int
var pc *proxycounter.ProxyCounter
var ls *libstore.Libstore
var revokeConn *rpc.Client

// Initialize proxy and libstore
func initLibstore(storage string, server string, myhostport string, flags int) net.Listener {
	// Start proxy
	l, err := net.Listen("tcp", server)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	pc = proxycounter.NewProxyCounter(storage, server)
	if pc == nil {
		fmt.Println("Could not start proxy")
		return nil
	}
	srpc := storagerpc.NewStorageRPC(pc)
	rpc.Register(srpc)
	rpc.HandleHTTP()
	go http.Serve(l, nil)

	// Start libstore
	ls, err = libstore.NewLibstore(server, myhostport, flags)
	if ls == nil || err != nil {
		fmt.Println("Could not start libstore")
		return nil
	}
	return l
}

// Cleanup libstore and rpc hooks
func cleanupLibstore(l net.Listener) {
	// Close listener to stop http serve thread
	if l != nil {
		l.Close()
	}
	// Recreate default http serve mux
	http.DefaultServeMux = http.NewServeMux()
	// Recreate default rpc server
	rpc.DefaultServer = rpc.NewServer()
	// Unset libstore just in case
	ls = nil
}

// Force key into cache by requesting 2 * QUERY_CACHE_THRESH gets
func forceCacheGet(key string, value string) {
	ls.Put(key, value)
	for i := 0; i < 2*storageproto.QUERY_CACHE_THRESH; i++ {
		ls.Get(key)
	}
}

// Force key into cache by requesting 2 * QUERY_CACHE_THRESH get lists
func forceCacheGetList(key string, value string) {
	ls.AppendToList(key, value)
	for i := 0; i < 2*storageproto.QUERY_CACHE_THRESH; i++ {
		ls.GetList(key)
	}
}

// Revoke lease
func revokeLease(key string) (error, int) {
	args := &storageproto.RevokeLeaseArgs{key}
	var reply storageproto.RevokeLeaseReply
	err := revokeConn.Call("CacheRPC.RevokeLease", args, &reply)
	return err, reply.Status
}

// Check rpc and byte count limits
func checkLimits(rpcCountLimit uint32, byteCountLimit uint32) bool {
	if pc.GetRpcCount() > rpcCountLimit {
		fmt.Fprintln(output, "FAIL: using too many RPCs")
		failCount++
		return true
	}
	if pc.GetByteCount() > byteCountLimit {
		fmt.Fprintln(output, "FAIL: transferring too much data")
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
			fmt.Fprintln(output, "FAIL: unexpected error returned")
			failCount++
			return true
		}
	}
	return false
}

// Test libstore returns nil when it cannot connect to the server
func testNonexistentServer() {
	if l, err := libstore.NewLibstore(fmt.Sprintf("localhost:%d", *portnum), fmt.Sprintf("localhost:%d", *portnum), libstore.NONE); l == nil || err != nil {
		fmt.Fprintln(output, "PASS")
		passCount++
	} else {
		fmt.Fprintln(output, "FAIL: libstore does not return nil when it cannot connect to nonexistent storage server")
		failCount++
	}
	cleanupLibstore(nil)
}

// Never request leases when myhostport is ""
func testNoLeases() {
	l := initLibstore(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum), "", libstore.NONE)
	if l == nil {
		fmt.Fprintln(output, "FAIL: could not init libstore")
		failCount++
		return
	}
	defer cleanupLibstore(l)
	pc.Reset()
	forceCacheGet("key:", "value")
	if pc.GetLeaseRequestCount() > 0 {
		fmt.Fprintln(output, "FAIL: should not request leases when myhostport is \"\"")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Always request leases when flags is ALWAYS_LEASE
func testAlwaysLeases() {
	l := initLibstore(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum), fmt.Sprintf("localhost:%d", *portnum), libstore.ALWAYS_LEASE)
	if l == nil {
		fmt.Fprintln(output, "FAIL: could not init libstore")
		failCount++
		return
	}
	defer cleanupLibstore(l)
	pc.Reset()
	ls.Put("key:", "value")
	ls.Get("key:")
	if pc.GetLeaseRequestCount() == 0 {
		fmt.Fprintln(output, "FAIL: should always request leases when flags is ALWAYS_LEASE")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle get error
func testGetError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	_, err := ls.Get("key:1")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle get error reply status
func testGetErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storageproto.EKEYNOTFOUND)
	defer pc.OverrideOff()
	_, err := ls.Get("key:2")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle valid get
func testGetValid() {
	ls.Put("key:3", "value")
	pc.Reset()
	v, err := ls.Get("key:3")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle put error
func testPutError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	err := ls.Put("key:4", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle put error reply status
func testPutErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storageproto.EPUTFAILED)
	defer pc.OverrideOff()
	err := ls.Put("key:5", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle valid put
func testPutValid() {
	pc.Reset()
	err := ls.Put("key:6", "value")
	if checkError(err, false) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	v, err := ls.Get("key:6")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle get list error
func testGetListError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	_, err := ls.GetList("keylist:1")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle get list error reply status
func testGetListErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storageproto.EITEMNOTFOUND)
	defer pc.OverrideOff()
	_, err := ls.GetList("keylist:2")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle valid get list
func testGetListValid() {
	ls.AppendToList("keylist:3", "value")
	pc.Reset()
	v, err := ls.GetList("keylist:3")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle append to list error
func testAppendToListError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	err := ls.AppendToList("keylist:4", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle append to list error reply status
func testAppendToListErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storageproto.EITEMEXISTS)
	defer pc.OverrideOff()
	err := ls.AppendToList("keylist:5", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle valid append to list
func testAppendToListValid() {
	pc.Reset()
	err := ls.AppendToList("keylist:6", "value")
	if checkError(err, false) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	v, err := ls.GetList("keylist:6")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle remove from list error
func testRemoveFromListError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	err := ls.RemoveFromList("keylist:7", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle remove from list error reply status
func testRemoveFromListErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storageproto.EITEMNOTFOUND)
	defer pc.OverrideOff()
	err := ls.RemoveFromList("keylist:8", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Handle valid remove from list
func testRemoveFromListValid() {
	err := ls.AppendToList("keylist:9", "value1")
	if checkError(err, false) {
		return
	}
	err = ls.AppendToList("keylist:9", "value2")
	if checkError(err, false) {
		return
	}
	pc.Reset()
	err = ls.RemoveFromList("keylist:9", "value1")
	if checkError(err, false) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	v, err := ls.GetList("keylist:9")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value2" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache < limit test for get
func testCacheGetLimit() {
	pc.Reset()
	ls.Put("keycacheget:1", "value")
	for i := 0; i < storageproto.QUERY_CACHE_THRESH-1; i++ {
		ls.Get("keycacheget:1")
	}
	if pc.GetLeaseRequestCount() > 0 {
		fmt.Fprintln(output, "FAIL: should not request lease")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache > limit test for get
func testCacheGetLimit2() {
	pc.Reset()
	forceCacheGet("keycacheget:2", "value")
	if pc.GetLeaseRequestCount() == 0 {
		fmt.Fprintln(output, "FAIL: should have requested lease")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Doesn't call server when using cache for get
func testCacheGetCorrect() {
	forceCacheGet("keycacheget:3", "value")
	pc.Reset()
	for i := 0; i < 100*storageproto.QUERY_CACHE_THRESH; i++ {
		v, err := ls.Get("keycacheget:3")
		if checkError(err, false) {
			return
		}
		if v != "value" {
			fmt.Fprintln(output, "FAIL: got wrong value from cache")
			failCount++
			return
		}
	}
	if pc.GetRpcCount() > 0 {
		fmt.Fprintln(output, "FAIL: should not contact server when using cache")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache respects granted flag for get
func testCacheGetLeaseNotGranted() {
	pc.DisableLease()
	defer pc.EnableLease()
	forceCacheGet("keycacheget:4", "value")
	pc.Reset()
	v, err := ls.Get("keycacheget:4")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		fmt.Fprintln(output, "FAIL: not respecting lease granted flag")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache requests leases until granted for get
func testCacheGetLeaseNotGranted2() {
	pc.DisableLease()
	defer pc.EnableLease()
	forceCacheGet("keycacheget:5", "value")
	pc.Reset()
	forceCacheGet("keycacheget:5", "value")
	if pc.GetLeaseRequestCount() == 0 {
		fmt.Fprintln(output, "FAIL: not requesting leases after lease wasn't granted")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache respects lease timeout for get
func testCacheGetLeaseTimeout() {
	pc.OverrideLeaseSeconds(1)
	defer pc.OverrideLeaseSeconds(0)
	forceCacheGet("keycacheget:6", "value")
	time.Sleep(2 * time.Second)
	pc.Reset()
	v, err := ls.Get("keycacheget:6")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		fmt.Fprintln(output, "FAIL: not respecting lease timeout")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache memory leak for get
func testCacheGetMemoryLeak() {
	pc.OverrideLeaseSeconds(1)
	defer pc.OverrideLeaseSeconds(0)

	var memstats runtime.MemStats
	var initAlloc uint64
	var midAlloc uint64
	var finalAlloc uint64
	longValue := strings.Repeat("this sentence is 30 char long\n", 30)

	// Run garbage collection and get memory stats
	runtime.GC()
	runtime.ReadMemStats(&memstats)
	initAlloc = memstats.Alloc

	// Cache a lot of data
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("keymemleakget:%d", i)
		pc.Reset()
		forceCacheGet(key, longValue)
		if pc.GetLeaseRequestCount() == 0 {
			fmt.Fprintln(output, "FAIL: not requesting leases")
			failCount++
			return
		}
		pc.Reset()
		v, err := ls.Get(key)
		if checkError(err, false) {
			return
		}
		if v != longValue {
			fmt.Fprintln(output, "FAIL: got wrong value")
			failCount++
			return
		}
		if pc.GetRpcCount() > 0 {
			fmt.Fprintln(output, "FAIL: not caching data")
			failCount++
			return
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&memstats)
	midAlloc = memstats.Alloc

	// Wait for data to expire and someone to cleanup
	time.Sleep(20 * time.Second)

	// Run garbage collection and get memory stats
	runtime.GC()
	runtime.ReadMemStats(&memstats)
	finalAlloc = memstats.Alloc

	if finalAlloc < initAlloc || (finalAlloc-initAlloc) < 5000000 {
		fmt.Fprintln(output, "PASS")
		passCount++
	} else {
		fmt.Fprintf(output, "FAIL: not cleaning cache - memory leak - init %d mid %d final %d\n", initAlloc, midAlloc, finalAlloc)
		failCount++
	}
}

// Revoke valid lease for get
func testRevokeGetValid() {
	forceCacheGet("keyrevokeget:1", "value")
	err, status := revokeLease("keyrevokeget:1")
	if checkError(err, false) {
		return
	}
	if status != storageproto.OK {
		fmt.Fprintln(output, "FAIL: revoke should return OK on success")
		failCount++
		return
	}
	pc.Reset()
	v, err := ls.Get("keyrevokeget:1")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		fmt.Fprintln(output, "FAIL: not respecting lease revoke")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Revoke nonexistent lease for get
func testRevokeGetNonexistent() {
	ls.Put("keyrevokeget:2", "value")
	// Just shouldn't die or cause future issues
	revokeLease("keyrevokeget:2")
	pc.Reset()
	v, err := ls.Get("keyrevokeget:2")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		fmt.Fprintln(output, "FAIL: should not be cached")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Revoke lease update for get
func testRevokeGetUpdate() {
	forceCacheGet("keyrevokeget:3", "value")
	pc.Reset()
	forceCacheGet("keyrevokeget:3", "value2")
	if pc.GetRpcCount() <= 1 || pc.GetLeaseRequestCount() == 0 {
		fmt.Fprintln(output, "FAIL: not respecting lease revoke")
		failCount++
		return
	}
	pc.Reset()
	v, err := ls.Get("keyrevokeget:3")
	if checkError(err, false) {
		return
	}
	if v != "value2" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() > 0 {
		fmt.Fprintln(output, "FAIL: should be cached")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache < limit test for get list
func testCacheGetListLimit() {
	pc.Reset()
	ls.AppendToList("keycachegetlist:1", "value")
	for i := 0; i < storageproto.QUERY_CACHE_THRESH-1; i++ {
		ls.GetList("keycachegetlist:1")
	}
	if pc.GetLeaseRequestCount() > 0 {
		fmt.Fprintln(output, "FAIL: should not request lease")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache > limit test for get list
func testCacheGetListLimit2() {
	pc.Reset()
	forceCacheGetList("keycachegetlist:2", "value")
	if pc.GetLeaseRequestCount() == 0 {
		fmt.Fprintln(output, "FAIL: should have requested lease")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Doesn't call server when using cache for get list
func testCacheGetListCorrect() {
	forceCacheGetList("keycachegetlist:3", "value")
	pc.Reset()
	for i := 0; i < 100*storageproto.QUERY_CACHE_THRESH; i++ {
		v, err := ls.GetList("keycachegetlist:3")
		if checkError(err, false) {
			return
		}
		if len(v) != 1 || v[0] != "value" {
			fmt.Fprintln(output, "FAIL: got wrong value from cache")
			failCount++
			return
		}
	}
	if pc.GetRpcCount() > 0 {
		fmt.Fprintln(output, "FAIL: should not contact server when using cache")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache respects granted flag for get list
func testCacheGetListLeaseNotGranted() {
	pc.DisableLease()
	defer pc.EnableLease()
	forceCacheGetList("keycachegetlist:4", "value")
	pc.Reset()
	v, err := ls.GetList("keycachegetlist:4")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		fmt.Fprintln(output, "FAIL: not respecting lease granted flag")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache requests leases until granted for get list
func testCacheGetListLeaseNotGranted2() {
	pc.DisableLease()
	defer pc.EnableLease()
	forceCacheGetList("keycachegetlist:5", "value")
	pc.Reset()
	forceCacheGetList("keycachegetlist:5", "value")
	if pc.GetLeaseRequestCount() == 0 {
		fmt.Fprintln(output, "FAIL: not requesting leases after lease wasn't granted")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache respects lease timeout for get list
func testCacheGetListLeaseTimeout() {
	pc.OverrideLeaseSeconds(1)
	defer pc.OverrideLeaseSeconds(0)
	forceCacheGetList("keycachegetlist:6", "value")
	time.Sleep(2 * time.Second)
	pc.Reset()
	v, err := ls.GetList("keycachegetlist:6")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		fmt.Fprintln(output, "FAIL: not respecting lease timeout")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Cache memory leak for get list
func testCacheGetListMemoryLeak() {
	pc.OverrideLeaseSeconds(1)
	defer pc.OverrideLeaseSeconds(0)

	var memstats runtime.MemStats
	var initAlloc uint64
	var midAlloc uint64
	var finalAlloc uint64
	longValue := strings.Repeat("this sentence is 30 char long\n", 30)

	// Run garbage collection and get memory stats
	runtime.GC()
	runtime.ReadMemStats(&memstats)
	initAlloc = memstats.Alloc

	// Cache a lot of data
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("keymemleakgetlist:%d", i)
		pc.Reset()
		forceCacheGetList(key, longValue)
		if pc.GetLeaseRequestCount() == 0 {
			fmt.Fprintln(output, "FAIL: not requesting leases")
			failCount++
			return
		}
		pc.Reset()
		v, err := ls.GetList(key)
		if checkError(err, false) {
			return
		}
		if len(v) != 1 || v[0] != longValue {
			fmt.Fprintln(output, "FAIL: got wrong value")
			failCount++
			return
		}
		if pc.GetRpcCount() > 0 {
			fmt.Fprintln(output, "FAIL: not caching data")
			failCount++
			return
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&memstats)
	midAlloc = memstats.Alloc

	// Wait for data to expire and someone to cleanup
	time.Sleep(20 * time.Second)

	// Run garbage collection and get memory stats
	runtime.GC()
	runtime.ReadMemStats(&memstats)
	finalAlloc = memstats.Alloc

	if finalAlloc < initAlloc || (finalAlloc-initAlloc) < 5000000 {
		fmt.Fprintln(output, "PASS")
		passCount++
	} else {
		fmt.Fprintf(output, "FAIL: not cleaning cache - memory leak - init %d mid %d final %d\n", initAlloc, midAlloc, finalAlloc)
		failCount++
	}
}

// Revoke valid lease for get list
func testRevokeGetListValid() {
	forceCacheGetList("keyrevokegetlist:1", "value")
	err, status := revokeLease("keyrevokegetlist:1")
	if checkError(err, false) {
		return
	}
	if status != storageproto.OK {
		fmt.Fprintln(output, "FAIL: revoke should return OK on success")
		failCount++
		return
	}
	pc.Reset()
	v, err := ls.GetList("keyrevokegetlist:1")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		fmt.Fprintln(output, "FAIL: not respecting lease revoke")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Revoke nonexistent lease for get list
func testRevokeGetListNonexistent() {
	ls.AppendToList("keyrevokegetlist:2", "value")
	// Just shouldn't die or cause future issues
	revokeLease("keyrevokegetlist:2")
	pc.Reset()
	v, err := ls.GetList("keyrevokegetlist:2")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		fmt.Fprintln(output, "FAIL: should not be cached")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Revoke lease update for get list
func testRevokeGetListUpdate() {
	forceCacheGetList("keyrevokegetlist:3", "value")
	ls.RemoveFromList("keyrevokegetlist:3", "value")
	pc.Reset()
	forceCacheGetList("keyrevokegetlist:3", "value2")
	if pc.GetRpcCount() <= 1 || pc.GetLeaseRequestCount() == 0 {
		fmt.Fprintln(output, "FAIL: not respecting lease revoke")
		failCount++
		return
	}
	pc.Reset()
	v, err := ls.GetList("keyrevokegetlist:3")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value2" {
		fmt.Fprintln(output, "FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() > 0 {
		fmt.Fprintln(output, "FAIL: should be cached")
		failCount++
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

func main() {
	var err error
	output = os.Stderr
	passCount = 0
	failCount = 0
	initTests := []TestFunc{
		TestFunc{"testNonexistentServer", testNonexistentServer},
		TestFunc{"testNoLeases", testNoLeases},
		TestFunc{"testAlwaysLeases", testAlwaysLeases}}
	tests := []TestFunc{
		TestFunc{"testGetError", testGetError},
		TestFunc{"testGetErrorStatus", testGetErrorStatus},
		TestFunc{"testGetValid", testGetValid},
		TestFunc{"testPutError", testPutError},
		TestFunc{"testPutErrorStatus", testPutErrorStatus},
		TestFunc{"testPutValid", testPutValid},
		TestFunc{"testGetListError", testGetListError},
		TestFunc{"testGetListErrorStatus", testGetListErrorStatus},
		TestFunc{"testGetListValid", testGetListValid},
		TestFunc{"testAppendToListError", testAppendToListError},
		TestFunc{"testAppendToListErrorStatus", testAppendToListErrorStatus},
		TestFunc{"testAppendToListValid", testAppendToListValid},
		TestFunc{"testRemoveFromListError", testRemoveFromListError},
		TestFunc{"testRemoveFromListErrorStatus", testRemoveFromListErrorStatus},
		TestFunc{"testRemoveFromListValid", testRemoveFromListValid},
		TestFunc{"testCacheGetLimit", testCacheGetLimit},
		TestFunc{"testCacheGetLimit2", testCacheGetLimit2},
		TestFunc{"testCacheGetCorrect", testCacheGetCorrect},
		TestFunc{"testCacheGetLeaseNotGranted", testCacheGetLeaseNotGranted},
		TestFunc{"testCacheGetLeaseNotGranted2", testCacheGetLeaseNotGranted2},
		TestFunc{"testCacheGetLeaseTimeout", testCacheGetLeaseTimeout},
		TestFunc{"testCacheGetMemoryLeak", testCacheGetMemoryLeak},
		TestFunc{"testRevokeGetValid", testRevokeGetValid},
		TestFunc{"testRevokeGetNonexistent", testRevokeGetNonexistent},
		TestFunc{"testRevokeGetUpdate", testRevokeGetUpdate},
		TestFunc{"testCacheGetListLimit", testCacheGetListLimit},
		TestFunc{"testCacheGetListLimit2", testCacheGetListLimit2},
		TestFunc{"testCacheGetListCorrect", testCacheGetListCorrect},
		TestFunc{"testCacheGetListLeaseNotGranted", testCacheGetListLeaseNotGranted},
		TestFunc{"testCacheGetListLeaseNotGranted2", testCacheGetListLeaseNotGranted2},
		TestFunc{"testCacheGetListLeaseTimeout", testCacheGetListLeaseTimeout},
		TestFunc{"testCacheGetListMemoryLeak", testCacheGetListMemoryLeak},
		TestFunc{"testRevokeGetListValid", testRevokeGetListValid},
		TestFunc{"testRevokeGetListNonexistent", testRevokeGetListNonexistent},
		TestFunc{"testRevokeGetListUpdate", testRevokeGetListUpdate}}

	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal("usage:  libtest <storage master node>")
	}

	// Run init tests
	for _, t := range initTests {
		if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
			fmt.Println("Starting " + t.name + ":")
			t.f()
		}
	}

	l := initLibstore(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum), fmt.Sprintf("localhost:%d", *portnum), libstore.NONE)
	if l == nil {
		return
	}
	revokeConn, err = rpc.DialHTTP("tcp", fmt.Sprintf("localhost:%d", *portnum))
	if err != nil {
		fmt.Println("Failed to connect to cacherpc")
		return
	}

	// Run tests
	for _, t := range tests {
		if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
			fmt.Println("Starting " + t.name + ":")
			t.f()
		}
	}

	fmt.Printf("Passed (%d/%d) tests\n", passCount, passCount+failCount)
}
