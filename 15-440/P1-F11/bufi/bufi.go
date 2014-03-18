package bufi

import (
	"encoding/json"
	"errors"
)

// Linked list element
type BufEle struct {
	Val  interface{}
	Next *BufEle
}

type Buf struct {
	Head *BufEle // Oldest element
	tail *BufEle // Most recently inserted element
	cnt  int     // Number of elements in list
}

func NewBuf() *Buf {
	return new(Buf)
}

func (bp *Buf) Insert(val interface{}) {
	ele := &BufEle{Val: val}
	if bp.Head == nil {
		// Inserting into empty list
		bp.Head = ele
		bp.tail = ele
	} else {
		bp.tail.Next = ele
		bp.tail = ele
	}
	bp.cnt = bp.cnt + 1
}

func (bp *Buf) Front() (interface{}, error) {
	if bp.Head == nil {
		return nil, errors.New("Empty Buffer")
	}
	return bp.Head.Val, nil
}

func (bp *Buf) Remove() (interface{}, error) {
	e := bp.Head
	if e == nil {
		return nil, errors.New("Empty Buffer")
	}
	bp.Head = e.Next
	// List becoming empty
	if e == bp.tail {
		bp.tail = nil
	}
	bp.cnt = bp.cnt - 1
	return e.Val, nil
}

func (bp *Buf) Empty() bool {
	return bp.Head == nil
}

func (bp *Buf) Flush() {
	bp.Head = nil
	bp.tail = nil
	bp.cnt = 0
}

// Return slice containing entire buffer contents
func (bp *Buf) Contents() []interface{} {
	result := make([]interface{}, bp.cnt)
	e := bp.Head
	for i := 0; i < bp.cnt; i++ {
		result[i] = e.Val
		e = e.Next
	}
	return result
}

func (bp *Buf) String() string {
	b, e := json.MarshalIndent(*bp, "", "  ")
	if e != nil {
		return e.Error()
	}
	return string(b)
}
