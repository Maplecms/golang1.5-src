// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package runtime

import "unsafe"

func epollcreate(size int32) int32
func epollcreate1(flags int32) int32

//go:noescape
func epollctl(epfd, op, fd int32, ev *epollevent) int32

//go:noescape
func epollwait(epfd int32, ev *epollevent, nev, timeout int32) int32
func closeonexec(fd int32)

var (
	epfd           int32 = -1 // epoll descriptor ȫ�ֵ�epoll������
	netpolllasterr int32
)

func netpollinit() { // ��ʼ��epoll,����epoll���������close_on_exec
	epfd = epollcreate1(_EPOLL_CLOEXEC)
	if epfd >= 0 {
		return
	}
	epfd = epollcreate(1024)
	if epfd >= 0 {
		closeonexec(epfd)
		return
	}
	println("netpollinit: failed to create epoll descriptor", -epfd)
	throw("netpollinit: failed to create descriptor")
}

func netpollopen(fd uintptr, pd *pollDesc) int32 { // ��fd���
	var ev epollevent
	ev.events = _EPOLLIN | _EPOLLOUT | _EPOLLRDHUP | _EPOLLET // ���ü����¼�
	*(**pollDesc)(unsafe.Pointer(&ev.data)) = pd              // ��pollDesc���õ�ev.data��
	return -epollctl(epfd, _EPOLL_CTL_ADD, int32(fd), &ev)    // ִ��epoll_ctl���������¼�
}

func netpollclose(fd uintptr) int32 { // ɾ��fd��Ӧ��epoll�¼�
	var ev epollevent
	return -epollctl(epfd, _EPOLL_CTL_DEL, int32(fd), &ev)
}

func netpollarm(pd *pollDesc, mode int) {
	throw("unused")
}

// polls for ready network connections
// returns list of goroutines that become runnable
func netpoll(block bool) *g {
	if epfd == -1 { // û������epoll���������
		return nil
	}
	waitms := int32(-1) // ��������ȴ������ȴ�ʱ������Ϊ-1����������
	if !block {         // ��������������̷���
		waitms = 0 // ������������
	}
	var events [128]epollevent // ���ȴ�128���¼�
retry:
	n := epollwait(epfd, &events[0], int32(len(events)), waitms) // ִ��epoll_wait
	if n < 0 {                                                   // ִ�г���
		if n != -_EINTR && n != netpolllasterr {
			netpolllasterr = n
			println("runtime: epollwait on fd", epfd, "failed with", -n)
		}
		goto retry // ���۳���ʲô������ת��retryִ��,�����ǳ���EINTR������ӡһ����ʾ
	}
	var gp guintptr
	for i := int32(0); i < n; i++ { // �������е��¼�
		ev := &events[i]    // ȡ���¼��ṹ
		if ev.events == 0 { // ���δ�����¼���������һ��
			continue
		}
		var mode int32                                                 // ����ģʽ���ö�����д
		if ev.events&(_EPOLLIN|_EPOLLRDHUP|_EPOLLHUP|_EPOLLERR) != 0 { // ��������˶��¼�
			mode += 'r'
		}
		if ev.events&(_EPOLLOUT|_EPOLLHUP|_EPOLLERR) != 0 { // ���������д�¼�
			mode += 'w'
		}
		if mode != 0 {
			pd := *(**pollDesc)(unsafe.Pointer(&ev.data))                // ȡ����pollDesc�ṹ

			netpollready(&gp, pd, mode)
		}
	}
	if block && gp == 0 {
		goto retry
	}
	return gp.ptr()
}
