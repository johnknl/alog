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

package archive

import (
	"context"

	"github.com/johnknl/alog/pkg/log"
	"github.com/johnknl/alog/pkg/log/write"
)

// Archive is a high-level abstraction over the Log for concurrent
// appends and ranged sequential reads.
type Archive struct {
	log    *log.Log
	writer *write.Writer
}

// New creates a new Archive.
func New(
	ctx context.Context,
	directory string,
	immutableSegments int,
	maxSegmentSize int64,
	buffer log.WriteBufferOptions,
) (*Archive, error) {
	l, err := log.New(log.Options{
		Storage: log.StorageOptions{
			MaxDiskSize:    int64(immutableSegments+1) * maxSegmentSize,
			MaxSegmentSize: maxSegmentSize,
			MaxSegments:    immutableSegments + 1, // +1 for the active segment
		},
	})
	if err != nil {
		return nil, err
	}

	if err := l.Load(directory); err != nil {
		return nil, err
	}

	writer := write.StartWriter(ctx, l, buffer)

	return &Archive{
		log:    l,
		writer: writer,
	}, nil
}

// Range over the archive from start to end passing the sequence numbers and payloads
// to passed callback. The payload slices should not be used outside of the callback.
func (a *Archive) Range(startSeq uint64, endSeq uint64, fn func(seq uint64, payload []byte) error) error {
	return a.log.Range(startSeq, endSeq, fn)
}

// Append appends the given payload to the archive and returns the last sequence number.
// This method is safe for concurrent use and uses optimistic grouped appends.
func (a *Archive) Append(ctx context.Context, payload []byte) (lastSeq uint64, err error) {
	return a.writer.Append(ctx, payload)
}

// Close closes the underlying log resources.
func (a *Archive) Close() error {
	return a.log.Close()
}
