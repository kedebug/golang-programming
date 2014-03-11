package abuf

import (
	"../bufi"
)

type Buf struct {
	buf      *bufi.Buf
	ackchan  chan int
	opchan   chan int
	readchan chan int
}

func NewBuf() *Buf {
	buf := &Buf{
		buf:      bufi.NewBuf(),
		ackchan:  make(chan int),
		opchan:   make(chan int),
		readchan: make(chan int),
	}
	go buf.director()
	return buf
}

func (bp *Buf) director() {
	for {
		if bp.buf.Empty() {
			bp.opchan <- 1
		} else {
			select {
			case bp.opchan <- 1:
			case bp.readchan <- 1:
			}
		}
		<-bp.ackchan
	}
}

func (bp *Buf) startop() { <-bp.opchan }

func (bp *Buf) startread() { <-bp.readchan }

func (bp *Buf) finish() { bp.ackchan <- 1 }

func (bp *Buf) Insert(val interface{}) {
	bp.startop()
	defer bp.finish()
	bp.buf.Insert(val)
}

func (bp *Buf) Remove() interface{} {
	bp.startread()
	defer bp.finish()
	val, _ := bp.buf.Remove()
	return val
}

func (bp *Buf) Front() interface{} {
	bp.startread()
	defer bp.finish()
	val, _ := bp.buf.Front()
	return val
}

func (bp *Buf) Empty() bool {
	bp.startop()
	defer bp.finish()
	return bp.buf.Empty()
}

func (bp *Buf) Flush() {
	bp.startop()
	defer bp.finish()
	bp.buf.Flush()
}
