package pbservice

import "github.com/kedebug/golang-programming/6.824-labs/viewservice"
import "net/rpc"

// You'll probably need to uncomment this:
// import "time"

type Clerk struct {
	vs   *viewservice.Clerk
	view viewservice.View
}

func MakeClerk(vshost string, me string) *Clerk {
	ck := new(Clerk)
	ck.vs = viewservice.MakeClerk(me, vshost)
	return ck
}

//
// call() sends an RPC to the rpcname handler on server srv
// with arguments args, waits for the reply, and leaves the
// reply in reply. the reply argument should be a pointer
// to a reply structure.
//
// the return value is true if the server responded, and false
// if call() was not able to contact the server. in particular,
// the reply's contents are only valid if call() returned true.
//
// you should assume that call() will time out and return an
// error after a while if it doesn't get a reply from the server.
//
// please use call() to send all RPCs, in client.go and server.go.
// please don't change this function.
//
func call(srv string, rpcname string,
	args interface{}, reply interface{}) bool {
	c, errx := rpc.Dial("unix", srv)
	if errx != nil {
		return false
	}
	defer c.Close()

	err := c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}
	return false
}

func (ck *Clerk) updateView() {
	if v, ok := ck.vs.Get(); ok {
		ck.view = v
	}
}

//
// fetch a key's value from the current primary;
// if they key has never been set, return "".
// Get() must keep trying until it either the
// primary replies with the value or the primary
// says the key doesn't exist (has never been Put().
//
func (ck *Clerk) Get(key string) string {
	for ck.view.Primary == "" {
		ck.updateView()
	}

	var args GetArgs = GetArgs{key}
	var reply GetReply

	ok := call(ck.view.Primary, "PBServer.Get", &args, &reply)
	if reply.Err == ErrNoKey {
		return string(reply.Err)
	}
	for !ok || reply.Err == ErrWrongServer {
		ck.updateView()
		ok = call(ck.view.Primary, "PBServer.Get", &args, &reply)
	}
	if reply.Err == OK {
		return reply.Value
	}

	return string(reply.Err)
}

//
// tell the primary to update key's value.
// must keep trying until it succeeds.
//
func (ck *Clerk) Put(key string, value string) {
	for ck.view.Primary == "" {
		ck.updateView()
	}

	var args PutArgs = PutArgs{key, value}
	var reply PutReply

	ok := call(ck.view.Primary, "PBServer.Put", &args, &reply)
	for !ok || reply.Err == ErrWrongServer {
		ck.updateView()
		ok = call(ck.view.Primary, "PBServer.Put", &args, &reply)
	}
}
