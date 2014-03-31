package viewservice

import "net"
import "net/rpc"
import "log"
import "time"
import "sync"
import "fmt"
import "os"

type ViewServer struct {
	mu   sync.Mutex
	l    net.Listener
	dead bool
	me   string

	// Your declarations here.
	view    View
	clients map[string]*Client

	acked            bool
	primaryRestarted bool
}

type Client struct {
	hostport    string
	dead        bool
	idle        bool
	lastViewnum uint
	lastPing    time.Time
}

//
// server Ping RPC handler.
//
func (vs *ViewServer) Ping(args *PingArgs, reply *PingReply) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	cli, ok := vs.clients[args.Me]
	if !ok {
		cli = new(Client)
		cli.hostport = args.Me
		cli.idle = true
		vs.clients[args.Me] = cli
	}

	if args.Me == vs.view.Primary && args.Viewnum == vs.view.Viewnum {
		vs.acked = true
	}

	if args.Me == vs.view.Primary && args.Viewnum == 0 && vs.view.Viewnum != 0 && vs.acked {
		vs.primaryRestarted = true
	}

	cli.dead = false
	cli.lastViewnum = args.Viewnum
	cli.lastPing = time.Now()

	reply.View = vs.view

	return nil
}

//
// server Get() RPC handler.
//
func (vs *ViewServer) Get(args *GetArgs, reply *GetReply) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	reply.View = vs.view

	return nil
}

//
// tick() is called once per PingInterval; it should notice
// if servers have died or recovered, and change the view
// accordingly.
//
func (vs *ViewServer) tick() {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	v := &vs.view

	var idle_client *Client
	var inited_idle_client *Client

	for _, cli := range vs.clients {
		dur := time.Since(cli.lastPing)
		if dur > PingInterval*DeadPings {
			cli.idle = true
			cli.dead = true
		}

		if !cli.dead && cli.idle && idle_client == nil {
			idle_client = cli
		}

		if !cli.dead && cli.idle &&
			cli.lastViewnum == v.Viewnum && inited_idle_client == nil {
			inited_idle_client = cli
		}
	}

	if vs.acked {
		if v.Primary != "" && vs.clients[v.Primary].dead {
			v.Primary = ""
		}

		if v.Backup != "" && vs.clients[v.Backup].dead {
			v.Backup = ""
		}

		if vs.primaryRestarted {
			vs.clients[v.Primary].idle = true
			v.Primary = ""
			vs.primaryRestarted = false
		}

		if v.Primary == "" && v.Backup == "" {
			if inited_idle_client != nil {
				v.Primary = inited_idle_client.hostport
				v.Viewnum++

				vs.acked = false
				inited_idle_client.idle = false
			}
		} else if v.Primary == "" {
			if vs.clients[v.Backup].lastViewnum == v.Viewnum {
				v.Primary = v.Backup
				v.Backup = ""
				v.Viewnum++

				vs.acked = false
			}
		} else if v.Backup == "" {
			if idle_client != nil {
				v.Backup = idle_client.hostport
				v.Viewnum++

				vs.acked = false
				idle_client.idle = false
			}
		}
	}
}

//
// tell the server to shut itself down.
// for testing.
// please don't change this function.
//
func (vs *ViewServer) Kill() {
	vs.dead = true
	vs.l.Close()
}

func StartServer(me string) *ViewServer {
	vs := new(ViewServer)
	vs.me = me
	// Your vs.* initializations here.
	vs.acked = true
	vs.clients = make(map[string]*Client)

	// tell net/rpc about our RPC server and handlers.
	rpcs := rpc.NewServer()
	rpcs.Register(vs)

	// prepare to receive connections from clients.
	// change "unix" to "tcp" to use over a network.
	os.Remove(vs.me) // only needed for "unix"
	l, e := net.Listen("unix", vs.me)
	if e != nil {
		log.Fatal("listen error: ", e)
	}
	vs.l = l

	// please don't change any of the following code,
	// or do anything to subvert it.

	// create a thread to accept RPC connections from clients.
	go func() {
		for vs.dead == false {
			conn, err := vs.l.Accept()
			if err == nil && vs.dead == false {
				go rpcs.ServeConn(conn)
			} else if err == nil {
				conn.Close()
			}
			if err != nil && vs.dead == false {
				fmt.Printf("ViewServer(%v) accept: %v\n", me, err.Error())
				vs.Kill()
			}
		}
	}()

	// create a thread to call tick() periodically.
	go func() {
		for vs.dead == false {
			vs.tick()
			time.Sleep(PingInterval)
		}
	}()

	return vs
}
