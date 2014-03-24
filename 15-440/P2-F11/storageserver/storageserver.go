package main

// Server driver - creates a storage server and registers it for RPC.
//
// DO NOT MODIFY THIS FILE FOR YOUR PROJECT

import (
	"flag"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/storageimpl"
	"github.com/kedebug/golang-programming/15-440/P2-F11/storagerpc"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
)

var portnum *int = flag.Int("port", 0, "port # to listen on.  Non-master nodes default to using an ephemeral port (0), master nodes default to 9009.")
var storageMasterNodePort *string = flag.String("master", "", "Specify the storage master node, making this node a slave.  Defaults to its own port (self-mastering).")
var numNodes *int = flag.Int("N", 0, "Become the master.  Specifies the number of nodes in the system, including the master.")
var nodeID *uint = flag.Uint("id", 0, "The node ID to use for consistent hashing.  Should be a 32 bit number.")

func main() {
	flag.Parse()
	if *storageMasterNodePort == "" {
		if *portnum == 0 {
			*portnum = 9009
		}
		// Single node execution
		*storageMasterNodePort = fmt.Sprintf("localhost:%d", *portnum)
		if *numNodes == 0 {
			*numNodes = 1
			log.Println("Self-mastering, setting nodes to 1")
		}
	}

	l, e := net.Listen("tcp", fmt.Sprintf(":%d", *portnum))
	if e != nil {
		log.Fatal("listen error:", e)
	}
	_, listenport, _ := net.SplitHostPort(l.Addr().String())
	log.Println("Server starting on ", listenport)
	*portnum, _ = strconv.Atoi(listenport)
	ss := storageimpl.NewStorageserver(*storageMasterNodePort, *numNodes, *portnum, uint32(*nodeID))
	srpc := storagerpc.NewStorageRPC(ss)
	rpc.Register(srpc)
	rpc.HandleHTTP()
	http.Serve(l, nil)
}
