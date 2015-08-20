// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"runtime"
	"sync/atomic"
	"syscall"
)

// Process stores the information about a process created by StartProcess.
type Process struct { // ���̽ṹ
	Pid    int // ����pid
	handle uintptr // handle is accessed atomically on Windows
	isdone uint32  // process has been successfully waited on, non zero if true
}

func newProcess(pid int, handle uintptr) *Process { // �´���һ�����̽ṹ
	p := &Process{Pid: pid, handle: handle}
	runtime.SetFinalizer(p, (*Process).Release)
	return p
}

func (p *Process) setDone() { // ����process�Ѿ�ִ�����
	atomic.StoreUint32(&p.isdone, 1)
}

func (p *Process) done() bool { // �鿴process�Ƿ�ִ�����
	return atomic.LoadUint32(&p.isdone) > 0
}

// ProcAttr holds the attributes that will be applied to a new process
// started by StartProcess.
type ProcAttr struct { // ��������
	// If Dir is non-empty, the child changes into the directory before
	// creating the process.
	Dir string // ���Ŀ¼�ǿգ�����������ǰ���뵽��Ŀ¼
	// If Env is non-nil, it gives the environment variables for the
	// new process in the form returned by Environ.
	// If it is nil, the result of Environ will be used.
	Env []string // �������̵Ļ�������
	// Files specifies the open files inherited by the new process.  The
	// first three entries correspond to standard input, standard output, and
	// standard error.  An implementation may support additional entries,
	// depending on the underlying operating system.  A nil entry corresponds
	// to that file being closed when the process starts.
	Files []*File // ���½��̼̳еĴ��ļ��б�ǰ�����Ӧ��׼���룬��׼����ͱ�׼����

	// Operating system-specific process creation attributes.
	// Note that setting this field means that your program
	// may not execute properly or even compile on some
	// operating systems.
	Sys *syscall.SysProcAttr // ����ϵͳ�ض��Ľ��̴�������
}

// A Signal represents an operating system signal.
// The usual underlying implementation is operating system-dependent:
// on Unix it is syscall.Signal.
type Signal interface { // �������ϵͳ���ź�
	String() string
	Signal() // to distinguish from other Stringers
}

// Getpid returns the process id of the caller.
func Getpid() int { return syscall.Getpid() } //���ؽ���id

// Getppid returns the process id of the caller's parent.
func Getppid() int { return syscall.Getppid() } // ���ؽ���parent id
