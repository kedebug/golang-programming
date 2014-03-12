package main

import (
	"fmt"
	"github.com/kedebug/golang-programming/15-440/class07/rpclib"
)

func main() {
	var port int = 19910
	var host string = "localhost"
	cli := rpclib.NewSClient(host, port)
	rpclib.SetVerbosity(5)
	for {
		fmt.Printf("cmd:")
		var cmd, arg string
		n, _ := fmt.Scanln(&cmd, &arg)
		if n < 1 {
			fmt.Printf("Exiting\n")
			return
		}
		switch cmd {
		case "i":
			if n < 2 {
				fmt.Println("need 2 argument")
			} else {
				var iarg int
				n, _ := fmt.Sscanf(arg, "%d", &iarg)
				if n > 0 {
					cli.Insert(iarg)
				} else {
					cli.Insert(arg)
				}
			}
		case "?":
			v := cli.Front()
			fmt.Println("front value: %v", v)
		case "r":
			v := cli.Remove()
			fmt.Printf("remove value: %v", v)
		case "f":
			cli.Flush()
			fmt.Println("flushed")
		case "e":
			e := cli.Empty()
			if e {
				fmt.Println("empty")
			} else {
				fmt.Println("not empty")
			}
		case "c":
			v := cli.Contents()
			fmt.Println("Contents: %v", v)
		case "l":
			b := cli.List()
			fmt.Println("List: %v", b.Contents())
		case "q":
			fmt.Println("Quit")
			return
		default:
			fmt.Println("Unrecognized command '%s'", cmd)
		}
	}
}
