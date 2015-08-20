// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
//
// Each Cond has an associated Locker L (often a *Mutex or *RWMutex),
// which must be held when changing the condition and
// when calling the Wait method.
//
// A Cond can be created as part of other structures.
// A Cond must not be copied after first use.
type Cond struct { // �����ṹ
	// L is held while observing or changing the condition
	L Locker // ��Ӧ�����������͸���һ�𣬹��ɶ���Դʹ�õı���

	sema    syncSema
	waiters uint32      // number of waiters �ȴ��ߵ�����
	checker copyChecker // ָ��checker�����һ��ָ�룬�����ж�cond�Ƿ񱻿�����
}

// NewCond returns a new Cond with Locker l.
func NewCond(l Locker) *Cond { // �����µ���������������Lock
	return &Cond{L: l}
}

// Wait atomically unlocks c.L and suspends execution
// of the calling goroutine.  After later resuming execution,
// Wait locks c.L before returning.  Unlike in other systems,
// Wait cannot return unless awoken by Broadcast or Signal.
//
// Because c.L is not locked when Wait first resumes, the caller
// typically cannot assume that the condition is true when
// Wait returns.  Instead, the caller should Wait in a loop:
//
//    c.L.Lock()
//    for !condition() {
//        c.Wait()
//    }
//    ... make use of condition ...
//    c.L.Unlock()
//
// �ͷ������ȴ�ִ�У�����������㣬��������ʱ�Ѽ�����
func (c *Cond) Wait() { // �ȴ���������
	c.checker.check() // ���Condû�б�����
	if raceenabled {
		raceDisable()
	}
	atomic.AddUint32(&c.waiters, 1) // �ȴ��ߵ���������
	if raceenabled {
		raceEnable()
	}
	c.L.Unlock()                    // �Ƚ���
	runtime_Syncsemacquire(&c.sema) // �ȴ���sema��
	c.L.Lock()
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
// ֪ͨ������һ���ȴ��ߣ�����������
func (c *Cond) Signal() {
	c.signalImpl(false)
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
// ֪ͨ���������еȴ��ߣ�����������
func (c *Cond) Broadcast() {
	c.signalImpl(true)
}

func (c *Cond) signalImpl(all bool) { // ֪ͨ�ľ���ʵ�֣�all��ʾ�Ƿ�֪ͨ���еĵȴ���
	c.checker.check() // ���Condû�б�����
	if raceenabled {
		raceDisable()
	}
	for {
		old := atomic.LoadUint32(&c.waiters) // �鿴�ж������ڵȴ�������
		if old == 0 {                        // ���û�˵ȴ���ֱ�ӷ���
			if raceenabled {
				raceEnable()
			}
			return
		}
		new := old - 1
		if all {
			new = 0
		}
		if atomic.CompareAndSwapUint32(&c.waiters, old, new) {
			if raceenabled {
				raceEnable()
			}
			runtime_Syncsemrelease(&c.sema, old-new) // ���û��Ѷ��ٸ�
			return
		}
	}
}

// copyChecker holds back pointer to itself to detect object copying.
type copyChecker uintptr // copyChecker����ָ���Լ���ָ�룬���������󿽱�

func (c *copyChecker) check() {
	if uintptr(*c) != uintptr(unsafe.Pointer(c)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(c), 0, uintptr(unsafe.Pointer(c))) &&
		uintptr(*c) != uintptr(unsafe.Pointer(c)) { // ���c��ָ�������ˣ�����c����Ϊ��(��ʼ�����������ʱ��c��ֵ)��
		panic("sync.Cond is copied") // �����ʼ��ʧ�ܣ����߲�ָ�������ˣ�panic
	}
}
