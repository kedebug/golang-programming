package libstore

import (
	"github.com/kedebug/golang-programming/15-440/P2-F11/cache"
	"github.com/kedebug/golang-programming/15-440/P2-F11/lsplog"
	sp "github.com/kedebug/golang-programming/15-440/P2-F11/storageproto"
	"net/rpc"
	"sort"
	"strings"
)

type NodeList []sp.Node

type Libstore struct {
	nodes    NodeList
	rpcConn  []*rpc.Client
	hostport string
	flags    int
	leases   *cache.Cache
}

func (l NodeList) Len() int           { return len(l) }
func (l NodeList) Less(i, j int) bool { return l[i].NodeID < l[j].NodeID }
func (l NodeList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

func iNewLibstore(master, myhostport string, flags int) (*Libstore, error) {
	lsplog.SetVerbose(3)

	if myhostport == "" {
		panic("[Libstore] hostport shouldn't be null")
	}

	cli, err := rpc.DialHTTP("tcp", master)
	if err != nil {
		lsplog.Vlogf(1, "[Libstore] DialHTTP error: %v\n", err)
		return nil, err
	}
	defer cli.Close()

	var args sp.GetServersArgs
	var reply sp.RegisterReply

	for i := 0; i < 5; i++ {
		cli.Call("StorageRPC.GetServers", &args, &reply)
		if reply.Ready {
			break
		}
	}
	if !reply.Ready {
		return nil, lsplog.MakeErr("StorageServer not ready")
	}

	ls := new(Libstore)

	ls.hostport = myhostport
	ls.flags = flags
	ls.nodes = reply.Servers
	ls.leases = cache.NewCache()
	ls.rpcConn = make([]*rpc.Client, len(ls.nodes))

	sort.Sort(ls.nodes)

	return ls, nil
}

func (ls *Libstore) pickServer(key string) (*rpc.Client, error) {
	hash := StoreHash(strings.Split(key, ":")[0])
	id := 0
	for i, node := range ls.nodes {
		if node.NodeID >= hash {
			id = i
			break
		}
	}
	if ls.rpcConn[id] == nil {
		cli, err := rpc.DialHTTP("tcp", ls.nodes[id].HostPort)
		if err != nil {
			lsplog.Vlogf(2, "[Libstore] DialHTTP error: %v\n", err)
			return nil, err
		}
		ls.rpcConn[id] = cli
	}
	return ls.rpcConn[id], nil
}

func (ls *Libstore) iGet(key string) (string, error) {
	var args sp.GetArgs = sp.GetArgs{key, false, ls.hostport}
	var reply sp.GetReply

	if val, err := ls.leases.Get(key, &args); err == nil {
		return val.(string), nil
	}

	if (ls.flags & ALWAYS_LEASE) != 0 {
		args.WantLease = true
	}

	cli, err := ls.pickServer(key)
	if err != nil {
		return "", err
	}

	err = cli.Call("StorageRPC.Get", &args, &reply)
	if err != nil {
		lsplog.Vlogf(3, "[Libstore] rpc call error: %v\n", err)
		return "", err
	}

	if reply.Status != sp.OK {
		lsplog.Vlogf(3, "[Libstore] Get error, status=%d\n", reply.Status)
		return "", lsplog.MakeErr("Get error")
	}

	if reply.Lease.Granted {
		ls.leases.Grant(key, reply.Value, reply.Lease)
	}

	return reply.Value, nil
}

func (ls *Libstore) iPut(key, value string) error {
	cli, err := ls.pickServer(key)
	if err != nil {
		return err
	}

	var args sp.PutArgs
	var reply sp.PutReply

	args.Key, args.Value = key, value

	err = cli.Call("StorageRPC.Put", &args, &reply)
	if err != nil {
		lsplog.Vlogf(3, "[Libstore] rpc call error: %v\n", err)
		return err
	}

	if reply.Status != sp.OK {
		lsplog.Vlogf(3, "[Libstore] Put error, status=%d\n", reply.Status)
		return lsplog.MakeErr("Put error")
	}
	return nil
}

func (ls *Libstore) iGetList(key string) ([]string, error) {
	var args sp.GetArgs = sp.GetArgs{key, false, ls.hostport}
	var reply sp.GetListReply

	if val, err := ls.leases.Get(key, &args); err == nil {
		return val.([]string), nil
	}

	if (ls.flags & ALWAYS_LEASE) != 0 {
		args.WantLease = true
	}

	cli, err := ls.pickServer(key)
	if err != nil {
		return nil, err
	}

	err = cli.Call("StorageRPC.GetList", &args, &reply)
	if err != nil {
		lsplog.Vlogf(3, "[Libstore] rpc call error: %v\n", err)
		return nil, err
	}

	if reply.Status != sp.OK {
		lsplog.Vlogf(3, "[Libstore] GetList error, status=%d\n", reply.Status)
		return nil, lsplog.MakeErr("GetList error")
	}

	if reply.Lease.Granted {
		ls.leases.Grant(key, reply.Value, reply.Lease)
	}

	return reply.Value, nil
}

func (ls *Libstore) iRemoveFromList(key, removeitem string) error {
	cli, err := ls.pickServer(key)
	if err != nil {
		return err
	}

	var args sp.PutArgs
	var reply sp.PutReply

	args.Key, args.Value = key, removeitem

	err = cli.Call("StorageRPC.RemoveFromList", &args, &reply)
	if err != nil {
		lsplog.Vlogf(3, "[Libstore] rpc call error: %v\n", err)
		return err
	}

	if reply.Status != sp.OK {
		lsplog.Vlogf(3, "[Libstore] RemoveFromList error, status=%d\n", reply.Status)
		return lsplog.MakeErr("RemoveFromList error")
	}
	return nil
}

func (ls *Libstore) iAppendToList(key, newitem string) error {
	cli, err := ls.pickServer(key)
	if err != nil {
		return err
	}

	var args sp.PutArgs
	var reply sp.PutReply

	args.Key, args.Value = key, newitem

	err = cli.Call("StorageRPC.AppendToList", &args, &reply)
	if err != nil {
		lsplog.Vlogf(3, "[Libstore] rpc call error: %v\n", err)
		return err
	}

	if reply.Status != sp.OK {
		lsplog.Vlogf(3, "[Libstore] AppendToList error, status=%d\n", reply.Status)
		return lsplog.MakeErr("AppendToList error")
	}
	return nil
}

func (ls *Libstore) RevokeLease(args *sp.RevokeLeaseArgs, reply *sp.RevokeLeaseReply) error {
	ok := ls.leases.Revoke(args.Key)
	if ok {
		reply.Status = sp.OK
	} else {
		reply.Status = sp.EKEYNOTFOUND
	}
	return nil
}
