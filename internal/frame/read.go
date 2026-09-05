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

package frame

import (
	"errors"
	"io"

	"github.com/johnknl/alog/internal/storage"
)

// Reader is a reader that uses a pool of borrowed values to avoid allocations.
type Reader struct {
	file storage.File
	pool *Pool
	max  uint32
}

// NewReader creates a new BorrowedFrameReader with the given file and pool.
func NewReader(file storage.File, pool *Pool, limit uint32) *Reader {
	return &Reader{
		file: file,
		pool: pool,
		max:  limit,
	}
}

// Read reads the frame header and payload for the given index and offset,
// and returns a borrowed value from the pool.
func (r *Reader) Read(index uint32, offset int64) (*Frame, error) {
	if r.max == 0 {
		f := &Frame{}
		if err := ReadHeader(r.file, index, offset, &f.Header); err != nil {
			return nil, err
		}

		return f, nil
	}

	f := r.pool.Get()
	err := ReadHeader(r.file, index, offset, &f.Header)
	if err != nil {
		f.Return()
		return nil, err
	}

	if r.max == 0 {
		return f, nil
	}

	n := min(f.Header.PayloadLength(), r.max)

	// Ensure the borrowed payload slice is large enough to hold the payload
	if uint32(cap(f.Payload)) < n { // #nosec: G115 r.max is uint32
		f.Payload = make([]byte, n)
	} else {
		f.Payload = f.Payload[:n]
	}

	err = readAt(r.file, f.Payload, offset+HeaderSize)
	if err != nil {
		f.Return()
		return nil, err
	}

	return f, err
}

// ReadHeader reads the frame header for the given index and offset into the provided Header
func ReadHeader(file storage.File, index uint32, offset int64, h *Header) error {
	err := readAt(file, h[:], offset)
	if err != nil {
		return err
	}

	if index != h.Index() {
		return ErrInvalidFrameIndex
	}

	return nil
}

// readAt reads from the file at the given offset into the destination slice
func readAt(file storage.File, dst []byte, offset int64) error {
	n, err := file.ReadAt(dst, offset)
	if err != nil {
		if errors.Is(err, io.EOF) && n > 0 {
			return io.ErrUnexpectedEOF
		}

		return err
	}

	return nil
}
