package rpclib

import (
	"encoding/gob"
	"fmt"
	"github.com/kedebug/golang-programming/15-440/class07/bufi"
	"github.com/kedebug/golang-programming/15-440/class07/dserver"
	"gob"
	"log"
	"net"
	"net/http"
	"net/rpc"
)

var verbosity int = 1

func SetVerbosity(verb int) {
	verbosity = verb
}

func Vlogf(level int, format string, v ...interface{}) {
	if level <= verbosity {
		log.Printf(format, v...)
	}
}

func CheckReport(level int, err error) bool {
	if err == nil {
		return false
	}
	Vlogf(level, "Error: %s", err.Error())
	return true
}

func CheckFatal(err error) {
	if err == nil {
		return
	}
	log.Fatalf("Fatal: %s", err.Error())
}

type Val struct {
	X interface{}
}

func nullVal() Val {
	return Val{nil}
}

func trueVal() Val {
	return Val{true}
}

func falseVal() Val {
	return Val{false}
}

func truth(v *Val) bool {
	return v.X.(bool)
}

type SrvBuf struct {
	abuf *dserver.Buf
}

type Islice []interface{}

var islice Islice
var sbuf *bufi.Buf

func NewSrvBuf() *SrvBuf {
	gob.Register(islice)
	gob.Register(sbuf)
	return &SrvBuf{abuf: dserver.NewBuf()}
}

func (srv *SrvBuf) Insert(arg *Val, reply *Val) error {
	srv.abuf.Insert(arg.X)
	*reply = nullVal()
	Vlogf(2, "Inserted: %v\n", arg.X)
	Vlogf(3, "Buffer: %s\n", srv.abuf.String())
	return nil
}

func (srv *SrvBuf) Front(arg *Val, reply *Val) error {
	*reply = Val{srv.abuf.Front()}
	Vlogf(2, "Front value %v\n", reply.X)
	Vlogf(3, "Buffer: %s\n", srv.abuf.String())
	return nil
}

// No argument, reply = front value
func (srv *SrvBuf) Remove(arg *Val, reply *Val) error {
	*reply = Val{srv.abuf.Remove()}
	Vlogf(2, "Removed %v\n", reply.X)
	Vlogf(3, "Buffer: %s\n", srv.abuf.String())
	return nil
}

// No argument, reply = boolean
func (srv *SrvBuf) Empty(arg *Val, reply *Val) error {
	if srv.abuf.Empty() {
		*reply = trueVal()
	} else {
		*reply = falseval()
	}
	Vlogf(2, "Empty? %v\n", reply.X)
	Vlogf(3, "Buffer: %s\n", srv.abuf.String())
	return nil
}

// No argument, no reply
func (srv *SrvBuf) Flush(arg *Val, reply *Val) error {
	srv.abuf.Flush()
	*reply = nullVal()
	Vlogf(2, "Flushed\n")
	Vlogf(3, "Buffer: %s\n", srv.abuf.String())
	return nil
}

// No argument, reply = slice
func (srv *SrvBuf) Contents(arg *Val, reply *Val) error {
	c := Islice(srv.abuf.Contents())
	*reply = Val{c}
	Vlogf(2, "Generated contents: %v\n", c)
	return nil
}

// No argument, reply = *bufi.Buf
func (srv *SrvBuf) List(arg *Val, reply *Val) error {
	b := srv.abuf.List()
	*reply = Val{b}
	Vlogf(2, "Generated list: %v\n", b.Contents())
	return nil
}

func Serve(port int) {
	srv := NewSrvBuf()
	rpc.Register(srv)
	rpc.HandleHTTP()
	addr := fmt.Sprintf(":%d", port)
	l, e := net.Listen("tcp", addr)
	CheckFatal(e)
	Vlogf(1, "Running server on port: %d\n", port)
	Vlogf(3, "Buffer: %s\n", srv.abuf.String())
}
