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

package write

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/johnknl/alog/pkg/log"
)

var (
	// ErrStopped is returned when the writer has been stopped and cannot accept new requests.
	ErrStopped = errors.New("writer stopped")
)

type response struct {
	err error
	seq uint64
}

type request struct {
	resp chan response
	data []byte
}

type appendSyncer interface {
	Append(payloads ...[]byte) (uint64, error)
}

// StartWriter creates and starts a writer for the provided log.
func StartWriter(ctx context.Context, l *log.Log, opts log.WriteBufferOptions) *Writer {
	w := NewWriter(l, opts.MaxLength, opts.MaxSize, opts.MaxDelay)
	w.Start(ctx)

	return w
}

// Writer buffers concurrent append requests and forwards batches to the
// configured append target.
type Writer struct {
	log       appendSyncer
	requestCh chan request
	stopCh    chan struct{}
	startOnce sync.Once

	// MaxLength is the maximum number of entries that can be buffered before a flush is triggered.
	MaxLength int

	// MaxSize is the maximum size in bytes of the buffered entries before a flush is triggered.
	MaxSize int64

	// MaxDelay is the maximum age of the buffered entries before a flush is triggered.
	MaxDelay time.Duration
}

// NewWriter creates a new writer instance with the given options.
func NewWriter(appender appendSyncer, maxLength int, maxSize int64, maxDelay time.Duration) *Writer {
	return &Writer{
		log:       appender,
		requestCh: make(chan request),
		stopCh:    make(chan struct{}),
		MaxLength: maxLength,
		MaxSize:   maxSize,
		MaxDelay:  maxDelay,
	}
}

// Append queues a payload and returns its assigned sequence when the batch flushes.
func (l *Writer) Append(ctx context.Context, data []byte) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	responseCh := make(chan response, 1)
	req := request{
		data: data,
		resp: responseCh,
	}

	select {
	case <-l.stopCh:
		return 0, ErrStopped
	case <-ctx.Done():
		return 0, ctx.Err()
	case l.requestCh <- req:
	}

	resp := <-responseCh

	return resp.seq, resp.err
}

// Start a background goroutine that listens for data on the buffer channel
func (l *Writer) Start(ctx context.Context) {
	l.startOnce.Do(func() {
		go l.flushLoop(ctx)
	})
}

func (l *Writer) flushLoop(ctx context.Context) {
	defer close(l.stopCh)

	batch := make([]request, 0, l.MaxLength)
	var size int64

	timer := newLazyTimer(l.MaxDelay)
	flush := func() {
		if len(batch) > 0 {
			l.writeBatch(batch)

			clear(batch)
			batch = batch[:0]
			size = 0
		}

		timer.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-timer.C:
			flush()
		case req := <-l.requestCh:
			if len(batch) == 0 {
				timer.Start()
			}

			batch = append(batch, req)
			size += int64(len(req.data))

			if size < l.MaxSize && len(batch) < l.MaxLength {
				continue
			}

			flush()
		}
	}
}

// writeBatch synchronously writes a batch of requests.
func (l *Writer) writeBatch(batch []request) {
	payloads := make([][]byte, 0, len(batch))
	for _, req := range batch {
		payloads = append(payloads, req.data)
	}

	lastSeq, err := l.log.Append(payloads...)
	if err != nil {
		for _, req := range batch {
			req.resp <- response{
				err: err,
			}
		}

		return
	}

	seq := lastSeq - uint64(len(batch)) + 1

	for _, req := range batch {
		req.resp <- response{
			seq: seq,
		}
		seq++
	}
}
