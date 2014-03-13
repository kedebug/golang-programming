// Implementation of an echo server based on LSP

package main

import (
	"flag"
	"fmt"
	"os"
	// When you are ready to test your own implementation of LSP, change
	// the following to "../contrib/lsp"
	"../official/lsp"
)

/* $begin lsp-echoserver */
func runserver(srv *lsp.LspServer) {
	for {
		// Read from client
		id, payload := srv.Read()
		if payload == nil {
			fmt.Printf("Connection %d has died\n", id)
		} else {
			/* $end lsp-echoserver */
			s := string(payload)
			fmt.Printf("Connection %d.  Received '%s'\n", id, s)
			/* $begin lsp-echoserver */
			// Echo back to client
			srv.Write(id, payload)
		}
	}
}

/* $end lsp-echoserver */

func main() {
	var ihelp *bool = flag.Bool("h", false, "Print help information")
	var iport *int = flag.Int("p", 6666, "Port number")
	var iverb *int = flag.Int("v", 1, "Verbosity (0-6)")
	var idrop *float64 = flag.Float64("d", 0.0, "Specify packet drop rate")
	var elength *float64 = flag.Float64("e", 2.0, "Epoch duration (secs.)")
	flag.Parse()
	if *ihelp {
		flag.Usage()
		os.Exit(0)
	}
	var port int = *iport
	if flag.NArg() > 0 {
		nread, _ := fmt.Sscanf(flag.Arg(0), "%d", &port)
		if nread != 1 {
			flag.Usage()
			os.Exit(0)
		}
	}
	lsp.SetVerbose(*iverb)
	lsp.SetDropRate(float32(*idrop))
	lsp.SetEpochLength(float32(*elength))
	fmt.Printf("Establishing server on port %d\n", port)
	srv := lsp.NewLspServer(port)
	runserver(srv)
}
