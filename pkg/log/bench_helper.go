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
//

package log

import (
	"testing"

	"github.com/johnknl/alog/internal/testutil"
)

// NewLogForBench creates a new Log instance for benchmarking purposes.
func NewLogForBench(b *testing.B, maxSegments int, maxSegmentSize int64) *Log {
	b.Helper()
	opts := DefaultOptions()
	opts.Storage.MaxSegments = maxSegments
	opts.Storage.MaxSegmentSize = maxSegmentSize

	l, err := New(opts)
	if err != nil {
		b.Fatalf("NewLog() error = %v", err)
	}
	b.Cleanup(func() { _ = l.Close() })

	if err = l.Load(b.TempDir()); err != nil {
		b.Fatalf("Load() error = %v", err)
	}

	return l
}

// SeedLogDataset seeds a log dataset with the specified number of records and payload
// size for benchmarking purposes.
func SeedLogDataset(b *testing.B, dir string, records int, payloadSize int, maxSegments int, maxSegSize int64) {
	b.Helper()
	opts := DefaultOptions()
	opts.Storage.MaxSegments = maxSegments
	opts.Storage.MaxSegmentSize = maxSegSize

	l, err := New(opts)
	if err != nil {
		b.Fatalf("NewLog() error = %v", err)
	}

	if err = l.Load(dir); err != nil {
		b.Fatalf("Load() error = %v", err)
	}

	p := testutil.BenchmarkPayload(payloadSize)
	for range records {
		if _, err = l.Append(p); err != nil {
			b.Fatalf("Log.Append seed error = %v", err)
		}
	}
	if err = l.Sync(); err != nil {
		b.Fatalf("Log.Sync seed error = %v", err)
	}
	if err = l.Close(); err != nil {
		b.Fatalf("Log.Close seed error = %v", err)
	}
}
