// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import "sync/atomic"

// fdMutex is a specialized synchronization primitive
// that manages lifetime of an fd and serializes access
// to Read and Write methods on netFD.
type fdMutex struct { // ���ļ������л�ͬ��ԭ�����fd���������ڲ����л���д����
	state uint64 // ��mutex��״̬
	rsema uint32
	wsema uint32
}

// fdMutex.state is organized as follows:
// 1 bit - whether netFD is closed, if set all subsequent lock operations will fail.
// 1 bit - lock for read operations.
// 1 bit - lock for write operations.
// 20 bits - total number of references (read+write+misc).
// 20 bits - number of outstanding read waiters.
// 20 bits - number of outstanding write waiters.
const (
	mutexClosed  = 1 << 0 // ��һλ�����Ƿ�netFD���ر�
	mutexRLock   = 1 << 1 // �ڶ�λ������������
	mutexWLock   = 1 << 2 // ����λ��д��������
	mutexRef     = 1 << 3 // �ӵ���λ��ʼΪ���ü������ܹ�20λ
	mutexRefMask = (1<<20 - 1) << 3
	mutexRWait   = 1 << 23
	mutexRMask   = (1<<20 - 1) << 23 // �ӵ�24λ��ʼΪ�ȴ����ߵ��������ܹ�20λ
	mutexWWait   = 1 << 43
	mutexWMask   = (1<<20 - 1) << 43 // �ӵ�44Ϊ��ʼΪ�ȴ�д�ߵ��������ܹ�20λ
)

// Read operations must do RWLock(true)/RWUnlock(true).
// Write operations must do RWLock(false)/RWUnlock(false).
// Misc operations must do Incref/Decref. Misc operations include functions like
// setsockopt and setDeadline. They need to use Incref/Decref to ensure that
// they operate on the correct fd in presence of a concurrent Close call
// (otherwise fd can be closed under their feet).
// Close operation must do IncrefAndClose/Decref.

// RWLock/Incref return whether fd is open.
// RWUnlock/Decref return whether fd is closed and there are no remaining references.

func (mu *fdMutex) Incref() bool {
	for {
		old := atomic.LoadUint64(&mu.state) // ԭ�ӷ����ϵ�stateֵ
		if old&mutexClosed != 0 {           // ����Ѿ��رգ��޷��������ü���
			return false
		}
		new := old + mutexRef      // �������ü�������mutexRefΪ��λ
		if new&mutexRefMask == 0 { // ���ܳ����������ü���
			panic("net: inconsistent fdMutex")
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) { // Ϊ���ü���ԭ�Ӹ�ֵ
			return true
		}
	}
}

func (mu *fdMutex) IncrefAndClose() bool { // �������ü�����ͬʱ���fdΪ�ر�״̬
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexClosed != 0 {
			return false
		}
		// Mark as closed and acquire a reference.
		new := (old | mutexClosed) + mutexRef
		if new&mutexRefMask == 0 {
			panic("net: inconsistent fdMutex")
		}
		// Remove all read and write waiters.
		new &^= mutexRMask | mutexWMask
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			// Wake all read and write waiters,
			// they will observe closed flag after wakeup.
			for old&mutexRMask != 0 {
				old -= mutexRWait
				runtime_Semrelease(&mu.rsema)
			}
			for old&mutexWMask != 0 {
				old -= mutexWWait
				runtime_Semrelease(&mu.wsema)
			}
			return true
		}
	}
}

func (mu *fdMutex) Decref() bool { // �������ü���
	for {
		old := atomic.LoadUint64(&mu.state) // ԭ�ӻ�ȡstateֵ
		if old&mutexRefMask == 0 {
			panic("net: inconsistent fdMutex")
		}
		new := old - mutexRef
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			return new&(mutexClosed|mutexRefMask) == mutexClosed
		}
	}
}

func (mu *fdMutex) RWLock(read bool) bool { // ����read��������fd�Ӷ���������д��
	var mutexBit, mutexWait, mutexMask uint64
	var mutexSema *uint32
	if read {
		mutexBit = mutexRLock
		mutexWait = mutexRWait
		mutexMask = mutexRMask
		mutexSema = &mu.rsema
	} else {
		mutexBit = mutexWLock
		mutexWait = mutexWWait
		mutexMask = mutexWMask
		mutexSema = &mu.wsema
	}
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexClosed != 0 {
			return false
		}
		var new uint64
		if old&mutexBit == 0 {
			// Lock is free, acquire it.
			new = (old | mutexBit) + mutexRef
			if new&mutexRefMask == 0 {
				panic("net: inconsistent fdMutex")
			}
		} else {
			// Wait for lock.
			new = old + mutexWait
			if new&mutexMask == 0 {
				panic("net: inconsistent fdMutex")
			}
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			if old&mutexBit == 0 {
				return true
			}
			runtime_Semacquire(mutexSema)
			// The signaller has subtracted mutexWait.
		}
	}
}

func (mu *fdMutex) RWUnlock(read bool) bool { // ����read��������fd���������д��
	var mutexBit, mutexWait, mutexMask uint64
	var mutexSema *uint32
	if read {
		mutexBit = mutexRLock
		mutexWait = mutexRWait
		mutexMask = mutexRMask
		mutexSema = &mu.rsema
	} else {
		mutexBit = mutexWLock
		mutexWait = mutexWWait
		mutexMask = mutexWMask
		mutexSema = &mu.wsema
	}
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexBit == 0 || old&mutexRefMask == 0 {
			panic("net: inconsistent fdMutex")
		}
		// Drop lock, drop reference and wake read waiter if present.
		new := (old &^ mutexBit) - mutexRef
		if old&mutexMask != 0 {
			new -= mutexWait
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			if old&mutexMask != 0 {
				runtime_Semrelease(mutexSema)
			}
			return new&(mutexClosed|mutexRefMask) == mutexClosed
		}
	}
}

// Implemented in runtime package.
func runtime_Semacquire(sema *uint32)
func runtime_Semrelease(sema *uint32)
