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

package log

import "time"

// Options defines the configuration options for the log.
type Options struct {
	// Storage options define the configuration options for segment
	// loading, creation and reaping.
	Storage StorageOptions

	// Write options define the configuration options for the log's buffered writing.
	Pool PoolOptions
}

// PoolOptions defines the configuration options for the frame pool.
type PoolOptions struct {
	// DefaultSize is number of pre-allocated bytes for pool-created slices.
	DefaultSize int

	// MaxSize is the maximum size of slices to be pooled.
	// Slices larger than this size will not be pooled.
	MaxSize int
}

// StorageOptions defines the configuration options for segment loading.
type StorageOptions struct {
	// MaxDiskSize is the maximum retained physical size of all segment files.
	// A value of 0 or smaller means disk retention is unbounded.
	MaxDiskSize int64

	// MaxSegments is the maximum number of segments that can be stored in the log.
	// A value of 0 or smaller means no explicit segment-count limit.
	// A value of 1 means that the log will only have one segment, and appending will be rejected
	// when the segment is full, assuming MaxSegmentSize is set to a positive value.
	MaxSegments int

	// MaxSegmentSize is the maximum size of a segment in bytes. When a segment reaches this size,
	// a new segment will be created, unless max number of segments is 1,
	// in which case appending will be rejected. A value of 0 or smaller means there is no limit
	// on the segment size. MaxSegments must then be set explicitly to 1.
	MaxSegmentSize int64

	// SyncOnAppend controls whether each append call fsyncs segment data before returning.
	// When false, callers can issue explicit durability barriers via Log.Sync().
	SyncOnAppend bool
}

// WriterOptions defines the configuration options for the log's buffered writing.
type WriterOptions struct {
	// Buffer defines the configuration options for the log's buffered writing.
	Buffer WriteBufferOptions
}

// WriteBufferOptions defines the configuration options for the log's buffered writing.
type WriteBufferOptions struct {
	// MaxLength is the maximum number of entries that can be buffered before a flush is triggered.
	MaxLength int

	// MaxSize is the maximum size in bytes of the buffered entries before a flush is triggered.
	MaxSize int64

	// MaxDelay is the maximum age of the buffered entries before a flush is triggered.
	MaxDelay time.Duration
}

// DefaultOptions returns a baseline configuration that is suitable for
// most local and server workloads and can be tweaked per deployment.
func DefaultOptions() Options {
	return Options{
		Storage: StorageOptions{
			MaxDiskSize:    10 * 1024 * 1024 * 1024,
			MaxSegments:    0,
			MaxSegmentSize: 128 * 1024 * 1024,
		},
		Pool: PoolOptions{
			DefaultSize: 4 * 1024,
			MaxSize:     1 * 1024 * 1024,
		},
	}
}
