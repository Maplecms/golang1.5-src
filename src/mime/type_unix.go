// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package mime

import (
	"bufio"
	"os"
	"strings"
)

func init() {
	osInitMime = initMimeUnix
}

var typeFiles = []string{
	"/etc/mime.types",
	"/etc/apache2/mime.types",
	"/etc/apache/mime.types",
}

func loadMimeFile(filename string) { // װ��MIME�ļ�
	f, err := os.Open(filename) // ���ļ�
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f) // ����һ��Scanner��ȱʡΪScanLines
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) <= 1 || fields[0][0] == '#' { // �Թ����к�ע����
			continue
		}
		mimeType := fields[0]            // ���mime����
		for _, ext := range fields[1:] { // �����չ��
			if ext[0] == '#' {
				break
			}
			setExtensionType("."+ext, mimeType) // ������չ����mime����
		}
	}
	if err := scanner.Err(); err != nil { // �������ļ��������ˣ�panic
		panic(err)
	}
}

func initMimeUnix() {
	for _, filename := range typeFiles { // װ��ÿ��Mime�ļ�
		loadMimeFile(filename)
	}
}

func initMimeForTests() map[string]string {
	typeFiles = []string{"testdata/test.types"}
	return map[string]string{
		".T1":  "application/test",
		".t2":  "text/test; charset=utf-8",
		".png": "image/png",
	}
}
