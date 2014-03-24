package main

// Libclient offers the same command-line interface as storageclient,
// but it uses libstore instead.  You can use this program to test your
// libstore implementation or to query what's in the entire storage
// system.

import (
	"flag"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/libstore"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"
)

// For parsing the command line
type cmd_info struct {
	cmdline string
	nargs   int // number of required args
}

const (
	CMD_PUT = iota
	CMD_GET
)

var portnum *int = flag.Int("port", 9999, "master server port # to connect to")
var serverAddress *string = flag.String("host", "localhost", "server host to connect to")
var handleLeases *bool = flag.Bool("l", false, "Run persistently.  Request leases and report lease revocation requests.")
var forceLease *bool = flag.Bool("fl", false, "Force a lease request on every get / getlist.  Default is to allow libstore to make the decision.  Requires -l")
var numTimes *int = flag.Int("n", 1, "Number of times to execute the get or put.")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s  <command>:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "   commands:  p  key val   (put)\n")
		fmt.Fprintf(os.Stderr, "              g  key       (get)\n")
		fmt.Fprintf(os.Stderr, "              la key val   (list append)\n")
		fmt.Fprintf(os.Stderr, "              lr key val   (list remove)\n")
		fmt.Fprintf(os.Stderr, "              lg key       (list get)\n")
	}

	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("Insufficient arguments to client")
	}

	cmd := flag.Arg(0)

	cmdlist := []cmd_info{
		{"p", 2},
		{"g", 1},
		{"la", 2},
		{"lr", 2},
		{"lg", 1},
	}

	cmdmap := make(map[string]cmd_info)
	for _, j := range cmdlist {
		cmdmap[j.cmdline] = j
	}

	ci, found := cmdmap[cmd]
	if !found {
		log.Fatal("Unknown command ", cmd)
	}
	if flag.NArg() < (ci.nargs + 1) {
		log.Fatal("Insufficient arguments for ", cmd)
	}

	leaseCBaddr := ""
	flags := 0
	if *handleLeases {
		l, e := net.Listen("tcp", ":0")
		if e != nil {
			log.Fatal("listen error:", e)
		}
		_, listenport, _ := net.SplitHostPort(l.Addr().String())
		leaseCBaddr = net.JoinHostPort("localhost", listenport)
		log.Println("Starting HTTP server on ", listenport)
		rpc.HandleHTTP()
		go http.Serve(l, nil)
	}

	if *handleLeases && *forceLease {
		flags |= libstore.ALWAYS_LEASE
	}
	ls, err := libstore.NewLibstore(net.JoinHostPort(*serverAddress, fmt.Sprintf("%d", *portnum)), leaseCBaddr, flags)
	if err != nil {
		log.Fatal("Could not create a libstore")
	}

	for i := 0; i < *numTimes; i++ {
		switch cmd {
		case "g":
			val, err := ls.Get(flag.Arg(1))
			if err != nil {
				fmt.Println("error: ", err)
			} else {
				fmt.Println("  ", val)
			}
		case "lg":
			val, err := ls.GetList(flag.Arg(1))
			if err != nil {
				fmt.Println("error: ", err)
			} else {
				for _, i := range val {
					fmt.Printf("%s  ", i)
				}
				fmt.Printf("\n")
			}
		case "p", "la", "lr":
			var err error
			switch cmd {
			case "p":
				err = ls.Put(flag.Arg(1), flag.Arg(2))
			case "la":
				err = ls.AppendToList(flag.Arg(1), flag.Arg(2))
			case "lr":
				err = ls.RemoveFromList(flag.Arg(1), flag.Arg(2))
			}
			switch err {
			case nil:
				fmt.Println("OK")
			default:
				fmt.Println("Error: ", err)
			}
		}
	}
	if *handleLeases {
		fmt.Println("waiting 20 seconds for lease callbacks")
		time.Sleep(20 * time.Second)
	}
}
