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

package write_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/johnknl/alog/internal/testutil"
	"github.com/johnknl/alog/pkg/log"
	"github.com/johnknl/alog/pkg/log/write"
)

// BenchmarkWriter measures concurrent grouped-append throughput.
func BenchmarkWriter(b *testing.B) {
	b.Run("parallelism=8", func(b *testing.B) {
		l := log.NewLogForBench(b, 4, 64*1024*1024)
		ctx, cancel := testutil.ContextWithCancel(b)
		b.Cleanup(cancel)

		writer := write.StartWriter(ctx, l, log.WriteBufferOptions{
			MaxLength: 1024,
			MaxSize:   4 * testutil.OneMiB,
			MaxDelay:  100 * time.Microsecond,
		})

		payload := testutil.BenchmarkPayload(16)
		b.SetBytes(int64(len(payload)))
		b.SetParallelism(8)

		var (
			errMu    sync.Mutex
			firstErr error
		)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if _, err := writer.Append(ctx, payload); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					return
				}
			}
		})
		if firstErr != nil {
			b.Fatalf("Writer.Append() error = %v", firstErr)
		}
		testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
	})
}

// BenchmarkWriterScaling measures writer throughput as producer concurrency grows.
func BenchmarkWriterScaling(b *testing.B) {
	for _, p := range []int{1, 2, 4, 8, 16, 32} {
		b.Run(fmt.Sprintf("parallelism=%d", p), func(b *testing.B) {
			l := log.NewLogForBench(b, 4, 64*1024*1024)
			ctx, cancel := testutil.ContextWithCancel(b)
			b.Cleanup(cancel)
			w := write.StartWriter(ctx, l, log.WriteBufferOptions{
				MaxLength: 1024,
				MaxSize:   4 * testutil.OneMiB,
				MaxDelay:  100 * time.Microsecond,
			})

			payload := testutil.BenchmarkPayload(16)
			b.SetBytes(int64(len(payload)))
			b.SetParallelism(p)
			var (
				errMu    sync.Mutex
				firstErr error
			)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, err := w.Append(ctx, payload); err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
						return
					}
				}
			})
			if firstErr != nil {
				b.Fatalf("Writer.Append() error = %v", firstErr)
			}
			testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
		})
	}
}

// BenchmarkWriterPayload measures writer throughput across payload sizes.
func BenchmarkWriterPayload(b *testing.B) {
	for _, c := range []struct {
		name string
		size int
	}{{"16B", 16}, {"256B", 256}, {"4KiB", 4 * testutil.OneKiB}, {"64KiB", 64 * testutil.OneKiB}, {"1MiB", testutil.OneMiB}} {
		b.Run(c.name, func(b *testing.B) {
			l := log.NewLogForBench(b, 4, 256*1024*1024)
			ctx, cancel := testutil.ContextWithCancel(b)
			b.Cleanup(cancel)
			w := write.StartWriter(ctx, l, log.WriteBufferOptions{
				MaxLength: 64,
				MaxSize:   64 * testutil.OneMiB,
				MaxDelay:  250 * time.Microsecond,
			})
			payload := testutil.BenchmarkPayload(c.size)
			b.SetBytes(int64(len(payload)))
			b.SetParallelism(8)
			var (
				errMu    sync.Mutex
				firstErr error
			)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, err := w.Append(ctx, payload); err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
						return
					}
				}
			})
			if firstErr != nil {
				b.Fatalf("Writer.Append() error = %v", firstErr)
			}
			testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
		})
	}
}

// BenchmarkWriterMaxLength measures batching behavior as MaxLength varies.
func BenchmarkWriterMaxLength(b *testing.B) {
	for _, maxLen := range []int{1, 8, 64, 1024} {
		b.Run(testutil.BenchmarkLabelInt("MaxLength", maxLen), func(b *testing.B) {
			l := log.NewLogForBench(b, 4, 64*1024*1024)
			ctx, cancel := testutil.ContextWithCancel(b)
			b.Cleanup(cancel)
			w := write.StartWriter(ctx, l, log.WriteBufferOptions{
				MaxLength: maxLen,
				MaxSize:   4 * testutil.OneMiB,
				MaxDelay:  100 * time.Millisecond,
			})
			payload := testutil.BenchmarkPayload(256)
			b.SetBytes(int64(len(payload)))
			b.SetParallelism(8)
			var (
				errMu    sync.Mutex
				firstErr error
			)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, err := w.Append(ctx, payload); err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
						return
					}
				}
			})
			if firstErr != nil {
				b.Fatalf("Writer.Append() error = %v", firstErr)
			}
			testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
		})
	}
}
