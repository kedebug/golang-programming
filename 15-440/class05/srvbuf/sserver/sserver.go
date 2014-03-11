package sserver

import (
	"../../chanbuf/bufi"
)

const (
	doinsert = iota
	doremove
	doflush
	doempty
	dofront
)

var deferOnEmpty = map[int]bool{
	doremove: true,
	dofront:  true,
}

type request struct {
	op     int
	val    interface{}
	replyc chan interface{}
}

type Buf struct {
	requestc chan *request
}

func NewBuf() *Buf {
	buf := &Buf{
		requestc: make(chan *request),
	}
	go buf.runServer()
	return buf
}

func (bp *Buf) runServer() {
	sb := bufi.NewBuf()
	db := bufi.NewBuf()
	for {
		var r *request
		if !sb.Empty() && !db.Empty() {
			b, _ := db.Remove()
			r = b.(*request)
		} else {
			r = <-bp.requestc
			if sb.Empty() && deferOnEmpty[r.op] {
				db.Insert(r)
				continue
			}
		}
		switch r.op {
		case doinsert:
			sb.Insert(r.val)
			r.replyc <- nil
		case doremove:
			v, _ := sb.Remove()
			r.replyc <- v
		case doempty:
			r.replyc <- sb.Empty()
		case doflush:
			sb.Flush()
			r.replyc <- nil
		case dofront:
			v, _ := sb.Front()
			r.replyc <- v
		}
	}
}

func (bp *Buf) dorequest(op int, val interface{}) interface{} {
	req := &request{
		op:     op,
		val:    val,
		replyc: make(chan interface{}),
	}
	bp.requestc <- req
	return <-req.replyc
}

func (bp *Buf) Insert(val interface{}) {
	bp.dorequest(doinsert, val)
}

func (bp *Buf) Remove() interface{} {
	return bp.dorequest(doremove, nil)
}

func (bp *Buf) Empty() bool {
	v := bp.dorequest(doempty, nil)
	return v.(bool)
}

func (bp *Buf) Flush() {
	bp.dorequest(doflush, nil)
}

func (bp *Buf) Front() interface{} {
	return bp.dorequest(dofront, nil)
}
