// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multipart

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/textproto"
	"os"
)

// TODO(adg,bradfitz): find a way to unify the DoS-prevention strategy here
// with that of the http package's ParseForm.

// ReadForm parses an entire multipart message whose parts have
// a Content-Disposition of "form-data".
// It stores up to maxMemory bytes of the file parts in memory
// and the remainder on disk in temporary files.
func (r *Reader) ReadForm(maxMemory int64) (f *Form, err error) { // ����һ��Form��maxMemory���������ڴ��У�������Ӳ����
	form := &Form{make(map[string][]string), make(map[string][]*FileHeader)} // ����From�ṹ
	defer func() {                                                           // ����defer������ִ����ɺ�����RemoveAll
		if err != nil {
			form.RemoveAll()
		}
	}()

	maxValueBytes := int64(10 << 20) // 10 MB is a lot of text.
	for {
		p, err := r.NextPart() // ���һ��part
		if err == io.EOF {     // �����������������
			break
		}
		if err != nil { // �����������
			return nil, err
		}

		name := p.FormName() // ȡ��form��name
		if name == "" {      // formû�����ƣ���������һ��Part
			continue
		}
		filename := p.FileName() // ȡ��form��filename

		var b bytes.Buffer // ����һ��byte��Buffer

		if filename == "" { // û���ļ���
			// value, store as string in memory �����ݱ������ڴ��У������ַ���
			n, err := io.CopyN(&b, p, maxValueBytes) // ��p�п������ݵ�b��
			if err != nil && err != io.EOF {         // �������ݳ�������
				return nil, err
			}
			maxValueBytes -= n
			if maxValueBytes == 0 { // �ﵽ��10M�����ƣ�������Ϣ����
				return nil, errors.New("multipart: message too large")
			}
			form.Value[name] = append(form.Value[name], b.String()) // �趨form��ֵ
			continue
		}

		// file, store in memory or on disk
		fh := &FileHeader{
			Filename: filename,
			Header:   p.Header,
		} // ����һ��FileHeader�ṹ
		n, err := io.CopyN(&b, p, maxMemory+1) // �����ݿ�����b��
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n > maxMemory { // ������ȴ���maxMemory
			// too big, write to disk and flush buffer
			file, err := ioutil.TempFile("", "multipart-") // ̫���ˣ�����һ����ʱ�ļ�
			if err != nil {
				return nil, err
			}
			defer file.Close()
			_, err = io.Copy(file, io.MultiReader(&b, p))
			if err != nil {
				os.Remove(file.Name())
				return nil, err
			}
			fh.tmpfile = file.Name()
		} else { // �ڴ��п�ȫ�����棬�����ݷ���content��
			fh.content = b.Bytes()
			maxMemory -= n
		}
		form.File[name] = append(form.File[name], fh) // �����ļ���handler
	}

	return form, nil
}

// Form is a parsed multipart form. ��ʾ��������multipart form
// Its File parts are stored either in memory or on disk, �ļ����ֻ��߱������ڴ�򱣴��ڴ�����
// and are accessible via the *FileHeader's Open method. ͨ��FileHeader��Open�������Է���
// Its Value parts are stored as strings.
// Both are keyed by field name.
type Form struct { // multipart��form�ṹ��File���ֻ��ߴ洢���ڴ��л��ߴ洢�ڴ�����
	Value map[string][]string      // value���ִ洢���ַ�����
	File  map[string][]*FileHeader // �������ڴ��У��������ļ���
}

// RemoveAll removes any temporary files associated with a Form.
func (f *Form) RemoveAll() error { // �����Form�е�������ʱ�ļ�
	var err error
	for _, fhs := range f.File { // �������е�FileHeader����
		for _, fh := range fhs { // ����FileHeader���������е�FileHeader
			if fh.tmpfile != "" { // �������ʱ�ļ�
				e := os.Remove(fh.tmpfile) // ɾ����ʱ�ļ�
				if e != nil && err == nil {
					err = e
				}
			}
		}
	}
	return err
}

// A FileHeader describes a file part of a multipart request.
type FileHeader struct { // ����һ��multipart������ļ����֣�����ֻ���ڴ���content��Ҳ�����ڴ�����
	Filename string               // �ļ���
	Header   textproto.MIMEHeader // MIMEHeader

	content []byte // ����ļ��������ڴ��У�������content��
	tmpfile string // ����ļ������ڴ����ϣ�ָ������ļ�
}

// Open opens and returns the FileHeader's associated File.
func (fh *FileHeader) Open() (File, error) { // ��FileHeader���һ���ļ��ӿ�
	if b := fh.content; b != nil { // ����������ڴ���
		r := io.NewSectionReader(bytes.NewReader(b), 0, int64(len(b))) // ����һ��sectionReader
		return sectionReadCloser{r}, nil
	}
	return os.Open(fh.tmpfile)
}

// File is an interface to access the file part of a multipart message.
// Its contents may be either stored in memory or on disk.
// If stored on disk, the File's underlying concrete type will be an *os.File.
type File interface { // File�ӿ�
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
}

// helper types to turn a []byte into a File

type sectionReadCloser struct { // Ϊsection Reader����Close
	*io.SectionReader
}

func (rc sectionReadCloser) Close() error {
	return nil
}
