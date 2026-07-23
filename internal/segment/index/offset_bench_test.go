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

package index

import "testing"

var (
	benchFrameSink  uint32
	benchOffsetSink int64
)

func BenchmarkOffsetIndexLifecycle(b *testing.B) {
	const (
		headerSize      = int64(64)
		recordSize      = int64(64 << 10)
		segmentSize     = int64(4 << 30)
		initialRecords  = uint32(segmentSize / recordSize)
		appendBatchSize = uint32(64)
	)

	b.ReportAllocs()

	var idx OffsetIndex
	var startFrame uint32
	writeFrame := initialRecords
	startOffset := headerSize
	writeOffset := headerSize + int64(writeFrame)*recordSize

	idx.Set(0, headerSize)
	for i := range initialRecords {
		offset := headerSize + int64(i)*recordSize
		idx.SparseSet(i, offset)
	}

	idx.Set(writeFrame, writeOffset)

	// loaded
	b.ResetTimer()

	var fi uint32
	var off int64
	for i := 0; i < b.N; i++ {
		for range appendBatchSize {
			idx.SparseSet(writeFrame, writeOffset)
			writeFrame++
			writeOffset += recordSize
		}

		idx.Set(writeFrame, writeOffset)

		span := writeFrame - startFrame
		targetStep := uint32(i) % (span + 1)
		targetFrame := startFrame + targetStep
		targetOffset := startOffset + int64(targetStep)*recordSize + recordSize/2

		f, o, ok := idx.ForFrameIndex(targetFrame)
		if !ok {
			b.Fatal("missing frame lookup")
		}

		fi ^= f
		off ^= o

		f, o, ok = idx.ForOffset(targetOffset)
		if !ok {
			b.Fatal("missing offset lookup")
		}

		fi ^= f
		off ^= o

		if i > 0 && i%256 == 0 && writeFrame-startFrame > appendBatchSize {
			startFrame += 37
			startOffset = headerSize + int64(startFrame)*recordSize

			idx.TruncateBefore(startFrame)
			idx.Set(0, headerSize)
			idx.Set(startFrame, startOffset)
			idx.Set(writeFrame, writeOffset)
		}

		if i > 0 && i%1024 == 0 && writeFrame-startFrame > appendBatchSize {
			writeFrame -= 91
			writeOffset = headerSize + int64(writeFrame)*recordSize
			idx.TruncateAfter(writeFrame)
			idx.Set(writeFrame, writeOffset)
		}

		if i > 0 && i%4096 == 0 {
			idx.Reset()
			startFrame = 0
			writeFrame = initialRecords
			startOffset = headerSize
			writeOffset = headerSize + int64(writeFrame)*recordSize
			idx.Set(0, headerSize)

			for i := range initialRecords {
				offset := headerSize + int64(i)*recordSize
				idx.SparseSet(i, offset)
			}

			idx.Set(writeFrame, writeOffset)
		}
	}

	benchFrameSink = fi
	benchOffsetSink = off
}
