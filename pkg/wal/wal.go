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

package wal

import (
	"github.com/johnknl/alog/pkg/log"
)

// WAL is a write-ahead log that provides an append-only log of records.
type WAL struct {
	log *log.Log
}

// New creates a new WAL rooted at filePath with a fixed max segment size.
func New(filePath string, maxSize int64) (*WAL, error) {
	l, err := log.New(log.Options{
		Storage: log.StorageOptions{
			MaxDiskSize:    0,
			MaxSegmentSize: maxSize,
			MaxSegments:    1,
		},
	})

	if err != nil {
		return nil, err
	}

	if err := l.Load(filePath); err != nil {
		return nil, err
	}

	return &WAL{
		log: l,
	}, nil
}

// Append appends the given payloads to the log and returns the sequence number of the last appended record.
// The segment is synced to disk after appending the records.
func (w *WAL) Append(payloads ...[]byte) (uint64, error) {
	seq, err := w.log.Append(payloads...)
	if err != nil {
		return 0, err
	}

	if err := w.log.Sync(); err != nil {
		return 0, err
	}

	return seq, nil
}

// Consume reads and processes records from the log, starting at max(fromSeq, internal cursor).
//
// On success, it advances the WAL consume cursor and truncates records before the next unread
// sequence number. If fn returns an error, Consume stops and returns that error without advancing
// the cursor past the failing record.
func (w *WAL) Consume(fromSeq uint64, fn func(seq uint64, payload []byte) error) error {
	return w.log.Consume(fromSeq, fn)
}

// Close closes the underlying log resources.
func (w *WAL) Close() error {
	return w.log.Close()
}
