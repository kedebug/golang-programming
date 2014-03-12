package main

import (
	"github.com/kedebug/golang-programming/15-440/class07/rpclib"
)

func main() {
	rpclib.SetVerbosity(5)
	rpclib.Serve(19910)
}
