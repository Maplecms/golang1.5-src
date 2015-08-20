// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Time-related runtime and pieces of package time.

package runtime

import "unsafe"

// Package time knows the layout of this structure.
// If this struct changes, adjust ../time/sleep.go:/runtimeTimer.
// For GOOS=nacl, package syscall knows the layout of this structure.
// If this struct changes, adjust ../syscall/net_nacl.go:/runtimeTimer.
type timer struct { // 定时器结构
	i int // heap index 堆索引

	// Timer wakes up at when, and then at when+period, ... (period > 0 only)
	// each time calling f(now, arg) in the timer goroutine, so f must be
	// a well-behaved function and not block.
	when   int64                      // 启动开始时间
	period int64                      // 周期性执行时间
	f      func(interface{}, uintptr) // 到期后执行的函数
	arg    interface{}                // 到期后执行函数的参数
	seq    uintptr
}

var timers struct { // 全局唯一的timers变量
	lock         mutex
	gp           *g   // 执行定时器处理的goroutine
	created      bool // 执行定时器处理的goroutine是否已被创建
	sleeping     bool
	rescheduling bool
	waitnote     note
	t            []*timer
}

// nacl fake time support - time in nanoseconds since 1970
var faketime int64

// Package time APIs.
// Godoc uses the comments in package time, not these.

// time.now is implemented in assembly.

// timeSleep puts the current goroutine to sleep for at least ns nanoseconds.
//go:linkname timeSleep time.Sleep
func timeSleep(ns int64) { // 睡眠ns时间
	if ns <= 0 { // n不能小于等于0
		return
	}

	t := new(timer)          // 创建新的timer结构
	t.when = nanotime() + ns // 获取唤醒时间
	t.f = goroutineReady     // 唤醒后执行goroutineReady
	t.arg = getg()
	lock(&timers.lock)
	addtimerLocked(t)
	goparkunlock(&timers.lock, "sleep", traceEvGoSleep, 2)
}

// startTimer adds t to the timer heap.
//go:linkname startTimer time.startTimer
func startTimer(t *timer) {
	if raceenabled {
		racerelease(unsafe.Pointer(t))
	}
	addtimer(t)
}

// stopTimer removes t from the timer heap if it is there.
// It returns true if t was removed, false if t wasn't even there.
//go:linkname stopTimer time.stopTimer
func stopTimer(t *timer) bool {
	return deltimer(t)
}

// Go runtime.

// Ready the goroutine arg.
func goroutineReady(arg interface{}, seq uintptr) {
	goready(arg.(*g), 0)
}

func addtimer(t *timer) {
	lock(&timers.lock)
	addtimerLocked(t)
	unlock(&timers.lock)
}

// Add a timer to the heap and start or kick the timer proc.
// If the new timer is earlier than any of the others.
// Timers are locked.
func addtimerLocked(t *timer) { // 在加锁之后添加定时器
	// when must never be negative; otherwise timerproc will overflow
	// during its delta calculation and never expire other runtime·timers.
	if t.when < 0 {
		t.when = 1<<63 - 1
	}
	t.i = len(timers.t)
	timers.t = append(timers.t, t)
	siftupTimer(t.i)
	if t.i == 0 {
		// siftup moved to top: new earliest deadline.
		if timers.sleeping {
			timers.sleeping = false
			notewakeup(&timers.waitnote)
		}
		if timers.rescheduling {
			timers.rescheduling = false
			goready(timers.gp, 0)
		}
	}
	if !timers.created { // 如果定时器还未创建
		timers.created = true // 设置定时器创建
		go timerproc()        // 启动定时器处理goroutine
	}
}

// Delete timer t from the heap.
// Do not need to update the timerproc: if it wakes up early, no big deal.
func deltimer(t *timer) bool {
	// Dereference t so that any panic happens before the lock is held.
	// Discard result, because t might be moving in the heap.
	_ = t.i

	lock(&timers.lock)
	// t may not be registered anymore and may have
	// a bogus i (typically 0, if generated by Go).
	// Verify it before proceeding.
	i := t.i
	last := len(timers.t) - 1
	if i < 0 || i > last || timers.t[i] != t {
		unlock(&timers.lock)
		return false
	}
	if i != last {
		timers.t[i] = timers.t[last]
		timers.t[i].i = i
	}
	timers.t[last] = nil
	timers.t = timers.t[:last]
	if i != last {
		siftupTimer(i)
		siftdownTimer(i)
	}
	unlock(&timers.lock)
	return true
}

// Timerproc runs the time-driven events.
// It sleeps until the next event in the timers heap.
// If addtimer inserts a new earlier event, addtimer1 wakes timerproc early.
func timerproc() { // 用于处理定时器的goroutine
	timers.gp = getg()        // 获取当前的goroutine
	for {
		lock(&timers.lock)
		timers.sleeping = false
		now := nanotime()
		delta := int64(-1)
		for {
			if len(timers.t) == 0 {
				delta = -1
				break
			}
			t := timers.t[0]
			delta = t.when - now
			if delta > 0 {
				break
			}
			if t.period > 0 {
				// leave in heap but adjust next time to fire
				t.when += t.period * (1 + -delta/t.period)
				siftdownTimer(0)
			} else {
				// remove from heap
				last := len(timers.t) - 1
				if last > 0 {
					timers.t[0] = timers.t[last]
					timers.t[0].i = 0
				}
				timers.t[last] = nil
				timers.t = timers.t[:last]
				if last > 0 {
					siftdownTimer(0)
				}
				t.i = -1 // mark as removed
			}
			f := t.f
			arg := t.arg
			seq := t.seq
			unlock(&timers.lock)
			if raceenabled {
				raceacquire(unsafe.Pointer(t))
			}
			f(arg, seq)
			lock(&timers.lock)
		}
		if delta < 0 || faketime > 0 {
			// No timers left - put goroutine to sleep.
			timers.rescheduling = true
			goparkunlock(&timers.lock, "timer goroutine (idle)", traceEvGoBlock, 1)
			continue
		}
		// At least one timer pending.  Sleep until then.
		timers.sleeping = true
		noteclear(&timers.waitnote)
		unlock(&timers.lock)
		notetsleepg(&timers.waitnote, delta)
	}
}

func timejump() *g {
	if faketime == 0 {
		return nil
	}

	lock(&timers.lock)
	if !timers.created || len(timers.t) == 0 {
		unlock(&timers.lock)
		return nil
	}

	var gp *g
	if faketime < timers.t[0].when {
		faketime = timers.t[0].when
		if timers.rescheduling {
			timers.rescheduling = false
			gp = timers.gp
		}
	}
	unlock(&timers.lock)
	return gp
}

// Heap maintenance algorithms.

func siftupTimer(i int) {
	t := timers.t
	when := t[i].when
	tmp := t[i]
	for i > 0 {
		p := (i - 1) / 4 // parent
		if when >= t[p].when {
			break
		}
		t[i] = t[p]
		t[i].i = i
		t[p] = tmp
		t[p].i = p
		i = p
	}
}

func siftdownTimer(i int) {
	t := timers.t
	n := len(t)
	when := t[i].when
	tmp := t[i]
	for {
		c := i*4 + 1 // left child
		c3 := c + 2  // mid child
		if c >= n {
			break
		}
		w := t[c].when
		if c+1 < n && t[c+1].when < w {
			w = t[c+1].when
			c++
		}
		if c3 < n {
			w3 := t[c3].when
			if c3+1 < n && t[c3+1].when < w3 {
				w3 = t[c3+1].when
				c3++
			}
			if w3 < w {
				w = w3
				c = c3
			}
		}
		if w >= when {
			break
		}
		t[i] = t[c]
		t[i].i = i
		t[c] = tmp
		t[c].i = c
		i = c
	}
}

// Entry points for net, time to call nanotime.

//go:linkname net_runtimeNano net.runtimeNano
func net_runtimeNano() int64 {
	return nanotime()
}

//go:linkname time_runtimeNano time.runtimeNano
func time_runtimeNano() int64 {
	return nanotime()
}