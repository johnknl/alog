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

import (
	"errors"
	"fmt"

	"github.com/johnknl/alog/internal/frame"
	"github.com/johnknl/alog/internal/segment"
)

// ErrSegmentFull is returned when the segment is full
// and a new one could not be created.
var ErrSegmentFull = segment.ErrSegmentFull

// ErrDiskFull is returned when retained byte budget cannot admit an append.
var ErrDiskFull = segment.ErrDiskFull

// ErrBatchTooLarge is returned when a batch cannot fit in an empty bounded segment.
var ErrBatchTooLarge = segment.ErrBatchTooLarge

// IsSegmentFull reports whether err indicates segment capacity was exhausted.
func IsSegmentFull(err error) bool {
	return errors.Is(err, segment.ErrSegmentFull) || errors.Is(err, segment.ErrBatchTooLarge)
}

// IsDiskFull reports whether err indicates retained byte budget exhaustion.
func IsDiskFull(err error) bool {
	return errors.Is(err, segment.ErrDiskFull)
}

// IsBatchTooLarge reports whether err indicates a batch cannot fit in an empty segment.
func IsBatchTooLarge(err error) bool {
	return errors.Is(err, segment.ErrBatchTooLarge)
}

// Log is responsible for loading existing segments from disk and managing them.
type Log struct {
	chain          *segment.Chain
	framePool      *frame.Pool
	nextConsumeSeq uint64
}

// New creates a new Log with the given options. It validates the options and
// returns an error if they are invalid.
func New(opts Options) (*Log, error) {
	chain, err := segment.NewChain(
		opts.Storage.MaxSegmentSize,
		opts.Storage.MaxDiskSize,
		opts.Storage.MaxSegments,
		opts.Storage.SyncOnAppend,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain: %w", err)
	}

	return &Log{
		chain:     chain,
		framePool: frame.NewPool(opts.Pool.DefaultSize, opts.Pool.MaxSize),
	}, nil
}

// Load creates a new Log with the given options and loads it from
// the given storage location.
func Load(storageLocation string, opts Options) (*Log, error) {
	log, err := New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create log: %w", err)
	}

	if err := log.Load(storageLocation); err != nil {
		return nil, fmt.Errorf("failed to load log: %w", err)
	}

	return log, nil
}

// Head returns the first sequence number in the log.
func (log *Log) Head() uint64 {
	head := log.chain.Head()
	if head == nil {
		return 0
	}

	return head.StartSequence()
}

// Load loads the log from the given storage location
// or creates the first segment if none exist.
func (log *Log) Load(storageLocation string) (err error) {
	err = log.chain.Load(storageLocation, log.framePool, nil)
	if err != nil {
		return fmt.Errorf("failed to load chain: %w", err)
	}

	log.nextConsumeSeq = log.Head()

	return nil
}

// Consume reads and processes records from the log, starting at max(fromSeq, internal cursor).
//
// On success, it advances the consume cursor and truncates records before the next unread
// sequence number. If fn returns an error, Consume stops and returns that error without advancing
// the cursor past the failing record.
func (log *Log) Consume(fromSeq uint64, fn func(seq uint64, payload []byte) error) error {
	start := max(fromSeq, log.nextConsumeSeq)
	if head := log.Head(); start < head {
		start = head
	}

	nextUnread := start

	err := log.Range(start, 0, func(seq uint64, payload []byte) error {
		if err := fn(seq, payload); err != nil {
			return err
		}
		nextUnread = seq + 1
		return nil
	})
	if err != nil {
		if nextUnread > log.nextConsumeSeq {
			log.nextConsumeSeq = nextUnread
			if truncErr := log.TruncateBefore(nextUnread); truncErr != nil {
				return errors.Join(err, truncErr)
			}
		}

		return err
	}

	if nextUnread == start {
		return nil
	}

	log.nextConsumeSeq = nextUnread

	return log.TruncateBefore(nextUnread)
}

// Range over the log from start to end passing the sequence numbers and payloads
// to passed callback. The payload slices should not be used outside of the callback.
func (log *Log) Range(startSeq uint64, endSeq uint64, fn func(seq uint64, payload []byte) error) error {
	s := NewScanner(log)
	s.Seek(startSeq)
	s.StopAt(endSeq)

	for s.Next() {
		seq, f := s.Borrow()
		err := fn(seq, f.Payload)
		f.Return()
		if err != nil {
			return err
		}
	}

	return s.Err()
}

// Append appends the given payloads to the log and returns the last sequence number
// This method is not safe for concurrent use. Use StartWriter to create a writer
// that can be used concurrently. The payloads are treated as one batch.
func (log *Log) Append(payloads ...[]byte) (lastSeq uint64, err error) {
	if len(payloads) == 0 {
		return 0, nil
	}

	seq, err := log.chain.Append(payloads...)
	if err != nil {
		if IsSegmentFull(err) {
			return 0, ErrSegmentFull
		}

		return 0, fmt.Errorf("failed to append payloads: %w", err)
	}

	return seq, nil
}

// Sync flushes file contents and metadata of all open segments to stable storage.
func (log *Log) Sync() error {
	return log.chain.Sync()
}

// TruncateBefore removes all records with sequence numbers less than the specified seq from the log.
func (log *Log) TruncateBefore(seq uint64) error {
	return log.chain.TruncateBefore(seq)
}

// TruncateAfter removes all records with sequence numbers greater than the specified seq from the log.
func (log *Log) TruncateAfter(seq uint64) error {
	return log.chain.TruncateAfter(seq)
}

// Close closes all segments in the log and returns any errors that occurred during closing.
func (log *Log) Close() error {
	return log.chain.Close()
}

func (log *Log) segments() []*segment.Segment {
	return log.chain.ActiveSegments()
}

// DiskSize returns the retained physical size in bytes of segment files.
func (log *Log) DiskSize() int64 {
	return log.chain.DiskSize()
}
