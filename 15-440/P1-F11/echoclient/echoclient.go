// Implementation of an echo client based on LSP

package main

import (
	"flag"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsp"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lsplog"
	"github.com/kedebug/golang-programming/15-440/P1-F11/lspnet"
	"os"
	"strings"
)

func runclient(cli *lsp.LspClient) {
	for {
		var s string
		// Get next token from input
		fmt.Printf("CLI-SRV: ")
		_, err := fmt.Scan(&s)
		if err != nil || strings.EqualFold(s, "quit") {
			cli.Close()
			return
		}
		// Send to server
		cli.Write([]byte(s))
		// Read from server
		payload := cli.Read()
		if payload != nil {
			fmt.Printf("SRV-CLI: [%s]\n", string(payload))
		}
	}
	fmt.Printf("Exiting\n")
	cli.Close()
}

func main() {
	var ihelp *bool = flag.Bool("h", false, "Show help information")
	var iport *int = flag.Int("p", 6666, "Port number")
	var ihost *string = flag.String("H", "localhost", "Host address")
	var iverb *int = flag.Int("v", 4, "Verbosity (0-6)")
	var irdrop *int = flag.Int("r", 0, "Network read packet drop percentage")
	var iwdrop *int = flag.Int("w", 0, "Network write packet drop percentage")
	var elim *int = flag.Int("k", 5, "Epoch limit")
	var ems *int = flag.Int("d", 2000, "Epoch duration (millisecconds)")
	flag.Parse()
	if *ihelp {
		flag.Usage()
		os.Exit(0)
	}
	if flag.NArg() > 0 {
		// Look for host:port on command line
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
	params := &lsp.LspParams{*elim, *ems}

	lsplog.SetVerbose(*iverb)
	lspnet.SetReadDropPercent(*irdrop)
	lspnet.SetWriteDropPercent(*iwdrop)
	hostport := fmt.Sprintf("%s:%v", *ihost, *iport)
	fmt.Printf("Connecting to server at %s\n", hostport)
	cli, err := lsp.NewLspClient(hostport, params)
	if err != nil {
		fmt.Printf("... failed.  Error message %s\n", err.Error())
	}
	if lsplog.CheckReport(1, err) {
		return
	}
	runclient(cli)
}
