package proxycounter

import (
	"errors"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/storageproto"
	"net/rpc"
	"sync/atomic"
)

type ProxyCounter struct {
	srv *rpc.Client

	myhostport string
	rpcCount   uint32
	byteCount  uint32

	leaseRequestCount uint32
	leaseGrantedCount uint32

	override             bool
	overrideErr          error
	overrideStatus       int
	disableLease         bool
	overrideLeaseSeconds int
}

func NewProxyCounter(server string, myhostport string) *ProxyCounter {
	pc := &ProxyCounter{}
	pc.myhostport = myhostport
	// Create RPC connection to storage server
	srv, err := rpc.DialHTTP("tcp", server)
	if err != nil {
		fmt.Printf("Could not connect to server %s, returning nil\n", server)
		return nil
	}
	pc.srv = srv
	return pc
}

func (pc *ProxyCounter) Reset() {
	pc.rpcCount = 0
	pc.byteCount = 0
	pc.leaseRequestCount = 0
	pc.leaseGrantedCount = 0
}

func (pc *ProxyCounter) OverrideLeaseSeconds(leaseSeconds int) {
	pc.overrideLeaseSeconds = leaseSeconds
}

func (pc *ProxyCounter) DisableLease() {
	pc.disableLease = true
}

func (pc *ProxyCounter) EnableLease() {
	pc.disableLease = false
}

func (pc *ProxyCounter) OverrideErr() {
	pc.overrideErr = errors.New("error")
	pc.override = true
}

func (pc *ProxyCounter) OverrideStatus(status int) {
	pc.overrideStatus = status
	pc.override = true
}

func (pc *ProxyCounter) OverrideOff() {
	pc.override = false
	pc.overrideErr = nil
	pc.overrideStatus = storageproto.OK
}

func (pc *ProxyCounter) GetRpcCount() uint32 {
	return pc.rpcCount
}

func (pc *ProxyCounter) GetByteCount() uint32 {
	return pc.byteCount
}

func (pc *ProxyCounter) GetLeaseRequestCount() uint32 {
	return pc.leaseRequestCount
}

func (pc *ProxyCounter) GetLeaseGrantedCount() uint32 {
	return pc.leaseGrantedCount
}

// RPC-able interfaces, bridged via StorageRPC.
// These should do something! :-)
func (pc *ProxyCounter) GetServers(args *storageproto.GetServersArgs, reply *storageproto.RegisterReply) error {
	err := pc.srv.Call("StorageRPC.GetServers", args, reply)
	// Modify reply so node point to myself
	if len(reply.Servers) > 1 {
		panic("ProxyCounter only works with 1 storage node")
	} else if len(reply.Servers) == 1 {
		reply.Servers[0].HostPort = pc.myhostport
	}
	return err
}

func (pc *ProxyCounter) RegisterServer(args *storageproto.RegisterArgs, reply *storageproto.RegisterReply) error {
	return nil
}

func (pc *ProxyCounter) Get(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key)
	if args.WantLease {
		atomic.AddUint32(&pc.leaseRequestCount, 1)
	}
	if pc.disableLease {
		args.WantLease = false
	}
	err := pc.srv.Call("StorageRPC.Get", args, reply)
	byteCount += len(reply.Value)
	if reply.Lease.Granted {
		if pc.overrideLeaseSeconds > 0 {
			reply.Lease.ValidSeconds = pc.overrideLeaseSeconds
		}
		atomic.AddUint32(&pc.leaseGrantedCount, 1)
	}
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}

func (pc *ProxyCounter) GetList(args *storageproto.GetArgs, reply *storageproto.GetListReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key)
	if args.WantLease {
		atomic.AddUint32(&pc.leaseRequestCount, 1)
	}
	if pc.disableLease {
		args.WantLease = false
	}
	err := pc.srv.Call("StorageRPC.GetList", args, reply)
	for _, s := range reply.Value {
		byteCount += len(s)
	}
	if reply.Lease.Granted {
		if pc.overrideLeaseSeconds > 0 {
			reply.Lease.ValidSeconds = pc.overrideLeaseSeconds
		}
		atomic.AddUint32(&pc.leaseGrantedCount, 1)
	}
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}

func (pc *ProxyCounter) Put(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key) + len(args.Value)
	err := pc.srv.Call("StorageRPC.Put", args, reply)
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}

func (pc *ProxyCounter) AppendToList(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key) + len(args.Value)
	err := pc.srv.Call("StorageRPC.AppendToList", args, reply)
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}

func (pc *ProxyCounter) RemoveFromList(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key) + len(args.Value)
	err := pc.srv.Call("StorageRPC.RemoveFromList", args, reply)
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}
