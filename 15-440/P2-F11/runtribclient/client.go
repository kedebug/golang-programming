package main

import (
	"flag"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/P2-F11/tribclient"
	"github.com/kedebug/golang-programming/15-440/P2-F11/tribproto"
	"log"
	"strings"
)

// For parsing the command line
type cmd_info struct {
	cmdline  string
	funcname string
	nargs    int // number of required args
}

const (
	CMD_PUT = iota
	CMD_GET
)

var portnum *int = flag.Int("port", 9010, "server port # to connect to")

func main() {

	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("Insufficient arguments to client")
	}

	cmd := flag.Arg(0)

	serverAddress := "localhost"
	serverPort := fmt.Sprintf("%d", *portnum)
	client, _ := tribclient.NewTribbleclient(serverAddress, serverPort)

	cmdlist := []cmd_info{
		{"uc", "Tribserver.CreateUser", 1},
		{"sl", "Tribserver.GetSubscriptions", 1},
		{"sa", "Tribserver.AddSubscription", 2},
		{"sr", "Tribserver.RemoveSubscription", 2},
		{"tl", "Tribserver.GetTribbles", 1},
		{"tp", "Tribserver.AddTribble", 2},
		{"ts", "Tribserver.GetTribblesBySubscription", 1},
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

	switch cmd {
	case "uc": // user create
		// oh, Go.  If only you let me do this right...
		status, err := client.CreateUser(flag.Arg(1))
		PrintStatus(ci.funcname, status, err)
	case "sl": // subscription list
		subs, status, err := client.GetSubscriptions(flag.Arg(1))
		PrintStatus(ci.funcname, status, err)
		if err == nil && status == tribproto.OK {
			fmt.Println(strings.Join(subs, " "))
		}
	case "sa":
		status, err := client.AddSubscription(flag.Arg(1), flag.Arg(2))
		PrintStatus(ci.funcname, status, err)
	case "sr": // subscription remove
		status, err := client.RemoveSubscription(flag.Arg(1), flag.Arg(2))
		PrintStatus(ci.funcname, status, err)
	case "tl": // tribble list
		tribbles, status, err := client.GetTribbles(flag.Arg(1))
		PrintStatus(ci.funcname, status, err)
		if err == nil && status == tribproto.OK {
			PrintTribbles(tribbles)
		}
	case "ts": // tribbles by subscription
		tribbles, status, err := client.GetTribblesBySubscription(flag.Arg(1))
		PrintStatus(ci.funcname, status, err)
		if err == nil && status == tribproto.OK {
			PrintTribbles(tribbles)
		}
	case "tp": // tribble post
		status, err := client.PostTribble(flag.Arg(1), flag.Arg(2))
		PrintStatus(ci.funcname, status, err)
	}
}

// This is a little lazy, but there are only 4 entries...
func TribStatusToString(status int) string {
	switch status {
	case tribproto.OK:
		return "OK"
	case tribproto.ENOSUCHUSER:
		return "No such user"
	case tribproto.ENOSUCHTARGETUSER:
		return "No such target user"
	case tribproto.EEXISTS:
		return "User already exists"
	}
	return "Unknown error"
}

func PrintStatus(cmdname string, status int, err error) {
	if status == tribproto.OK {
		fmt.Printf("%s succeeded\n", cmdname)
	} else {
		fmt.Printf("%s failed: %s\n", cmdname, TribStatusToString(status))
	}
}

func PrintTribble(t tribproto.Tribble) {
	fmt.Printf("%16.16s - %s - %s\n",
		t.Userid, t.Posted.String(), t.Contents)

}

func PrintTribbles(tribbles []tribproto.Tribble) {
	for _, t := range tribbles {
		PrintTribble(t)
	}
}
