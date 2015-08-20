// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sync provides basic synchronization primitives such as mutual
// exclusion locks.  Other than the Once and WaitGroup types, most are intended
// for use by low-level library routines.  Higher-level synchronization is
// better done via channels and communication.
//
// Values containing the types defined in this package should not be copied.
package sync

import (
	"sync/atomic"
	"unsafe"
)

// A Mutex is a mutual exclusion lock.
// Mutexes can be created as part of other structures;
// the zero value for a Mutex is an unlocked mutex.
type Mutex struct { //Mutex������
	state int32  // ��������״̬��Ϣ�����һλΪ1������������������2Ϊ�����Ƿ�ձ����ѣ�Ȼ��ǰ���λ�����ȴ��ߵ�����
	sema  uint32 // �ȴ�˯�ߵ�sema
}

// A Locker represents an object that can be locked and unlocked.
type Locker interface { // ���ӿڣ�����һ��������Ա��ӽ���
	Lock()
	Unlock()
}

const ( // ����״̬
	mutexLocked      = 1 << iota // mutex is locked ����������һλ�����Ƿ�����
	mutexWoken                   // �ڶ�λ�����Ƿ�ձ�����
	mutexWaiterShift = iota      // �ȴ�����ʼ��λ��������λ
)

// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.
func (m *Mutex) Lock() { // ����������
	// Fast path: grab unlocked mutex.
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) { // CAS�������������ԭֵΪ0����Ϊ1�����������ɹ�
		if raceenabled {
			raceAcquire(unsafe.Pointer(m))
		}
		return // �����ɹ�
	}
	// ����������ɹ�
	awoke := false // ��ֵΪ��ǰ��goroutineδ������
	iter := 0
	for {
		old := m.state
		new := old | mutexLocked
		if old&mutexLocked != 0 {
			if runtime_canSpin(iter) {
				// Active spinning makes sense.
				// Try to set mutexWoken flag to inform Unlock
				// to not wake other blocked goroutines.
				if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
					atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
					awoke = true
				}
				runtime_doSpin()
				iter++
				continue
			}
			new = old + 1<<mutexWaiterShift
		}
		if awoke { // ��ǰ��goroutine��˯���б����ѣ�
			// The goroutine has been woken from sleep,
			// so we need to reset the flag in either case.
			if new&mutexWoken == 0 {
				panic("sync: inconsistent mutex state")
			}
			new &^= mutexWoken // ��Wokenλ��0
		}
		if atomic.CompareAndSwapInt32(&m.state, old, new) { // ״̬δ�䣬����״̬������״̬������һ���ȴ��ߣ����������for��һ��
			if old&mutexLocked == 0 { // ��������ͷ��ˣ�ֱ������
				break
			}
			runtime_Semacquire(&m.sema) // ��ǰ��goroutine�����ȴ�������
			awoke = true                // ��ǰ��goroutine�����ѣ�����ִ��һ��forѭ��
			iter = 0
		}
	}

	if raceenabled { // �����������
		raceAcquire(unsafe.Pointer(m))
	}
}

// Unlock unlocks m.
// It is a run-time error if m is not locked on entry to Unlock.
//
// A locked Mutex is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a Mutex and then
// arrange for another goroutine to unlock it.
func (m *Mutex) Unlock() { // ����������
	if raceenabled {
		_ = m.state
		raceRelease(unsafe.Pointer(m))
	}

	// Fast path: drop lock bit.
	new := atomic.AddInt32(&m.state, -mutexLocked) // ��ȥmutexLockedλ���൱�ڽ���
	if (new+mutexLocked)&mutexLocked == 0 {        // ����ظ������ˣ�panic
		panic("sync: unlock of unlocked mutex")
	}
	// ������п����ڶ��goroutine��������
	old := new
	for {
		// If there are no waiters or a goroutine has already
		// been woken or grabbed the lock, no need to wake anyone.
		if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken) != 0 { // û���˵ȴ���������һ���Ѿ������ˣ�ֱ�ӷ���
			return
		}
		// Grab the right to wake someone.
		new = (old - 1<<mutexWaiterShift) | mutexWoken      // ���û���һ��
		if atomic.CompareAndSwapInt32(&m.state, old, new) { // ���Ի���һ��
			runtime_Semrelease(&m.sema) // ���ѵȴ���
			return
		}
		old = m.state
	}
}
