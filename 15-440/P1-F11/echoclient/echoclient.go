// Implementation of an echo client based on LSP

package main

import (
	"fmt"
	"os"
	"flag"
	"strings"
	// When you are ready to test your own implementation of LSP, change
	// the following to "../contrib/lsp"
	"../official/lsp"
        )

/* $begin lsp-echoclient */
func runclient(cli *lsp.LspClient) {
	for {
		var s string
		// Get next token from input
		n, _ := fmt.Scan(&s)
		if (n <= 0) {
			cli.Close()
			return
		}
		// Send to server
		cli.Write([]byte(s))
		// Read from server
		payload := cli.Read()
		if payload == nil {
			fmt.Printf("Lost contact with server\n")
			return
		}
		fmt.Printf("[%s]\n", string(payload))
	}
}
/* $end lsp-echoclient */

func main() {
	var ihelp *bool = flag.Bool("h", false, "Show help information")
	var iport *int = flag.Int("p", 6666, "Port number")
	var ihost *string = flag.String("H", "localhost", "Host address")
	var iverb *int = flag.Int("v", 1, "Verbosity (0-6)")
	var idrop *float64 = flag.Float64("d", 0.0, "Packet drop rate")
	var elength *float64 = flag.Float64("e", 2.0, "Epoch duration (secs.)")
	flag.Parse()
	if *ihelp {
		flag.Usage()
		os.Exit(0)
	}
	if flag.NArg() > 0 {
		ok := true
		fields := strings.Split(flag.Arg(0), ":")
		ok = ok && len(fields) == 2
		if ok {
			*ihost = fields[0]
			n, err := fmt.Sscanf(fields[1], "%d", iport)
			ok = ok && n == 1 && err == nil
		}
		if !ok {
			flag.Usage()
			os.Exit(0)
		}
	}
	lsp.SetVerbose(*iverb)
	lsp.SetDropRate(float32(*idrop))
	lsp.SetEpochLength(float32(*elength))
	fmt.Printf("Connecting to server at %s:%d\n",*ihost, *iport)
	cli := lsp.NewLspClient(*ihost, *iport)
	runclient(cli)
}
