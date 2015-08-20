// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io

type multiReader struct { // MultiReader�ṹ��ʵ��Reader�ӿڣ���װ�˶��Reader
	readers []Reader
}

func (mr *multiReader) Read(p []byte) (n int, err error) { // ���˴�ÿ��Reader�ж�ȡ������һ���ٶ���һ��
	for len(mr.readers) > 0 {
		n, err = mr.readers[0].Read(p) // �ȶ��б��е�һ��
		if n > 0 || err != EOF {       // �������ݣ����أ�ֱ����һ�����꣬��˳����ڶ���
			if err == EOF {
				// Don't return EOF yet. There may be more bytes
				// in the remaining readers.
				err = nil
			}
			return
		}
		mr.readers = mr.readers[1:] // ��reader slice��ȥ����һ��
	}
	return 0, EOF
}

// MultiReader returns a Reader that's the logical concatenation of
// the provided input readers.  They're read sequentially.  Once all
// inputs have returned EOF, Read will return EOF.  If any of the readers
// return a non-nil, non-EOF error, Read will return that error.
func MultiReader(readers ...Reader) Reader { // ����MultiReader�ṹ
	r := make([]Reader, len(readers))
	copy(r, readers)
	return &multiReader{r}
}

type multiWriter struct { // multiWriter�ṹ�����writer�ķ�װ
	writers []Writer
}

func (t *multiWriter) Write(p []byte) (n int, err error) { // multiwriter�����˶����е�writerд
	for _, w := range t.writers { // �������е�writer��ÿ��д��p��һ���������������
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = ErrShortWrite
			return
		}
	}
	return len(p), nil
}

// MultiWriter creates a writer that duplicates its writes to all the
// provided writers, similar to the Unix tee(1) command.
func MultiWriter(writers ...Writer) Writer { // ����MultiWriter�ṹ
	w := make([]Writer, len(writers))
	copy(w, writers)
	return &multiWriter{w}
}
