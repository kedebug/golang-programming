package storageimpl

import (
	"encoding/json"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/lsplog"
	sp "github.com/kedebug/golang-programming/15-440/P2-F11/storageproto"
	"math/rand"
	"net/rpc"
	"sync"
	"time"
)

type StorageServer struct {
	nodeid   uint32
	ismaster bool
	numnodes int
	port     int

	mu        sync.RWMutex
	nodes     map[sp.Node]bool
	hashmap   map[string][]byte
	leasePool map[string]*leaseEntry
}

type holder struct {
	address   string
	issueTime time.Time
}

type leaseEntry struct {
	mu      sync.Mutex
	pending bool
	holders []holder
}

func NewStorageserver(master string, numnodes int, port int, nodeid uint32) *StorageServer {
	lsplog.SetVerbose(3)

	if nodeid == 0 {
		rand.Seed(time.Now().UnixNano())
		nodeid = rand.Uint32()
	}

	ss := new(StorageServer)
	ss.nodeid = nodeid
	ss.port = port
	ss.hashmap = make(map[string][]byte)
	ss.leasePool = make(map[string]*leaseEntry)

	addr := fmt.Sprintf("localhost:%d", port)
	if master == addr {
		lsplog.Vlogf(1, "[StorageServer] create master node: %v\n", master)

		ss.ismaster = true
		ss.numnodes = numnodes
		ss.nodes = make(map[sp.Node]bool)

		node := sp.Node{master, nodeid}
		ss.nodes[node] = true
	} else {
		lsplog.Vlogf(1, "[StorageServer] create non-master node: %v\n", addr)

		ss.ismaster = false

		cli, err := rpc.DialHTTP("tcp", master)
		if err != nil {
			lsplog.Vlogf(1, "[StorageServer] create node failed: %v\n", err)
			return nil
		}

		var args sp.RegisterArgs
		var reply sp.RegisterReply

		args.ServerInfo.HostPort = fmt.Sprintf("localhost:%d", port)
		args.ServerInfo.NodeID = nodeid
		for i := 0; i < 10; i++ {
			cli.Call("StorageRPC.Register", &args, &reply)
			if reply.Ready == true {
				break
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}
	return ss
}

func (ss *StorageServer) addLeasePool(args *sp.GetArgs, lease *sp.LeaseStruct) {
	lsplog.Vlogf(3, "[StorageServer] addLeasePool: %v\n", args.Key)

	ss.mu.RLock()
	e, ok := ss.leasePool[args.Key]
	ss.mu.RUnlock()

	lease.Granted = true
	lease.ValidSeconds = sp.LEASE_SECONDS

	if ok {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.pending {
			lease.Granted = false
			lease.ValidSeconds = 0
			return
		}
		for _, h := range e.holders {
			if h.address == args.LeaseClient {
				h.issueTime = time.Now()
				return
			}
		}
	} else {
		e = &leaseEntry{}

		ss.mu.Lock()
		ss.leasePool[args.Key] = e
		ss.mu.Unlock()

		e.mu.Lock()
		defer e.mu.Unlock()
	}
	h := holder{args.LeaseClient, time.Now()}
	e.holders = append(e.holders, h)
}

func (ss *StorageServer) revokeLease(key string) {
	lsplog.Vlogf(3, "[StorageServer] revokeLease: %v\n", key)

	ss.mu.Lock()
	e, ok := ss.leasePool[key]
	ss.mu.Unlock()
	if !ok {
		return
	}

	e.mu.Lock()
	e.pending = true
	e.mu.Unlock()

	for _, h := range e.holders {
		dur := time.Since(h.issueTime).Seconds()
		if dur > sp.LEASE_SECONDS+sp.LEASE_GUARD_SECONDS {
			// timeout
			continue
		}

		cli, err := rpc.DialHTTP("tcp", h.address)
		defer cli.Close()
		if err != nil {
			lsplog.Vlogf(2, "[StorageServer] DialHTTP failed: %v\n", err)
			return
		}

		var args sp.RevokeLeaseArgs
		var reply sp.RevokeLeaseReply
		done := make(chan error)

		go func() {
			args.Key = key
			cli.Call("CacheRPC.RevokeLease", &args, &reply)
			done <- nil
		}()

		select {
		case <-done:
		case <-time.After((sp.LEASE_SECONDS + sp.LEASE_GUARD_SECONDS) * time.Second):
		}
	}

	e.mu.Lock()
	e.pending = false
	e.mu.Unlock()
}

func (ss *StorageServer) Get(args *sp.GetArgs, reply *sp.GetReply) error {
	ss.mu.RLock()
	value, present := ss.hashmap[args.Key]
	ss.mu.RUnlock()

	if !present {
		lsplog.Vlogf(3, "[StorageServer] Get key: %s nonexist\n", args.Key)
		reply.Status = sp.EWRONGSERVER
		return nil
	}

	json.Unmarshal(value, &reply.Value)
	if args.WantLease {
		ss.addLeasePool(args, &reply.Lease)
	}
	reply.Status = sp.OK

	lsplog.Vlogf(3, "[StorageServer] Get, key=%v, value=%v\n", args.Key, reply.Value)

	return nil
}

func (ss *StorageServer) GetList(args *sp.GetArgs, reply *sp.GetListReply) error {
	ss.mu.RLock()
	value, present := ss.hashmap[args.Key]
	ss.mu.RUnlock()

	if !present {
		lsplog.Vlogf(3, "[StorageServer] GetList key: %s nonexist\n", args.Key)
		reply.Status = sp.EKEYNOTFOUND
		return nil
	}

	json.Unmarshal(value, &(reply.Value))
	if args.WantLease {
		ss.addLeasePool(args, &reply.Lease)
	}
	reply.Status = sp.OK
	return nil
}

func (ss *StorageServer) Put(args *sp.PutArgs, reply *sp.PutReply) error {
	ss.revokeLease(args.Key)

	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.hashmap[args.Key], _ = json.Marshal(args.Value)
	reply.Status = sp.OK

	lsplog.Vlogf(3, "[StorageServer] Put, key=%v, value=%v\n", args.Key, args.Value)
	return nil
}

func (ss *StorageServer) AppendToList(args *sp.PutArgs, reply *sp.PutReply) error {
	ss.revokeLease(args.Key)

	ss.mu.RLock()
	bytes, _ := ss.hashmap[args.Key]
	ss.mu.RUnlock()

	var l []string
	json.Unmarshal(bytes, &l)

	for _, v := range l {
		if v == args.Value {
			reply.Status = sp.EITEMEXISTS
			return nil
		}
	}

	l = append(l, args.Value)
	ss.mu.Lock()
	ss.hashmap[args.Key], _ = json.Marshal(l)
	ss.mu.Unlock()

	reply.Status = sp.OK
	return nil
}

func (ss *StorageServer) RemoveFromList(args *sp.PutArgs, reply *sp.PutReply) error {
	ss.mu.RLock()
	bytes, present := ss.hashmap[args.Key]
	ss.mu.RUnlock()
	if !present {
		lsplog.Vlogf(3, "[StorageServer] key: %v, nonexist\n", args.Key)
		reply.Status = sp.EKEYNOTFOUND
		return nil
	}

	ss.revokeLease(args.Key)

	var l []string
	json.Unmarshal(bytes, &l)
	for i, v := range l {
		if v == args.Value {
			l = append(l[:i], l[i+1:]...)
			reply.Status = sp.OK
			ss.mu.Lock()
			ss.hashmap[args.Key], _ = json.Marshal(l)
			ss.mu.Unlock()
			return nil
		}
	}

	reply.Status = sp.EITEMNOTFOUND
	return nil
}

func (ss *StorageServer) RegisterServer(
	args *sp.RegisterArgs, reply *sp.RegisterReply) error {

	lsplog.Vlogf(3, "[StorageServer] RegisterServer invoked\n")

	if ss.ismaster == false {
		lsplog.Vlogf(2, "[StorageServer] calling non-master node for register\n")
		return lsplog.MakeErr("calling non-master node for register")
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.nodes[args.ServerInfo]; !ok {
		ss.nodes[args.ServerInfo] = true
	}

	reply.Ready = false
	if ss.numnodes == len(ss.nodes) {
		reply.Ready = true
		for node, _ := range ss.nodes {
			reply.Servers = append(reply.Servers, node)
		}
	}
	return nil
}

func (ss *StorageServer) GetServers(args *sp.GetServersArgs, reply *sp.RegisterReply) error {
	lsplog.Vlogf(3, "[StorageServer] GetServers invoked\n")

	if ss.ismaster == false {
		lsplog.Vlogf(2, "[StorageServer] calling non-master node for GetServers\n")
		return lsplog.MakeErr("calling non-master node for GetServers")
	}

	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if ss.numnodes != len(ss.nodes) {
		reply.Ready = false
		return nil
	}

	for node, _ := range ss.nodes {
		reply.Servers = append(reply.Servers, node)
	}
	reply.Ready = true
	return nil
}
