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

package archive_test

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/johnknl/alog/pkg/archive"
	"github.com/johnknl/alog/pkg/log"
)

func ExampleArchive() {
	dir, err := os.MkdirTemp("", "alog-example-archive-")
	if err != nil {
		fmt.Printf("mkdir temp: %v\n", err)
		return
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a, err := archive.New(
		ctx,
		dir,
		2,
		1024,
		log.WriteBufferOptions{MaxLength: 16, MaxSize: 4 << 10, MaxDelay: 5 * time.Millisecond},
	)
	if err != nil {
		fmt.Printf("new archive: %v\n", err)
		return
	}

	seq0, err := a.Append(ctx, []byte("one"))
	if err != nil {
		fmt.Printf("append one: %v\n", err)
		return
	}
	seq1, err := a.Append(ctx, []byte("two"))
	if err != nil {
		fmt.Printf("append two: %v\n", err)
		return
	}

	fmt.Println(seq0, seq1)

	err = a.Range(0, 0, func(seq uint64, payload []byte) error {
		fmt.Printf("%d:%s\n", seq, payload)
		return nil
	})
	if err != nil {
		fmt.Printf("range: %v\n", err)
		return
	}

	if err = a.Close(); err != nil {
		fmt.Printf("close: %v\n", err)
	}

	// Output:
	// 0 1
	// 0:one
	// 1:two
}
