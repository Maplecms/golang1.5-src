// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ring implements operations on circular lists.
package ring

// A Ring is an element of a circular list, or ring.
// Rings do not have a beginning or end; a pointer to any ring element
// serves as reference to the entire ring. Empty rings are represented
// as nil Ring pointers. The zero value for a Ring is a one-element
// ring with a nil Value.
//
type Ring struct { // ���Ľṹ
	next, prev *Ring       // ����˫��ָ��
	Value      interface{} // for use by client; untouched by this library
}

func (r *Ring) init() *Ring { // ��ʼ��һ��Ring��ָ������
	r.next = r
	r.prev = r
	return r
}

// Next returns the next ring element. r must not be empty.
func (r *Ring) Next() *Ring { // ��ú�һ�����ṹ
	if r.next == nil { // ���nextָ��Ϊnil���Ƚ��г�ʼ��
		return r.init()
	}
	return r.next
}

// Prev returns the previous ring element. r must not be empty.
func (r *Ring) Prev() *Ring { // ���ǰһ�����ṹ
	if r.next == nil {
		return r.init()
	}
	return r.prev
}

// Move moves n % r.Len() elements backward (n < 0) or forward (n >= 0)
// in the ring and returns that ring element. r must not be empty.
//
func (r *Ring) Move(n int) *Ring { // ��ǰ������ƶ�n��
	if r.next == nil {
		return r.init()
	}
	switch {
	case n < 0:
		for ; n < 0; n++ {
			r = r.prev
		}
	case n > 0:
		for ; n > 0; n-- {
			r = r.next
		}
	}
	return r
}

// New creates a ring of n elements.
func New(n int) *Ring { // ����һ���µĻ���nΪ��ʼԪ�ص�����
	if n <= 0 {
		return nil
	}
	r := new(Ring)
	p := r
	for i := 1; i < n; i++ { // �������ṹ
		p.next = &Ring{prev: p}
		p = p.next
	}
	p.next = r
	r.prev = p
	return r
}

// Link connects ring r with ring s such that r.Next()
// becomes s and returns the original value for r.Next().
// r must not be empty.
//
// If r and s point to the same ring, linking
// them removes the elements between r and s from the ring.
// The removed elements form a subring and the result is a
// reference to that subring (if no elements were removed,
// the result is still the original value for r.Next(),
// and not nil).
//
// If r and s point to different rings, linking
// them creates a single ring with the elements of s inserted
// after r. The result points to the element following the
// last element of s after insertion.
//
func (r *Ring) Link(s *Ring) *Ring { // ��s���뻷�У�����rԭ����nextԪ��
	n := r.Next()
	if s != nil {
		p := s.Prev()
		// Note: Cannot use multiple assignment because
		// evaluation order of LHS is not specified.
		r.next = s
		s.prev = r
		n.prev = p
		p.next = n
	}
	return n
}

// Unlink removes n % r.Len() elements from the ring r, starting
// at r.Next(). If n % r.Len() == 0, r remains unchanged.
// The result is the removed subring. r must not be empty.
//
func (r *Ring) Unlink(n int) *Ring { // ɾ������n��Ԫ��
	if n <= 0 {
		return nil
	}
	return r.Link(r.Move(n + 1))
}

// Len computes the number of elements in ring r.
// It executes in time proportional to the number of elements.
//
func (r *Ring) Len() int { // ����Ring�ĳ���
	n := 0
	if r != nil {
		n = 1
		for p := r.Next(); p != r; p = p.next {
			n++
		}
	}
	return n
}

// Do calls function f on each element of the ring, in forward order.
// The behavior of Do is undefined if f changes *r.
func (r *Ring) Do(f func(interface{})) { // ��ÿ��Ԫ��ѭ������f
	if r != nil { // �����ǰ���ǿ�
		f(r.Value)
		for p := r.Next(); p != r; p = p.next {
			f(p.Value)
		}
	}
}
