// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hash provides interfaces for hash functions.
package hash

import "io"

// Hash is the common interface implemented by all hash functions.
type Hash interface { //Hash�ӿ�
	// Write (via the embedded io.Writer interface) adds more data to the running hash.
	// It never returns an error.
	io.Writer // ����io.Wirter�ӿ�

	// Sum appends the current hash to b and returns the resulting slice.
	// It does not change the underlying hash state.
	Sum(b []byte) []byte // ����Hash�������ֵ�������׷�ӵ�b֮��

	// Reset resets the Hash to its initial state.
	Reset() // ����HashΪ��ʼ״̬

	// Size returns the number of bytes Sum will return.
	Size() int // ����Sum����󷵻ص��ֽ���

	// BlockSize returns the hash's underlying block size.
	// The Write method must be able to accept any amount
	// of data, but it may operate more efficiently if all writes
	// are a multiple of the block size.
	BlockSize() int // ���ؼ����Ĵ�С
}

// Hash32 is the common interface implemented by all 32-bit hash functions.
type Hash32 interface { // Hash32�ӿ�
	Hash
	Sum32() uint32
}

// Hash64 is the common interface implemented by all 64-bit hash functions.
type Hash64 interface { // Hash64�ӿ�
	Hash
	Sum64() uint64
}
