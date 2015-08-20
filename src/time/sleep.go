// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

// Sleep pauses the current goroutine for at least the duration d.
// A negative or zero duration causes Sleep to return immediately.
func Sleep(d Duration) // ˯��ָ����ʱ��

// runtimeNano returns the current value of the runtime clock in nanoseconds.
func runtimeNano() int64 // ���������ʽ���ص�ǰ������ʱʱ��

// Interface to timers implemented in package runtime.
// Must be in sync with ../runtime/runtime.h:/^struct.Timer$
type runtimeTimer struct { // ����һ������ʱtimer�ṹ����runtime�е�Timer�ṹ��ͬ
	i      int
	when   int64                      // timer��ʱ����
	period int64                      // ����೤ʱ�䣬������
	f      func(interface{}, uintptr) // NOTE: must not be closure
	arg    interface{}                // ����ִ�к����Ĳ���
	seq    uintptr
}

// when is a helper function for setting the 'when' field of a runtimeTimer.
// It returns what the time will be, in nanoseconds, Duration d in the future.
// If d is negative, it is ignored.  If the returned value would be less than
// zero because of an overflow, MaxInt64 is returned.
func when(d Duration) int64 { // ����dʱ���Ժ��Ӧ������ʱ��
	if d <= 0 { // ���dС�ڵ���0�����ص�ǰʱ��
		return runtimeNano()
	}
	t := runtimeNano() + int64(d)
	if t < 0 {
		t = 1<<63 - 1 // math.MaxInt64
	}
	return t
}

func startTimer(*runtimeTimer)
func stopTimer(*runtimeTimer) bool

// The Timer type represents a single event.
// When the Timer expires, the current time will be sent on C,
// unless the Timer was created by AfterFunc.
// A Timer must be created with NewTimer or AfterFunc.
type Timer struct { // ��ʱ���ṹ
	C <-chan Time  // ����֪ͨ��chan�����͵�����ΪTime
	r runtimeTimer // ����һ��runtimeTimer�ṹ
}

// Stop prevents the Timer from firing.
// It returns true if the call stops the timer, false if the timer has already
// expired or been stopped.
// Stop does not close the channel, to prevent a read from the channel succeeding
// incorrectly.
func (t *Timer) Stop() bool {
	if t.r.f == nil {
		panic("time: Stop called on uninitialized Timer")
	}
	return stopTimer(&t.r)
}

// NewTimer creates a new Timer that will send
// the current time on its channel after at least duration d.
func NewTimer(d Duration) *Timer { // ����һ���µĶ�ʱ�������ں���chan����ʱ�䣬ִֻ��һ��
	c := make(chan Time, 1) // ����chan
	t := &Timer{            // ����Timer
		C: c,
		r: runtimeTimer{
			when: when(d),  // ��ʱ����
			f:    sendTime, // ���ں�ִ�еĺ��������ں���chan�з��͵�ǰʱ��
			arg:  c,        // ִ�к����Ĳ���
		},
	}
	startTimer(&t.r) // ���붨ʱ��
	return t
}

// Reset changes the timer to expire after duration d.
// It returns true if the timer had been active, false if the timer had
// expired or been stopped.
func (t *Timer) Reset(d Duration) bool { // ���ö�ʱ����dʱ�����Ч
	if t.r.f == nil {
		panic("time: Reset called on uninitialized Timer")
	}
	w := when(d)
	active := stopTimer(&t.r)
	t.r.when = w // ������ʱ��
	startTimer(&t.r)
	return active
}

func sendTime(c interface{}, seq uintptr) {
	// Non-blocking send of time on c.
	// Used in NewTimer, it cannot block anyway (buffer).
	// Used in NewTicker, dropping sends on the floor is
	// the desired behavior when the reader gets behind,
	// because the sends are periodic.
	select {
	case c.(chan Time) <- Now(): // ����ǰ��ʱ�䷢�͸�chan����������򲻷���
	default:
	}
}

// After waits for the duration to elapse and then sends the current time
// on the returned channel.
// It is equivalent to NewTimer(d).C.
func After(d Duration) <-chan Time { // �´���һ����ʱ����������chan���ȴ�dʱ����͵�ǰʱ�䵽chan��
	return NewTimer(d).C // �½�һ��timer������chan
}

// AfterFunc waits for the duration to elapse and then calls f
// in its own goroutine. It returns a Timer that can
// be used to cancel the call using its Stop method.
func AfterFunc(d Duration, f func()) *Timer { // ��ʱ���ִ�к���f
	t := &Timer{ // �´���һ��Timer
		r: runtimeTimer{
			when: when(d),
			f:    goFunc, // ��ʱ��ִ��goFunc������Ϊf
			arg:  f,
		},
	}
	startTimer(&t.r) // ִ�и�Timer
	return t
}

func goFunc(arg interface{}, seq uintptr) {
	go arg.(func())()
}
