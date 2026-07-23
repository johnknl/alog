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

package journal_test

import (
	"fmt"
	"os"

	"github.com/johnknl/alog/pkg/journal"
)

func ExampleJournal() {
	dir, err := os.MkdirTemp("", "alog-example-journal-")
	if err != nil {
		fmt.Printf("mkdir temp: %v\n", err)
		return
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	j, err := journal.New(dir, 1024)
	if err != nil {
		fmt.Printf("new journal: %v\n", err)
		return
	}

	lastSeq, err := j.Append([]byte("cmd-1"), []byte("cmd-2"))
	if err != nil {
		fmt.Printf("append: %v\n", err)
		return
	}

	_, err = j.Append([]byte("cmd-3"), []byte("cmd-4"))
	if err != nil {
		fmt.Printf("append: %v\n", err)
		return
	}

	if err = j.TruncateBefore(1); err != nil {
		fmt.Printf("truncate before: %v\n", err)
		return
	}

	if err = j.TruncateAfter(2); err != nil {
		fmt.Printf("truncate after: %v\n", err)
		return
	}

	if err = j.Sync(); err != nil {
		fmt.Printf("sync: %v\n", err)
		return
	}

	fmt.Println(lastSeq)
	err = j.Range(0, 0, func(seq uint64, payload []byte) error {
		fmt.Printf("%d:%s\n", seq, payload)
		return nil
	})
	if err != nil {
		fmt.Printf("range: %v\n", err)
		return
	}

	if err = j.Close(); err != nil {
		fmt.Printf("close: %v\n", err)
		return
	}

	// Output:
	// 1
	// 1:cmd-2
	// 2:cmd-3
}
