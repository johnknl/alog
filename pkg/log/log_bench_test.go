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

package log_test

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/johnknl/alog/internal/testutil"
	"github.com/johnknl/alog/pkg/log"
)

// BenchmarkAppend measures buffered append throughput for representative sizes.
func BenchmarkLog_Append(b *testing.B) {
	for _, c := range []struct {
		name string
		size int
	}{{"16B", 16}, {"1MiB", testutil.OneMiB}} {
		// BenchmarkAppend/16B measures buffered single-record append throughput with a 16-byte payload.
		// BenchmarkAppend/1MiB measures buffered single-record append throughput with a 1 MiB payload.
		b.Run(c.name, func(b *testing.B) {
			maxSeg := int64(64 * 1024 * 1024)
			if c.size >= testutil.OneMiB {
				maxSeg = 256 * 1024 * 1024
			}
			l := log.NewLogForBench(b, 4, maxSeg)
			payload := testutil.BenchmarkPayload(c.size)
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := l.Append(payload); err != nil {
					b.Fatalf("Log.Append() error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
		})
	}
}

// BenchmarkAppendSync measures append+sync durability cost.
func BenchmarkLog_AppendSync(b *testing.B) {
	// BenchmarkAppendSync/16B measures append-plus-sync durability cost with a 16-byte payload.
	b.Run("16B", func(b *testing.B) {
		l := log.NewLogForBench(b, 4, 64*1024*1024)
		payload := testutil.BenchmarkPayload(16)
		b.SetBytes(int64(len(payload)))
		b.ResetTimer()
		for b.Loop() {
			if _, err := l.Append(payload); err != nil {
				b.Fatalf("Log.Append() error = %v", err)
			}
			if err := l.Sync(); err != nil {
				b.Fatalf("Log.Sync() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
	})
}

// BenchmarkAppendBatch measures explicit batching efficiency.
func BenchmarkLog_AppendBatch(b *testing.B) {
	// BenchmarkAppendBatch/records=64 measures explicit 64-record batch append throughput with 256-byte payloads.
	b.Run("records=64", func(b *testing.B) {
		l := log.NewLogForBench(b, 4, 64*1024*1024)
		payload := testutil.BenchmarkPayload(256)
		batch := make([][]byte, 64)
		for i := range 64 {
			batch[i] = payload
		}
		b.SetBytes(int64(64 * len(payload)))
		b.ResetTimer()
		for b.Loop() {
			if _, err := l.Append(batch...); err != nil {
				b.Fatalf("Log.Append(batch) error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 64, int64(64*len(payload)))
	})
}

// BenchmarkScan measures small-record sequential scan throughput.
func BenchmarkLog_Scan(b *testing.B) {
	// BenchmarkScan/records=10K measures full sequential scan throughput over 10,000 records with 16-byte payloads.
	b.Run("records=10K", func(b *testing.B) {
		l := log.NewLogForBench(b, 0, 64*1024*1024)
		payload := testutil.BenchmarkPayload(16)
		for range 10_000 {
			if _, err := l.Append(payload); err != nil {
				b.Fatalf("Log.Append preload error = %v", err)
			}
		}
		b.SetBytes(int64(10_000 * len(payload)))
		b.ResetTimer()
		for b.Loop() {
			s := log.NewScanner(l)
			for s.Next() {
				_, f := s.Borrow()
				f.Return()
			}
			if err := s.Err(); err != nil {
				b.Fatalf("scanner error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 10_000, int64(10_000*len(payload)))
	})

	// BenchmarkScan/size=1MiB measures full sequential scan bandwidth over 64 records with 1 MiB payloads.
	b.Run("size=1MiB", func(b *testing.B) {
		l := log.NewLogForBench(b, 4, 256*1024*1024)
		payload := testutil.BenchmarkPayload(testutil.OneMiB)
		for range 64 {
			if _, err := l.Append(payload); err != nil {
				b.Fatalf("Log.Append preload error = %v", err)
			}
		}
		b.SetBytes(int64(64 * len(payload)))
		b.ResetTimer()
		for b.Loop() {
			s := log.NewScanner(l)
			for s.Next() {
				_, f := s.Borrow()
				f.Return()
			}
			if err := s.Err(); err != nil {
				b.Fatalf("scanner error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 64, int64(64*len(payload)))
	})
}

// BenchmarkLog_Range measures bounded range-read throughput over a fixed window.
func BenchmarkLog_Range(b *testing.B) {
	b.Run("records=50K/window=5K", func(b *testing.B) {
		l := log.NewLogForBench(b, 0, 64*1024*1024)
		payload := testutil.BenchmarkPayload(16)
		for range 50_000 {
			if _, err := l.Append(payload); err != nil {
				b.Fatalf("Log.Append preload error = %v", err)
			}
		}

		const start = uint64(20_000)
		const end = uint64(25_000)
		b.SetBytes(int64((end - start) * uint64(len(payload))))
		b.ResetTimer()
		for b.Loop() {
			err := l.Range(start, end, func(_ uint64, _ []byte) error { return nil })
			if err != nil {
				b.Fatalf("Log.Range() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, int64(end-start), int64((end-start)*uint64(len(payload))))
	})
}

// BenchmarkSeek measures deep seek cost in a 100K-record segment.
func BenchmarkLog_Seek(b *testing.B) {
	// BenchmarkSeek/records=100K/middle measures seek-to-middle cost in a 100,000-record dataset.
	b.Run("records=100K/middle", func(b *testing.B) {
		l := log.NewLogForBench(b, 0, 128*1024*1024)
		payload := testutil.BenchmarkPayload(16)
		for range 100_000 {
			if _, err := l.Append(payload); err != nil {
				b.Fatalf("Log.Append preload error = %v", err)
			}
		}

		target := uint64(50_000)
		b.ResetTimer()
		for b.Loop() {
			s := log.NewScanner(l)
			s.Seek(target)
			if !s.Next() {
				b.Fatalf("seek next false: %v", s.Err())
			}
			seq, f := s.Borrow()
			if seq != target {
				b.Fatalf("seq mismatch: got=%d want=%d", seq, target)
			}
			f.Return()
			if err := s.Err(); err != nil {
				b.Fatalf("scanner error = %v", err)
			}
		}
	})
}

func BenchmarkLog_Load(b *testing.B) {
	b.Run("records=100K", func(b *testing.B) {
		dir := b.TempDir()
		log.SeedLogDataset(b, dir, 100_000, 16, 0, 64*1024*1024)

		opts := log.DefaultOptions()
		opts.Storage.MaxSegments = 0
		opts.Storage.MaxSegmentSize = 64 * 1024 * 1024

		b.ResetTimer()
		for b.Loop() {
			log, err := log.Load(dir, opts)
			if err != nil {
				b.Fatalf("Load() error = %v", err)
			}
			if err = log.Close(); err != nil {
				b.Fatalf("Log.Close() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 100_000, int64(100_000*16))
	})
}

func BenchmarkLog_Consume(b *testing.B) {
	b.Run("consume/batch=1", func(b *testing.B) {
		log, err := log.Load(
			b.TempDir(),
			log.Options{
				Storage: log.StorageOptions{
					MaxSegments:    0,
					MaxSegmentSize: 64 * 1024 * 1024,
				},
			},
		)

		if err != nil {
			b.Fatalf("NewWAL() error = %v", err)
		}
		b.Cleanup(func() { _ = log.Close() })
		payload := testutil.BenchmarkPayload(16)
		b.SetBytes(int64(len(payload)))

		for range 100_000 {
			if _, err = log.Append(payload); err != nil {
				b.Fatalf("Log.Append preload error = %v", err)
			}
		}

		b.ResetTimer()
		for b.Loop() {
			err = log.Consume(0, func(_ uint64, _ []byte) error { return nil })
			if err != nil {
				b.Fatalf("WAL.Consume() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
	})
}

// BenchmarkRotate isolates append-at-rotation-boundary cost.
func BenchmarkLog_Rotate(b *testing.B) {
	// BenchmarkRotate/append-rotate measures append cost at a forced segment-rotation boundary.
	b.Run("append-rotate", func(b *testing.B) {
		payload := testutil.BenchmarkPayload(256)
		maxSeg := int64(64 + 16 + len(payload))
		l := log.NewLogForBench(b, 0, maxSeg)
		if _, err := l.Append(payload); err != nil {
			b.Fatalf("seed append error = %v", err)
		}
		b.SetBytes(int64(len(payload)))
		b.ResetTimer()
		for b.Loop() {
			if _, err := l.Append(payload); err != nil {
				b.Fatalf("Log.Append() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
	})
}

// BenchmarkAppendMatrix measures direct append across the full canonical payload matrix.
func BenchmarkLog_AppendMatrix(b *testing.B) {
	for _, c := range []struct {
		name string
		size int
	}{{"0B", 0}, {"16B", 16}, {"256B", 256}, {"4KiB", 4 * testutil.OneKiB}, {"64KiB", 64 * testutil.OneKiB}, {"1MiB", testutil.OneMiB}} {
		b.Run(c.name, func(b *testing.B) {
			maxSeg := int64(64 * 1024 * 1024)
			if c.size >= testutil.OneMiB {
				maxSeg = 256 * 1024 * 1024
			}
			l := log.NewLogForBench(b, 4, maxSeg)
			payload := testutil.BenchmarkPayload(c.size)
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := l.Append(payload); err != nil {
					b.Fatalf("Log.Append() error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
		})
	}
}

// BenchmarkAppendSyncMatrix measures append+sync across the durability matrix.
func BenchmarkLog_AppendSyncMatrix(b *testing.B) {
	for _, c := range []struct {
		name string
		size int
	}{{"16B", 16}, {"4KiB", 4 * testutil.OneKiB}, {"1MiB", testutil.OneMiB}} {
		b.Run(c.name, func(b *testing.B) {
			l := log.NewLogForBench(b, 4, 256*1024*1024)
			payload := testutil.BenchmarkPayload(c.size)
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := l.Append(payload); err != nil {
					b.Fatalf("Log.Append() error = %v", err)
				}
				if err := l.Sync(); err != nil {
					b.Fatalf("Log.Sync() error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, 1, int64(len(payload)))
		})
	}
}

// BenchmarkAppendBatchMatrix measures explicit batching without writer coordination.
func BenchmarkLog_AppendBatchMatrix(b *testing.B) {
	for _, n := range []int{1, 8, 64, 1024} {
		b.Run(testutil.BenchmarkLabelInt("records", n), func(b *testing.B) {
			l := log.NewLogForBench(b, 4, 64*1024*1024)
			payload := testutil.BenchmarkPayload(256)
			batch := make([][]byte, n)
			for i := range n {
				batch[i] = payload
			}
			b.SetBytes(int64(n * len(payload)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := l.Append(batch...); err != nil {
					b.Fatalf("Log.Append(batch) error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, int64(n), int64(n*len(payload)))
		})
	}
}

// BenchmarkAppendBatchSync measures explicit batching with one sync per batch.
func BenchmarkLog_AppendBatchSync(b *testing.B) {
	for _, n := range []int{1, 8, 64, 1024} {
		b.Run(testutil.BenchmarkLabelInt("records", n), func(b *testing.B) {
			l := log.NewLogForBench(b, 4, 64*1024*1024)
			payload := testutil.BenchmarkPayload(256)
			batch := make([][]byte, n)
			for i := range n {
				batch[i] = payload
			}
			b.SetBytes(int64(n * len(payload)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := l.Append(batch...); err != nil {
					b.Fatalf("Log.Append(batch) error = %v", err)
				}
				if err := l.Sync(); err != nil {
					b.Fatalf("Log.Sync() error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, int64(n), int64(n*len(payload)))
		})
	}
}

// BenchmarkScanMatrix measures small-record scan scaling over record counts.
func BenchmarkLog_ScanMatrix(b *testing.B) {
	for _, records := range []int{1_000, 10_000, 100_000, 1_000_000} {
		b.Run(testutil.BenchmarkLabelInt("records", records), func(b *testing.B) {
			l := log.NewLogForBench(b, 0, 256*1024*1024)
			payload := testutil.BenchmarkPayload(16)
			for range records {
				if _, err := l.Append(payload); err != nil {
					b.Fatalf("Log.Append preload error = %v", err)
				}
			}
			b.SetBytes(int64(records * len(payload)))
			b.ResetTimer()
			for b.Loop() {
				s := log.NewScanner(l)
				for s.Next() {
					_, f := s.Borrow()
					f.Return()
				}
				if err := s.Err(); err != nil {
					b.Fatalf("scanner error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, int64(records), int64(records*len(payload)))
		})
	}
}

// BenchmarkScanLargeMatrix measures large-record scan bandwidth.
func BenchmarkLog_ScanLargeMatrix(b *testing.B) {
	for _, c := range []struct {
		name    string
		records int
		size    int
	}{{"4KiB", 16_384, 4 * testutil.OneKiB}, {"64KiB", 1_024, 64 * testutil.OneKiB}, {"1MiB", 64, testutil.OneMiB}} {
		b.Run(c.name, func(b *testing.B) {
			l := log.NewLogForBench(b, 0, 256*1024*1024)
			payload := testutil.BenchmarkPayload(c.size)
			for range c.records {
				if _, err := l.Append(payload); err != nil {
					b.Fatalf("Log.Append preload error = %v", err)
				}
			}
			b.SetBytes(int64(c.records * c.size))
			b.ResetTimer()
			for b.Loop() {
				s := log.NewScanner(l)
				for s.Next() {
					_, f := s.Borrow()
					f.Return()
				}
				if err := s.Err(); err != nil {
					b.Fatalf("scanner error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, int64(c.records), int64(c.records*c.size))
		})
	}
}

// BenchmarkScanRangeMatrix measures seek/setup vs scan cost as range width changes.
func BenchmarkLog_ScanRangeMatrix(b *testing.B) {
	l := log.NewLogForBench(b, 0, 64*1024*1024)
	p := testutil.BenchmarkPayload(16)
	for range 60_000 {
		if _, err := l.Append(p); err != nil {
			b.Fatalf("Log.Append preload error = %v", err)
		}
	}
	for _, width := range []uint64{1, 10, 100, 5_000, 50_000} {
		b.Run(testutil.BenchmarkLabelInt("range", int(width)), func(b *testing.B) {
			start := uint64(5_000)
			end := start + width
			b.SetBytes(int64(width) * int64(len(p)))
			b.ResetTimer()
			for b.Loop() {
				err := l.Range(start, end, func(_ uint64, _ []byte) error { return nil })
				if err != nil {
					b.Fatalf("Range error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, int64(width), int64(width)*int64(len(p)))
		})
	}
}

// BenchmarkSeekPositions measures seek position scaling within one segment.
func BenchmarkLog_SeekPositions(b *testing.B) {
	for _, records := range []int{10_000, 100_000, 1_000_000} {
		b.Run(testutil.BenchmarkLabelInt("records", records), func(b *testing.B) {
			l := log.NewLogForBench(b, 0, 512*1024*1024)
			p := testutil.BenchmarkPayload(16)
			for range records {
				if _, err := l.Append(p); err != nil {
					b.Fatalf("append preload: %v", err)
				}
			}
			positions := map[string]uint64{
				"first":          0,
				"q25":            uint64(records / 4),
				"middle":         uint64(records / 2),
				"q75":            uint64((records * 3) / 4),
				"last":           uint64(records - 1),
				"exclusive-tail": uint64(records),
			}
			for label, target := range positions {
				b.Run(label, func(b *testing.B) {
					b.ResetTimer()
					for b.Loop() {
						s := log.NewScanner(l)
						s.Seek(target)
						if target < uint64(records) {
							if !s.Next() {
								b.Fatalf("seek next false: %v", s.Err())
							}
							_, f := s.Borrow()
							f.Return()
						}
						if err := s.Err(); err != nil {
							b.Fatalf("scanner error: %v", err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkSeekSegments measures seek overhead as segment count increases.
func BenchmarkLog_SeekSegments(b *testing.B) {
	const totalRecords = 100_000
	payload := testutil.BenchmarkPayload(16)
	for _, segments := range []int{1, 4, 16, 64, 256} {
		b.Run(testutil.BenchmarkLabelInt("segments", segments), func(b *testing.B) {
			frameSize := int64(16 + len(payload))
			totalBytes := int64(totalRecords)*frameSize + 64
			maxSeg := max(totalBytes/int64(segments), frameSize+64)
			l := log.NewLogForBench(b, 0, maxSeg)
			for range totalRecords {
				if _, err := l.Append(payload); err != nil {
					b.Fatalf("append preload: %v", err)
				}
			}
			target := uint64(totalRecords / 2)
			b.ResetTimer()
			for b.Loop() {
				s := log.NewScanner(l)
				s.Seek(target)
				if !s.Next() {
					b.Fatalf("seek next false: %v", s.Err())
				}
				_, f := s.Borrow()
				f.Return()
				if err := s.Err(); err != nil {
					b.Fatalf("scanner error: %v", err)
				}
			}
		})
	}
}

// BenchmarkSeekRandom measures random seek workloads with deterministic random targets.
func BenchmarkLog_SeekRandom(b *testing.B) {
	const totalRecords = 200_000
	l := log.NewLogForBench(b, 0, 256*1024*1024)
	p := testutil.BenchmarkPayload(16)
	for range totalRecords {
		if _, err := l.Append(p); err != nil {
			b.Fatalf("append preload: %v", err)
		}
	}
	targets := testutil.RandomSeqs(8192, totalRecords)
	i := 0
	b.ResetTimer()
	for b.Loop() {
		t := targets[i%len(targets)]
		i++
		s := log.NewScanner(l)
		s.Seek(t)
		if !s.Next() {
			b.Fatalf("seek next false: %v", s.Err())
		}
		_, f := s.Borrow()
		f.Return()
		if err := s.Err(); err != nil {
			b.Fatalf("scanner error: %v", err)
		}
	}
}

// BenchmarkLoadMatrix measures startup load cost by record count.
func BenchmarkLog_LoadMatrix(b *testing.B) {
	for _, records := range []int{10_000, 100_000, 1_000_000} {
		b.Run(testutil.BenchmarkLabelInt("records", records), func(b *testing.B) {
			dir := b.TempDir()
			log.SeedLogDataset(b, dir, records, 16, 0, 256*1024*1024)
			opts := log.DefaultOptions()
			opts.Storage.MaxSegments = 0
			opts.Storage.MaxSegmentSize = 256 * 1024 * 1024
			b.ResetTimer()
			for b.Loop() {
				l, err := log.Load(dir, opts)
				if err != nil {
					b.Fatalf("Load() error = %v", err)
				}
				if err = l.Close(); err != nil {
					b.Fatalf("Close() error = %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, int64(records), int64(records*16))
		})
	}
}

// BenchmarkLoadSegments measures startup cost as segment count increases.
func BenchmarkLog_LoadSegments(b *testing.B) {
	const totalRecords = 100_000
	for _, segments := range []int{1, 4, 16, 64, 256, 1024} {
		b.Run(testutil.BenchmarkLabelInt("segments", segments), func(b *testing.B) {
			dir := b.TempDir()
			frameSize := int64(16 + 16)
			totalBytes := int64(totalRecords)*frameSize + 64
			maxSeg := max(totalBytes/int64(segments), frameSize+64)
			log.SeedLogDataset(b, dir, totalRecords, 16, 0, maxSeg)
			opts := log.DefaultOptions()
			opts.Storage.MaxSegments = 0
			opts.Storage.MaxSegmentSize = maxSeg
			b.ResetTimer()
			for b.Loop() {
				l, err := log.Load(dir, opts)
				if err != nil {
					b.Fatalf("Load() error = %v", err)
				}
				if err = l.Close(); err != nil {
					b.Fatalf("Close() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkRotateExtended measures steady, rotate, reaping, and frequent rotation paths.
func BenchmarkLog_RotateExtended(b *testing.B) {
	b.Run("append-steady-state", func(b *testing.B) {
		l := log.NewLogForBench(b, 0, 64*1024*1024)
		p := testutil.BenchmarkPayload(256)
		b.SetBytes(int64(len(p)))
		b.ResetTimer()
		for b.Loop() {
			if _, err := l.Append(p); err != nil {
				b.Fatalf("Log.Append() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 1, int64(len(p)))
	})
	b.Run("append-rotate", func(b *testing.B) {
		p := testutil.BenchmarkPayload(256)
		l := log.NewLogForBench(b, 0, int64(64+16+len(p)))
		if _, err := l.Append(p); err != nil {
			b.Fatalf("seed append error = %v", err)
		}
		b.SetBytes(int64(len(p)))
		b.ResetTimer()
		for b.Loop() {
			if _, err := l.Append(p); err != nil {
				b.Fatalf("Log.Append() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 1, int64(len(p)))
	})
	b.Run("append-reap-rotate", func(b *testing.B) {
		p := testutil.BenchmarkPayload(256)
		l := log.NewLogForBench(b, 2, int64(64+16+len(p)))
		if _, err := l.Append(p); err != nil {
			b.Fatalf("seed append error = %v", err)
		}
		b.SetBytes(int64(len(p)))
		b.ResetTimer()
		for b.Loop() {
			if _, err := l.Append(p); err != nil {
				b.Fatalf("Log.Append() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 1, int64(len(p)))
	})
	b.Run("append-frequent-rotation", func(b *testing.B) {
		p := testutil.BenchmarkPayload(64)
		l := log.NewLogForBench(b, 0, int64(64+8*(16+len(p))))
		b.SetBytes(int64(len(p)))
		b.ResetTimer()
		for b.Loop() {
			if _, err := l.Append(p); err != nil {
				b.Fatalf("Log.Append() error = %v", err)
			}
		}
		testutil.ReportDomainMetrics(b, 1, int64(len(p)))
	})
}

// BenchmarkTruncate measures prefix/suffix truncation costs.
func BenchmarkLog_Truncate(b *testing.B) {
	b.Run("prefix/step=1", func(b *testing.B) { benchTruncatePrefix(b, 1) })
	b.Run("prefix/step=100", func(b *testing.B) { benchTruncatePrefix(b, 100) })
	b.Run("prefix/step=10K", func(b *testing.B) { benchTruncatePrefix(b, 10_000) })
	b.Run("suffix/near-tail", func(b *testing.B) { benchTruncateSuffix(b, 2_000) })
	b.Run("suffix/middle", func(b *testing.B) { benchTruncateSuffix(b, 10_000) })
	b.Run("suffix/near-head", func(b *testing.B) { benchTruncateSuffix(b, 18_000) })
}

func benchTruncatePrefix(b *testing.B, step int) {
	root := b.TempDir()
	payload := testutil.BenchmarkPayload(16)
	target := min(uint64(step), uint64(20_000))
	i := 0
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(root, "prefix-"+strconv.Itoa(i))
		i++
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("MkdirAll() error = %v", err)
		}
		j := seedLogAt(b, dir, 20_000, payload, 64*1024*1024)
		b.StartTimer()
		if err := j.TruncateBefore(target); err != nil {
			b.Fatalf("TruncateBefore() error = %v", err)
		}
		b.StopTimer()
		if err := j.Close(); err != nil {
			b.Fatalf("Journal.Close() error = %v", err)
		}
		b.StartTimer()
	}
}

func benchTruncateSuffix(b *testing.B, cut int) {
	root := b.TempDir()
	payload := testutil.BenchmarkPayload(16)
	target := min(uint64(cut), uint64(19_999))
	i := 0
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(root, "suffix-"+strconv.Itoa(i))
		i++
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("MkdirAll() error = %v", err)
		}
		j := seedLogAt(b, dir, 20_000, payload, 64*1024*1024)
		b.StartTimer()
		if err := j.TruncateAfter(target); err != nil {
			b.Fatalf("TruncateAfter() error = %v", err)
		}
		b.StopTimer()
		if err := j.Close(); err != nil {
			b.Fatalf("Journal.Close() error = %v", err)
		}
		b.StartTimer()
	}
}

// BenchmarkRawFileWrite compares raw file writes versus log append paths.
func BenchmarkRawFile_Write(b *testing.B) {
	for _, c := range []struct {
		name string
		size int
	}{{"16B", 16}, {"4KiB", 4 * testutil.OneKiB}, {"1MiB", testutil.OneMiB}} {
		b.Run("raw-write/"+c.name, func(b *testing.B) {
			f := testutil.RawFileForBench(b)
			p := testutil.BenchmarkPayload(c.size)
			b.SetBytes(int64(len(p)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := f.Write(p); err != nil {
					b.Fatalf("raw write error: %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, 1, int64(len(p)))
		})
	}
}

// BenchmarkRawFileWriteSync compares raw write+sync with log durability paths.
func BenchmarkRawFile_WriteSync(b *testing.B) {
	for _, c := range []struct {
		name string
		size int
	}{{"16B", 16}, {"4KiB", 4 * testutil.OneKiB}} {
		b.Run("raw-write-sync/"+c.name, func(b *testing.B) {
			f := testutil.RawFileForBench(b)
			p := testutil.BenchmarkPayload(c.size)
			b.SetBytes(int64(len(p)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := f.Write(p); err != nil {
					b.Fatalf("raw write error: %v", err)
				}
				if err := f.Sync(); err != nil {
					b.Fatalf("raw sync error: %v", err)
				}
			}
			testutil.ReportDomainMetrics(b, 1, int64(len(p)))
		})
	}
}

// BenchmarkRawFileRead compares raw sequential reads to scanner throughput.
func BenchmarkRawFile_Read(b *testing.B) {
	f := testutil.RawFileForBench(b)
	p := testutil.BenchmarkPayload(64 * testutil.OneKiB)
	for range 1024 {
		if _, err := f.Write(p); err != nil {
			b.Fatalf("seed write error: %v", err)
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		b.Fatalf("seek start error: %v", err)
	}
	buf := make([]byte, len(p))
	b.SetBytes(int64(len(p) * 1024))
	b.ResetTimer()
	for b.Loop() {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			b.Fatalf("seek reset error: %v", err)
		}
		read := 0
		for read < len(p)*1024 {
			n, err := f.Read(buf)
			if err != nil && err != io.EOF {
				b.Fatalf("read error: %v", err)
			}
			if n == 0 {
				break
			}
			read += n
		}
	}
	testutil.ReportDomainMetrics(b, 1024, int64(len(p)*1024))
}

func seedLogAt(b *testing.B, dir string, records int, payload []byte, maxSeg int64) *log.Log {
	b.Helper()
	l, err := log.Load(dir, log.Options{
		Storage: log.StorageOptions{
			MaxDiskSize:    0,
			MaxSegmentSize: maxSeg,
			MaxSegments:    0,
		},
	})
	if err != nil {
		b.Fatalf("Load() error = %v", err)
	}

	for range records {
		if _, err = l.Append(payload); err != nil {
			if closeErr := l.Close(); closeErr != nil {
				b.Fatalf("Journal.Append preload error = %v (close error: %v)", err, closeErr)
			}
			b.Fatalf("Journal.Append preload error = %v", err)
		}
	}

	if err = l.Sync(); err != nil {
		if closeErr := l.Close(); closeErr != nil {
			b.Fatalf("Journal.Sync preload error = %v (close error: %v)", err, closeErr)
		}
		b.Fatalf("Journal.Sync preload error = %v", err)
	}

	return l
}

func BenchmarkLog_ConsumeBatch(b *testing.B) {
	for _, n := range []int{1, 64} {
		b.Run(testutil.BenchmarkLabelInt("batch", n), func(b *testing.B) {
			log, err := log.Load(
				b.TempDir(),
				log.Options{
					Storage: log.StorageOptions{
						MaxSegmentSize: 64 * 1024 * 1024,
						MaxSegments:    1,
					},
				},
			)
			if err != nil {
				b.Fatalf("LoadLog() error = %v", err)
			}

			b.Cleanup(func() { _ = log.Close() })
			p := testutil.BenchmarkPayload(16)
			b.SetBytes(int64(n * len(p)))
			b.ResetTimer()

			for b.Loop() {
				for range n {
					if _, err = log.Append(p); err != nil {
						b.Fatalf("WAL.Append() error = %v", err)
					}
				}
				err = log.Consume(0, func(_ uint64, _ []byte) error { return nil })
				if err != nil {
					b.Fatalf("WAL.Consume() error = %v", err)
				}
			}

			testutil.ReportDomainMetrics(b, int64(n), int64(n*len(p)))
		})
	}
}
