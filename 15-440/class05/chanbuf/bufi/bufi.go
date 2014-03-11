package bufi

import (
	"errors"
)

type BufEle struct {
	val  interface{}
	next *BufEle
}

type Buf struct {
	head *BufEle
	tail *BufEle
}

func NewBuf() *Buf {
	return new(Buf)
}

func (bp *Buf) Insert(val interface{}) {
	ele := &BufEle{val: val}
	if bp.head == nil {
		bp.head = ele
	} else {
		bp.tail.next = ele
	}
	bp.tail = ele
}

func (bp *Buf) Front() (val interface{}, err error) {
	if bp.head == nil {
		err = errors.New("Empty buffer")
		return
	}
	val = bp.head.val
	return
}

func (bp *Buf) Remove() (val interface{}, err error) {
	ele := bp.head
	if ele == nil {
		err = errors.New("Empty buffer")
		return
	} else {
		val = ele.val
		bp.head = ele.next
		if ele == bp.tail {
			bp.tail = nil
		}
	}
	return
}

func (bp *Buf) Empty() bool {
	return bp.head == nil
}

func (bp *Buf) Flush() {
	bp.head = nil
	bp.tail = nil
}
