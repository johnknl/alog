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

package wal_test

import (
	"fmt"
	"os"

	"github.com/johnknl/alog/pkg/wal"
)

// ExampleWAL_Consume shows appending records and consuming them in-order.
//
// `WAL` is the durable append plus destructive consume abstraction.
// Use it when records should be processed in order and removed after
// successful processing.
// WAL provides write-ahead semantics over the base log abstraction and exposes
// a linear consume API for replay flows.
func ExampleWAL_Consume() {
	dir, err := os.MkdirTemp("", "alog-example-wal-")
	if err != nil {
		fmt.Printf("mkdir temp: %v\n", err)
		return
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	w, err := wal.New(dir, 1024)
	if err != nil {
		fmt.Printf("new wal: %v\n", err)
		return
	}

	if _, err = w.Append([]byte("one"), []byte("two")); err != nil {
		fmt.Printf("wal append: %v\n", err)
		return
	}

	err = w.Consume(0, func(seq uint64, payload []byte) error {
		fmt.Printf("%d:%s\n", seq, payload)
		return nil
	})
	if err != nil {
		fmt.Printf("wal consume: %v\n", err)
		return
	}

	if err = w.Close(); err != nil {
		fmt.Printf("wal close: %v\n", err)
	}

	// Output:
	// 0:one
	// 1:two
}
