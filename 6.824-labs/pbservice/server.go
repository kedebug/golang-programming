package pbservice

import "net"
import "fmt"
import "github.com/kedebug/golang-programming/6.824-labs/viewservice"
import "net/rpc"
import "log"
import "time"
import "sync"
import "os"
import "syscall"
import "math/rand"

type PBServer struct {
	mu         sync.Mutex
	l          net.Listener
	dead       bool // for testing
	unreliable bool // for testing
	me         string
	store      map[string]string
	vs         *viewservice.Clerk
	view       viewservice.View
}

func (pb *PBServer) Get(args *GetArgs, reply *GetReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	var fargs ForwardArgs
	var freply ForwardReply

	fargs.Op = OP_GET
	fargs.Key = args.Key

	if pb.view.Primary == pb.me && pb.view.Backup != "" {
		ok := call(pb.view.Backup, "PBServer.Forward", &fargs, &freply)
		if !ok || freply.Err != OK {
			reply.Err = ErrWrongServer
			return nil
		}
	}

	value, ok := pb.store[args.Key]
	if !ok {
		reply.Err = ErrNoKey
		return nil
	}

	reply.Err = OK
	reply.Value = value

	return nil
}

func (pb *PBServer) Put(args *PutArgs, reply *PutReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	var fargs ForwardArgs
	var freply ForwardReply

	fargs.Op = OP_PUT
	fargs.Key = args.Key
	fargs.Value = args.Value

	if pb.view.Primary == pb.me && pb.view.Backup != "" {
		ok := call(pb.view.Backup, "PBServer.Forward", &fargs, &freply)
		if !ok || freply.Err != OK {
			reply.Err = ErrWrongServer
			return nil
		}
	}

	reply.Err = OK
	pb.store[args.Key] = args.Value

	return nil
}

func (pb *PBServer) Forward(args *ForwardArgs, reply *ForwardReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.view.Backup != pb.me {
		reply.Err = ErrWrongServer
		return nil
	}

	if args.Op == OP_GET {
		reply.Err = OK
	} else if args.Op == OP_PUT {
		reply.Err = OK
		pb.store[args.Key] = args.Value
	}

	return nil
}

func (pb *PBServer) Sync(args *SyncArgs, reply *SyncReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	v, _ := pb.vs.Ping(pb.view.Viewnum)
	if v.Viewnum != pb.view.Viewnum {
		pb.view = v
	}

	if pb.view.Backup != pb.me {
		reply.Err = ErrWrongServer
		return nil
	}

	reply.Err = OK
	pb.store = args.Store

	return nil
}

//
// ping the viewserver periodically.
// if view changed:
//   transition to new view.
//   manage transfer of state from primary to new backup.
//
func (pb *PBServer) tick() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	v, err := pb.vs.Ping(pb.view.Viewnum)
	if err != nil {
		log.Println("Ping error:", err)
		return
	}

	if v.Viewnum == pb.view.Viewnum {
		return
	}
	pb.view = v

	log.Println("viewnum:", pb.view.Viewnum)
	log.Println("from:", pb.me)
	log.Println("primary:", pb.view.Primary)
	log.Println("backup:", pb.view.Backup)

	if pb.view.Primary == pb.me && pb.view.Backup != "" {
		args := &SyncArgs{pb.store}
		reply := &SyncReply{}
		ok := call(pb.view.Backup, "PBServer.Sync", args, reply)
		for !ok {
			log.Println(pb.view.Backup, "crashed")
			pb.view, _ = pb.vs.Ping(pb.view.Viewnum)
			if pb.view.Backup != "" {
				ok = call(pb.view.Backup, "PBServer.Sync", args, reply)
			} else {
				ok = true
			}
		}
		if reply.Err != OK {
			log.Println("Sync error:", reply.Err)
		}
	}
}

// tell the server to shut itself down.
// please do not change this function.
func (pb *PBServer) kill() {
	pb.dead = true
	pb.l.Close()
}

func StartServer(vshost string, me string) *PBServer {
	pb := new(PBServer)
	pb.me = me
	pb.vs = viewservice.MakeClerk(me, vshost)
	// Your pb.* initializations here.
	pb.store = make(map[string]string)

	rpcs := rpc.NewServer()
	rpcs.Register(pb)

	os.Remove(pb.me)
	l, e := net.Listen("unix", pb.me)
	if e != nil {
		log.Fatal("listen error: ", e)
	}
	pb.l = l

	// please do not change any of the following code,
	// or do anything to subvert it.

	go func() {
		for pb.dead == false {
			conn, err := pb.l.Accept()
			if err == nil && pb.dead == false {
				if pb.unreliable && (rand.Int63()%1000) < 100 {
					// discard the request.
					conn.Close()
				} else if pb.unreliable && (rand.Int63()%1000) < 200 {
					// process the request but force discard of reply.
					c1 := conn.(*net.UnixConn)
					f, _ := c1.File()
					err := syscall.Shutdown(int(f.Fd()), syscall.SHUT_WR)
					if err != nil {
						fmt.Printf("shutdown: %v\n", err)
					}
					go rpcs.ServeConn(conn)
				} else {
					go rpcs.ServeConn(conn)
				}
			} else if err == nil {
				conn.Close()
			}
			if err != nil && pb.dead == false {
				fmt.Printf("PBServer(%v) accept: %v\n", me, err.Error())
				pb.kill()
			}
		}
	}()

	go func() {
		for pb.dead == false {
			pb.tick()
			time.Sleep(viewservice.PingInterval)
		}
	}()

	return pb
}
