// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"runtime"
	"sync/atomic"
	"unsafe"
)

// A Pool is a set of temporary objects that may be individually saved and
// retrieved.
//
// Any item stored in the Pool may be removed automatically at any time without
// notification. If the Pool holds the only reference when this happens, the
// item might be deallocated.
//
// A Pool is safe for use by multiple goroutines simultaneously.
//
// Pool's purpose is to cache allocated but unused items for later reuse,
// relieving pressure on the garbage collector. That is, it makes it easy to
// build efficient, thread-safe free lists. However, it is not suitable for all
// free lists.
//
// An appropriate use of a Pool is to manage a group of temporary items
// silently shared among and potentially reused by concurrent independent
// clients of a package. Pool provides a way to amortize allocation overhead
// across many clients.
//
// An example of good use of a Pool is in the fmt package, which maintains a
// dynamically-sized store of temporary output buffers. The store scales under
// load (when many goroutines are actively printing) and shrinks when
// quiescent.
//
// On the other hand, a free list maintained as part of a short-lived object is
// not a suitable use for a Pool, since the overhead does not amortize well in
// that scenario. It is more efficient to have such objects implement their own
// free list.
//
type Pool struct {
	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal ��ʵ����ΪpoolLocal������
	localSize uintptr        // size of the local array local array�Ĵ�С

	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	New func() interface{} // ������Get��Pool�л�ȡ����ʱ������New����һ���¶���
}

// Local per-P Pool appendix.
type poolLocal struct { // ��Ӧÿ��P��poolLocal��ÿ��Pool��Ӧÿ��P����һ��poolLocal
	private interface{}   // Can be used only by the respective P. // ֻ�ܱ���ǰ��Pʹ��
	shared  []interface{} // Can be used by any P. // ���Ա��κε�Pʹ��
	Mutex                 // Protects shared. // ������������
	pad     [128]byte     // Prevents false sharing.
}

// Put adds x to the pool.
func (p *Pool) Put(x interface{}) { // ���������pool��
	if raceenabled {
		// Under race detector the Pool degenerates into no-op.
		// It's conforming, simple and does not introduce excessive
		// happens-before edges between unrelated goroutines.
		return
	}
	if x == nil { // Ҫ����Ķ���Ϊ�գ�ֱ�ӷ���
		return
	}
	l := p.pin()
	if l.private == nil { // �ȳ��Լ���private����
		l.private = x
		x = nil
	}
	runtime_procUnpin()
	if x == nil {
		return
	}
	l.Lock()
	l.shared = append(l.shared, x) // ���Լ���shared����
	l.Unlock()
}

// Get selects an arbitrary item from the Pool, removes it from the
// Pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
//
// If Get would otherwise return nil and p.New is non-nil, Get returns
// the result of calling p.New.
func (p *Pool) Get() interface{} { // ��Pool�л�ȡһ��Ԫ��
	if raceenabled { // ��raceenabled�����£�ֻʹ��New����
		if p.New != nil {
			return p.New()
		}
		return nil
	}
	l := p.pin()   // ��ö�Ӧ��poolLocal
	x := l.private // ���poolLocal�е�private����
	l.private = nil
	runtime_procUnpin()
	if x != nil { // ��������Ԫ�أ�����
		return x
	}
	l.Lock()
	last := len(l.shared) - 1 // ���û��private�ӹ����ֻ��Ԫ��
	if last >= 0 {
		x = l.shared[last]
		l.shared = l.shared[:last]
	}
	l.Unlock()
	if x != nil {
		return x
	}
	return p.getSlow()
}

func (p *Pool) getSlow() (x interface{}) {
	// See the comment in pin regarding ordering of the loads.
	size := atomic.LoadUintptr(&p.localSize) // load-acquire
	local := p.local                         // load-consume
	// Try to steal one element from other procs.
	pid := runtime_procPin()
	runtime_procUnpin()
	for i := 0; i < int(size); i++ { // �Ӹ�Pool����poolLocal�Ĺ�����ѡ��
		l := indexLocal(local, (pid+i+1)%int(size))
		l.Lock()
		last := len(l.shared) - 1
		if last >= 0 {
			x = l.shared[last]
			l.shared = l.shared[:last]
			l.Unlock()
			break
		}
		l.Unlock()
	}

	if x == nil && p.New != nil { // ���û�д�Pool���ҵ�������New����һ��
		x = p.New() // ������New����һ��Ԫ��
	}
	return x
}

// pin pins the current goroutine to P, disables preemption and returns poolLocal pool for the P.
// Caller must call runtime_procUnpin() when done with the pool.
func (p *Pool) pin() *poolLocal { // ��ȡ�ض���P��pool
	pid := runtime_procPin() // ��õ�ǰP��id
	// In pinSlow we store to localSize and then to local, here we load in opposite order.
	// Since we've disabled preemption, GC can not happen in between.
	// Thus here we must observe local at least as large localSize.
	// We can observe a newer/larger local, it is fine (we must observe its zero-initialized-ness).
	s := atomic.LoadUintptr(&p.localSize) // load-acquire ���pool�ı��ش�С
	l := p.local                          // load-consume
	if uintptr(pid) < s {                 // ���pidС��localSize�Ĵ�С������P�������ޱ仯��ֱ��ȡ��poolLocal
		return indexLocal(l, pid) // ���ض�Ӧpid��poolLocal
	}
	return p.pinSlow() // �����õ�pid����localSize������P�Ĵ�С�仯�ˣ�ʹ��pinSlow���poolLocal
}

func (p *Pool) pinSlow() *poolLocal {
	// Retry under the mutex.
	// Can not lock the mutex while pinned.
	runtime_procUnpin() // ��allPoolsMu����������²��ң���ʱ�����unpin
	allPoolsMu.Lock()
	defer allPoolsMu.Unlock() // ��allPoolsMu�ı�����ִ��
	pid := runtime_procPin()  // �ٴλ��P��id
	// poolCleanup won't be called while we are pinned.
	s := p.localSize
	l := p.local
	if uintptr(pid) < s { // ���Ի�ȡpoolLocal
		return indexLocal(l, pid)
	}
	if p.local == nil { // ��ȡʧ�ܣ������ǵ�һ�Σ�����ǰ��Pool���뵽allPools��
		allPools = append(allPools, p)
	}
	// If GOMAXPROCS changes between GCs, we re-allocate the array and lose the old one.
	size := runtime.GOMAXPROCS(0)                                               // ������������P�����������仯�ˣ�������ǰ��poolLocal
	local := make([]poolLocal, size)                                            // ����procs��poolLocal
	atomic.StorePointer((*unsafe.Pointer)(&p.local), unsafe.Pointer(&local[0])) // store-release �洢poolLocal
	atomic.StoreUintptr(&p.localSize, uintptr(size))                            // store-release �洢��С
	return &local[pid]                                                          // ���ض�ӦP��poolLocalָ��
}

func poolCleanup() {
	// This function is called with the world stopped, at the beginning of a garbage collection.
	// It must not allocate and probably should not call any runtime functions.
	// Defensively zero out everything, 2 reasons:
	// 1. To prevent false retention of whole Pools.
	// 2. If GC happens while a goroutine works with l.shared in Put/Get,
	//    it will retain whole Pool. So next cycle memory consumption would be doubled.
	for i, p := range allPools { // �������е�Pool
		allPools[i] = nil
		for i := 0; i < int(p.localSize); i++ {
			l := indexLocal(p.local, i)
			l.private = nil
			for j := range l.shared {
				l.shared[j] = nil
			}
			l.shared = nil
		}
		p.local = nil
		p.localSize = 0
	}
	allPools = []*Pool{}
}

var (
	allPoolsMu Mutex
	allPools   []*Pool
)

func init() {
	runtime_registerPoolCleanup(poolCleanup) // ע��pool��cleanup����
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal { // ��lת��ΪpoolLocal���ͣ����������iλ�õ�ֵ
	return &(*[1000000]poolLocal)(l)[i]
}

// Implemented in runtime.
func runtime_registerPoolCleanup(cleanup func())
func runtime_procPin() int
func runtime_procUnpin()
