// MIT License
//
// Copyright (C) 2025 John Kleijn
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE

package log_test

import (
	"fmt"
	"os"

	"github.com/johnknl/alog/pkg/log"
)

func ExampleLog_Append_andSync() {
	dir, err := os.MkdirTemp("", "alog-example-append-")
	if err != nil {
		fmt.Printf("mkdir temp: %v\n", err)
		return
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	l, err := log.Load(dir, log.DefaultOptions())
	if err != nil {
		fmt.Printf("load log: %v\n", err)
		return
	}
	defer l.Close() //nolint:errcheck

	lastSeq, err := l.Append([]byte("alpha"), []byte("beta"), []byte("gamma"))
	if err != nil {
		fmt.Printf("append: %v\n", err)
		return
	}

	if err = l.Sync(); err != nil {
		fmt.Printf("sync: %v\n", err)
		return
	}

	fmt.Println(lastSeq)

	// Output:
	// 2
}

func ExampleScanner_Seek_andStopAt() {
	dir, err := os.MkdirTemp("", "alog-example-scan-")
	if err != nil {
		fmt.Printf("mkdir temp: %v\n", err)
		return
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	l, err := log.Load(dir, log.DefaultOptions())
	if err != nil {
		fmt.Printf("load log: %v\n", err)
		return
	}
	defer l.Close() //nolint:errcheck

	for _, v := range []string{"a", "b", "c", "d", "e"} {
		if _, err = l.Append([]byte(v)); err != nil {
			fmt.Printf("append: %v\n", err)
			return
		}
	}

	s := log.NewScanner(l)
	s.Seek(2)
	s.StopAt(4)

	for s.Next() {
		seq, frame := s.Borrow()
		fmt.Printf("%d:%s\n", seq, frame.Payload)
		frame.Return()
	}
	if err = s.Err(); err != nil {
		fmt.Printf("scan: %v\n", err)
	}

	// Output:
	// 2:c
	// 3:d
}

func ExampleNewLimitedScanner() {
	dir, err := os.MkdirTemp("", "alog-example-limited-scan-")
	if err != nil {
		fmt.Printf("mkdir temp: %v\n", err)
		return
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	l, err := log.Load(dir, log.DefaultOptions())
	if err != nil {
		fmt.Printf("load log: %v\n", err)
		return
	}
	defer l.Close() //nolint:errcheck

	if _, err = l.Append([]byte("alphabet"), []byte("xy")); err != nil {
		fmt.Printf("append: %v\n", err)
		return
	}

	s := log.NewLimitedScanner(l, 3)
	for s.Next() {
		seq, frame := s.Borrow()
		fmt.Printf("%d:%s\n", seq, frame.Payload)
		frame.Return()
	}
	if err = s.Err(); err != nil {
		fmt.Printf("scan: %v\n", err)
	}

	// Output:
	// 0:alp
	// 1:xy
}
