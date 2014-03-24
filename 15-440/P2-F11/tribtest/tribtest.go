package main

import (
	"flag"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/proxycounter"
	"github.com/kedebug/golang-programming/15-440/P2-F11/storagerpc"
	"github.com/kedebug/golang-programming/15-440/P2-F11/tribimpl"
	"github.com/kedebug/golang-programming/15-440/P2-F11/tribproto"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"regexp"
	"strings"
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
var ts *tribimpl.Tribserver

// Initialize proxy and tribserver
func initTribserver(storage string, server string, myhostport string) net.Listener {
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

	// Start tribserver
	ts = tribimpl.NewTribserver(server, myhostport)
	if ts == nil {
		fmt.Println("Could not start tribserver")
		return nil
	}
	rpc.Register(ts)
	return l
}

// Cleanup tribserver and rpc hooks
func cleanupTribserver(l net.Listener) {
	// Close listener to stop http serve thread
	if l != nil {
		l.Close()
	}
	// Recreate default http serve mux
	http.DefaultServeMux = http.NewServeMux()
	// Recreate default rpc server
	rpc.DefaultServer = rpc.NewServer()
	// Unset tribserver just in case
	ts = nil
}

// Test tribserver returns nil when it cannot connect to the server
func testNonexistentServer() {
	if tribimpl.NewTribserver(fmt.Sprintf("localhost:%d", *portnum), fmt.Sprintf("localhost:%d", *portnum)) == nil {
		fmt.Fprintln(output, "PASS")
		passCount++
	} else {
		fmt.Fprintln(output, "FAIL: tribserver does not return nil when it cannot connect to nonexistent storage server")
		failCount++
	}
	cleanupTribserver(nil)
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

// Check subscriptions
func checkSubscriptions(subs []string, expectedSubs []string) bool {
	if len(subs) != len(expectedSubs) {
		fmt.Fprintf(output, "FAIL: incorrect subscriptions %v, expected subscriptions %v\n", subs, expectedSubs)
		failCount++
		return true
	}
	m := make(map[string]bool)
	for _, s := range subs {
		m[s] = true
	}
	for _, s := range expectedSubs {
		if m[s] == false {
			fmt.Fprintf(output, "FAIL: incorrect subscriptions %v, expected subscriptions %v\n", subs, expectedSubs)
			failCount++
			return true
		}
	}
	return false
}

// Check tribbles
func checkTribbles(tribbles []tribproto.Tribble, expectedTribbles []tribproto.Tribble) bool {
	if len(tribbles) != len(expectedTribbles) {
		fmt.Fprintf(output, "FAIL: incorrect tribbles %v, expected tribbles %v\n", tribbles, expectedTribbles)
		failCount++
		return true
	}
	lastTime := int64(0)
	for i := len(tribbles) - 1; i >= 0; i-- {
		if tribbles[i].Userid != expectedTribbles[i].Userid {
			fmt.Fprintf(output, "FAIL: incorrect tribbles %v, expected tribbles %v\n", tribbles, expectedTribbles)
			failCount++
			return true
		}
		if tribbles[i].Contents != expectedTribbles[i].Contents {
			fmt.Fprintf(output, "FAIL: incorrect tribbles %v, expected tribbles %v\n", tribbles, expectedTribbles)
			failCount++
			return true
		}
		if tribbles[i].Posted.UnixNano() < lastTime {
			fmt.Fprintln(output, "FAIL: tribble timestamps not in reverse chronological order")
			failCount++
			return true
		}
		lastTime = tribbles[i].Posted.UnixNano()
	}
	return false
}

// Helper functions
func createUser(user string) (error, int) {
	args := &tribproto.CreateUserArgs{user}
	var reply tribproto.CreateUserReply
	err := ts.CreateUser(args, &reply)
	return err, reply.Status
}

func addSubscription(user string, target string) (error, int) {
	args := &tribproto.SubscriptionArgs{user, target}
	var reply tribproto.SubscriptionReply
	err := ts.AddSubscription(args, &reply)
	return err, reply.Status
}

func removeSubscription(user string, target string) (error, int) {
	args := &tribproto.SubscriptionArgs{user, target}
	var reply tribproto.SubscriptionReply
	err := ts.RemoveSubscription(args, &reply)
	return err, reply.Status
}

func getSubscription(user string) (error, int, []string) {
	args := &tribproto.GetSubscriptionsArgs{user}
	var reply tribproto.GetSubscriptionsReply
	err := ts.GetSubscriptions(args, &reply)
	return err, reply.Status, reply.Userids
}

func postTribble(user string, contents string) (error, int) {
	args := &tribproto.PostTribbleArgs{user, contents}
	var reply tribproto.PostTribbleReply
	err := ts.PostTribble(args, &reply)
	return err, reply.Status
}

func getTribbles(user string) (error, int, []tribproto.Tribble) {
	args := &tribproto.GetTribblesArgs{user}
	var reply tribproto.GetTribblesReply
	err := ts.GetTribbles(args, &reply)
	return err, reply.Status, reply.Tribbles
}

func getTribblesBySubscription(user string) (error, int, []tribproto.Tribble) {
	args := &tribproto.GetTribblesArgs{user}
	var reply tribproto.GetTribblesReply
	err := ts.GetTribblesBySubscription(args, &reply)
	return err, reply.Status, reply.Tribbles
}

// Create valid user
func testCreateUserValid() {
	pc.Reset()
	err, status := createUser("user")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Create duplicate user
func testCreateUserDuplicate() {
	createUser("user")
	pc.Reset()
	err, status := createUser("user")
	if checkErrorStatus(err, status, tribproto.EEXISTS) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Add subscription with invalid user
func testAddSubscriptionInvalidUser() {
	createUser("user")
	pc.Reset()
	err, status := addSubscription("invalidUser", "user")
	if checkErrorStatus(err, status, tribproto.ENOSUCHUSER) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Add subscription with invaild target user
func testAddSubscriptionInvalidTargetUser() {
	createUser("user")
	pc.Reset()
	err, status := addSubscription("user", "invalidUser")
	if checkErrorStatus(err, status, tribproto.ENOSUCHTARGETUSER) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Add valid subscription
func testAddSubscriptionValid() {
	createUser("user1")
	createUser("user2")
	pc.Reset()
	err, status := addSubscription("user1", "user2")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Add duplicate subscription
func testAddSubscriptionDuplicate() {
	createUser("user1")
	createUser("user2")
	addSubscription("user1", "user2")
	pc.Reset()
	err, status := addSubscription("user1", "user2")
	if checkErrorStatus(err, status, tribproto.EEXISTS) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Remove subscription with invalid user
func testRemoveSubscriptionInvalidUser() {
	createUser("user")
	pc.Reset()
	err, status := removeSubscription("invalidUser", "user")
	if checkErrorStatus(err, status, tribproto.ENOSUCHUSER) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Remove valid subscription
func testRemoveSubscriptionValid() {
	createUser("user1")
	createUser("user2")
	addSubscription("user1", "user2")
	pc.Reset()
	err, status := removeSubscription("user1", "user2")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Remove subscription with missing target user
func testRemoveSubscriptionMissingTarget() {
	createUser("user1")
	createUser("user2")
	removeSubscription("user1", "user2")
	pc.Reset()
	err, status := removeSubscription("user1", "user2")
	if checkErrorStatus(err, status, tribproto.ENOSUCHTARGETUSER) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get subscription with invalid user
func testGetSubscriptionInvalidUser() {
	pc.Reset()
	err, status, _ := getSubscription("invalidUser")
	if checkErrorStatus(err, status, tribproto.ENOSUCHUSER) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get valid subscription
func testGetSubscriptionValid() {
	createUser("user1")
	createUser("user2")
	createUser("user3")
	createUser("user4")
	addSubscription("user1", "user2")
	addSubscription("user1", "user3")
	addSubscription("user1", "user4")
	pc.Reset()
	err, status, subs := getSubscription("user1")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkSubscriptions(subs, []string{"user2", "user3", "user4"}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Post tribble with invalid user
func testPostTribbleInvalidUser() {
	pc.Reset()
	err, status := postTribble("invalidUser", "contents")
	if checkErrorStatus(err, status, tribproto.ENOSUCHUSER) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Post valid tribble
func testPostTribbleValid() {
	createUser("user")
	pc.Reset()
	err, status := postTribble("user", "contents")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles invalid user
func testGetTribblesInvalidUser() {
	pc.Reset()
	err, status, _ := getTribbles("invalidUser")
	if checkErrorStatus(err, status, tribproto.ENOSUCHUSER) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles 0 tribbles
func testGetTribblesZeroTribbles() {
	createUser("tribUser")
	pc.Reset()
	err, status, tribbles := getTribbles("tribUser")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, []tribproto.Tribble{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles < 100 tribbles
func testGetTribblesFewTribbles() {
	createUser("tribUser")
	expectedTribbles := []tribproto.Tribble{}
	for i := 0; i < 5; i++ {
		expectedTribbles = append(expectedTribbles, tribproto.Tribble{Userid: "tribUser", Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].Userid, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribbles("tribUser")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(50, 5000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles > 100 tribbles
func testGetTribblesManyTribbles() {
	createUser("tribUser")
	postTribble("tribUser", "should not see this old msg")
	expectedTribbles := []tribproto.Tribble{}
	for i := 0; i < 100; i++ {
		expectedTribbles = append(expectedTribbles, tribproto.Tribble{Userid: "tribUser", Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].Userid, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribbles("tribUser")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles by subscription invalid user
func testGetTribblesBySubscriptionInvalidUser() {
	pc.Reset()
	err, status, _ := getTribblesBySubscription("invalidUser")
	if checkErrorStatus(err, status, tribproto.ENOSUCHUSER) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles by subscription no subscriptions
func testGetTribblesBySubscriptionNoSubscriptions() {
	createUser("tribUser")
	postTribble("tribUser", "contents")
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, []tribproto.Tribble{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles by subscription 0 tribbles
func testGetTribblesBySubscriptionZeroTribbles() {
	createUser("tribUser1")
	createUser("tribUser2")
	addSubscription("tribUser1", "tribUser2")
	pc.Reset()
	err, status, tribbles := getTribbles("tribUser1")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, []tribproto.Tribble{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles by subscription < 100 tribbles
func testGetTribblesBySubscriptionFewTribbles() {
	createUser("tribUser1")
	createUser("tribUser2")
	createUser("tribUser3")
	createUser("tribUser4")
	addSubscription("tribUser1", "tribUser2")
	addSubscription("tribUser1", "tribUser3")
	addSubscription("tribUser1", "tribUser4")
	postTribble("tribUser1", "should not see this unsubscribed msg")
	expectedTribbles := []tribproto.Tribble{tribproto.Tribble{Userid: "tribUser2", Contents: "contents"}, tribproto.Tribble{Userid: "tribUser4", Contents: "contents"}}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].Userid, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser1")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(20, 2000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles by subscription > 100 tribbles
func testGetTribblesBySubscriptionManyTribbles() {
	createUser("tribUser1")
	createUser("tribUser2")
	createUser("tribUser3")
	createUser("tribUser4")
	addSubscription("tribUser1", "tribUser2")
	addSubscription("tribUser1", "tribUser3")
	addSubscription("tribUser1", "tribUser4")
	postTribble("tribUser1", "should not see this old msg")
	postTribble("tribUser2", "should not see this old msg")
	postTribble("tribUser3", "should not see this old msg")
	postTribble("tribUser4", "should not see this old msg")
	expectedTribbles := []tribproto.Tribble{}
	for i := 0; i < 100; i++ {
		expectedTribbles = append(expectedTribbles, tribproto.Tribble{Userid: fmt.Sprintf("tribUser%d", (i%3)+2), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].Userid, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser1")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles by subscription all recent tribbles by one subscription
func testGetTribblesBySubscriptionManyTribbles2() {
	createUser("tribUser1b")
	createUser("tribUser2b")
	createUser("tribUser3b")
	createUser("tribUser4b")
	addSubscription("tribUser1b", "tribUser2b")
	addSubscription("tribUser1b", "tribUser3b")
	addSubscription("tribUser1b", "tribUser4b")
	postTribble("tribUser1b", "should not see this old msg")
	postTribble("tribUser2b", "should not see this old msg")
	postTribble("tribUser3b", "should not see this old msg")
	postTribble("tribUser4b", "should not see this old msg")
	expectedTribbles := []tribproto.Tribble{}
	for i := 0; i < 100; i++ {
		expectedTribbles = append(expectedTribbles, tribproto.Tribble{Userid: fmt.Sprintf("tribUser3b"), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].Userid, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser1b")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

// Get tribbles by subscription test not performing too many RPCs or transferring too much data
func testGetTribblesBySubscriptionManyTribbles3() {
	createUser("tribUser1c")
	createUser("tribUser2c")
	createUser("tribUser3c")
	createUser("tribUser4c")
	createUser("tribUser5c")
	createUser("tribUser6c")
	createUser("tribUser7c")
	createUser("tribUser8c")
	createUser("tribUser9c")
	addSubscription("tribUser1c", "tribUser2c")
	addSubscription("tribUser1c", "tribUser3c")
	addSubscription("tribUser1c", "tribUser4c")
	addSubscription("tribUser1c", "tribUser5c")
	addSubscription("tribUser1c", "tribUser6c")
	addSubscription("tribUser1c", "tribUser7c")
	addSubscription("tribUser1c", "tribUser8c")
	addSubscription("tribUser1c", "tribUser9c")
	postTribble("tribUser1c", "should not see this old msg")
	postTribble("tribUser2c", "should not see this old msg")
	postTribble("tribUser3c", "should not see this old msg")
	postTribble("tribUser4c", "should not see this old msg")
	postTribble("tribUser5c", "should not see this old msg")
	postTribble("tribUser6c", "should not see this old msg")
	postTribble("tribUser7c", "should not see this old msg")
	postTribble("tribUser8c", "should not see this old msg")
	postTribble("tribUser9c", "should not see this old msg")
	longContents := strings.Repeat("this sentence is 30 char long\n", 30)
	for i := 0; i < 100; i++ {
		for j := 1; j <= 9; j++ {
			postTribble(fmt.Sprintf("tribUser%dc", j), longContents)
		}
	}
	expectedTribbles := []tribproto.Tribble{}
	for i := 0; i < 100; i++ {
		expectedTribbles = append(expectedTribbles, tribproto.Tribble{Userid: fmt.Sprintf("tribUser%dc", (i%8)+2), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].Userid, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser1c")
	if checkErrorStatus(err, status, tribproto.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(200, 200000) {
		return
	}
	fmt.Fprintln(output, "PASS")
	passCount++
}

func main() {
	output = os.Stderr
	passCount = 0
	failCount = 0
	initTests := []TestFunc{
		TestFunc{"testNonexistentServer", testNonexistentServer}}
	tests := []TestFunc{
		TestFunc{"testCreateUserValid", testCreateUserValid},
		TestFunc{"testCreateUserDuplicate", testCreateUserDuplicate},
		TestFunc{"testAddSubscriptionInvalidUser", testAddSubscriptionInvalidUser},
		TestFunc{"testAddSubscriptionInvalidTargetUser", testAddSubscriptionInvalidTargetUser},
		TestFunc{"testAddSubscriptionValid", testAddSubscriptionValid},
		TestFunc{"testAddSubscriptionDuplicate", testAddSubscriptionDuplicate},
		TestFunc{"testRemoveSubscriptionInvalidUser", testRemoveSubscriptionInvalidUser},
		TestFunc{"testRemoveSubscriptionValid", testRemoveSubscriptionValid},
		TestFunc{"testRemoveSubscriptionMissingTarget", testRemoveSubscriptionMissingTarget},
		TestFunc{"testGetSubscriptionInvalidUser", testGetSubscriptionInvalidUser},
		TestFunc{"testGetSubscriptionValid", testGetSubscriptionValid},
		TestFunc{"testPostTribbleInvalidUser", testPostTribbleInvalidUser},
		TestFunc{"testPostTribbleValid", testPostTribbleValid},
		TestFunc{"testGetTribblesInvalidUser", testGetTribblesInvalidUser},
		TestFunc{"testGetTribblesZeroTribbles", testGetTribblesZeroTribbles},
		TestFunc{"testGetTribblesFewTribbles", testGetTribblesFewTribbles},
		TestFunc{"testGetTribblesManyTribbles", testGetTribblesManyTribbles},
		TestFunc{"testGetTribblesBySubscriptionInvalidUser", testGetTribblesBySubscriptionInvalidUser},
		TestFunc{"testGetTribblesBySubscriptionNoSubscriptions", testGetTribblesBySubscriptionNoSubscriptions},
		TestFunc{"testGetTribblesBySubscriptionZeroTribbles", testGetTribblesBySubscriptionZeroTribbles},
		TestFunc{"testGetTribblesBySubscriptionZeroTribbles", testGetTribblesBySubscriptionZeroTribbles},
		TestFunc{"testGetTribblesBySubscriptionFewTribbles", testGetTribblesBySubscriptionFewTribbles},
		TestFunc{"testGetTribblesBySubscriptionManyTribbles", testGetTribblesBySubscriptionManyTribbles},
		TestFunc{"testGetTribblesBySubscriptionManyTribbles2", testGetTribblesBySubscriptionManyTribbles2},
		TestFunc{"testGetTribblesBySubscriptionManyTribbles3", testGetTribblesBySubscriptionManyTribbles3}}

	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal("usage:  tribtest <storage master node>")
	}

	// Run init tests
	for _, t := range initTests {
		if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
			fmt.Println("Starting " + t.name + ":")
			t.f()
		}
	}
	fmt.Println(flag.Arg(0))

	l := initTribserver(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum), fmt.Sprintf("localhost:%d", *portnum))
	if l == nil {
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
