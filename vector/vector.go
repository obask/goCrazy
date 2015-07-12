// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package list implements a doubly linked list.
//
// To iterate over a list (where l is a *List):
//	for e := l.Front(); e != nil; e = e.Next() {
//		// do something with e.Value
//	}
//
package vector

// FIXME it looks like vector but it is not!! replace dLists to []


// Element is an element of a linked list.
type Elem struct {
	// Next and previous pointers in the doubly-linked list of elements.
	// To simplify the implementation, internally a list l is implemented
	// as a ring, such that &l.root is both the next element of the last
	// list element (l.Back()) and the previous element of the first list
	// element (l.Front()).
	next, prev *Elem

	// The list to which this element belongs.
	list *Vector

	// The value stored with this element.
	Value interface{}
}


// List represents a doubly linked list.
// The zero value for List is an empty list ready to use.
type Vector struct {
	root Elem // sentinel list element, only &root, root.prev, and root.next are used
	len  int     // current list length excluding (this) sentinel element
}

// Init initializes or clears list l.
func (l *Vector) Init() *Vector {
	l.root.next = &l.root
	l.root.prev = &l.root
	l.len = 0
	return l
}

// New returns an initialized list.
func New() *Vector { return new(Vector).Init() }

// New returns an initialized list.
func FromArray(arr []interface{}) *Vector {
	ll := New()
	for _, val := range arr {
		ll.PushBack(val)
	}
	return ll
}


// insertValue is a convenience wrapper for insert(&Element{Value: v}, at).
func (l *Vector) insertValue(v interface{}, at *Elem) *Elem {
	return l.insert(&Elem{Value: v}, at)
}

// lazyInit lazily initializes a zero List value.
func (l *Vector) lazyInit() {
	if l.root.next == nil {
		l.Init()
	}
}

// PushBack inserts a new element e with value v at the back of list l and returns e.
func (l *Vector) PushBack(v interface{}) *Elem {
	l.lazyInit()
	return l.insertValue(v, l.root.prev)
}

// insert inserts e after at, increments l.len, and returns e.
func (l *Vector) insert(e, at *Elem) *Elem {
	n := at.next
	at.next = e
	e.prev = at
	e.next = n
	n.prev = e
	e.list = l
	l.len++
	return e
}

// Next returns the next list element or nil.
func (e *Elem) Next() *Elem {
	if p := e.next; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// Front returns the first element of list l or nil.
func (l *Vector) Front() *Elem {
	if l.len == 0 {
		return nil
	}
	return l.root.next
}


// TODO add description.
func (l *Vector) Map(f func(interface{}) interface{}) *Vector {
	l.lazyInit()
	res := New()
	for e := l.Front(); e != nil; e = e.Next() {
		//		fmt.Println(e.Value)
		res.PushBack(f(e.Value))
	}
	return res
}


// TODO add description.
func (l *Vector) FlatMap(f func(interface{}) []interface{}) *Vector {
	l.lazyInit()
	res := New()
	for e := l.Front(); e != nil; e = e.Next() {
		tmp := f(e.Value)
		for _, val := range tmp {
			res.PushBack(val)
		}
	}
	return res
}



// TODO add description.
func (l *Vector) Filter(f func(interface{}) bool) *Vector {
	l.lazyInit()
	res := New()
	for e := l.Front(); e != nil; e = e.Next() {
		if f(e.Value) {
			res.PushBack(e.Value)
		}
	}
	return res
}


// TODO add description.
func (l *Vector) ToArray() []interface{} {
	var ys []interface{}
	l.lazyInit()
	for e := l.Front(); e != nil; e = e.Next() {
		ys = append(ys, e.Value)
	}
	return ys
}
