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
	"sync"
)

// Pool is a pool of borrowed values that can be reused to avoid allocations.
type Pool struct {
	pool            sync.Pool
	maxRetainedSize int
	defaultSize     int
}

// NewPool creates a new pool of borrowed values with the given default byte slice size.
func NewPool(defaultSize, maxRetainedSize int) *Pool {
	p := &Pool{
		defaultSize:     defaultSize,
		maxRetainedSize: maxRetainedSize,
	}

	p.pool = sync.Pool{
		New: func() any {
			return &Frame{
				pool:    p,
				Payload: make([]byte, 0, defaultSize),
			}
		},
	}

	return p
}

// Get returns a borrowed frame from the pool. The returned value should be released back to the pool
func (p *Pool) Get() *Frame {
	return p.pool.Get().(*Frame) // nolint:errcheck // safe
}

// put returns a borrowed value to the pool. The value should not be used after calling Put.
func (p *Pool) put(v *Frame) {
	if cap(v.Payload) > p.maxRetainedSize {
		v.Payload = make([]byte, 0, p.defaultSize)
	} else {
		v.Payload = v.Payload[:0]
	}

	p.pool.Put(v)
}

// Frame is a wrapper around a byte slice that is borrowed from a pool.
type Frame struct {
	pool    *Pool
	Payload []byte
	Header  Header
}

// Value copies the borrowed value into a new slice and returns it along with the header.
func (f *Frame) Value() (Header, []byte) {
	var h Header

	payload := make([]byte, len(f.Payload))

	copy(payload, f.Payload)
	copy(h[:], f.Header[:])

	f.Return()

	return h, payload
}

// Validate the frame header and payload.
func (f *Frame) Validate() error {
	return f.Header.Validate(f.Payload)
}

// Set sets the index and value of the borrowed value.
func (f *Frame) Set(header Header, payload []byte) *Frame {
	f.Header = header
	f.Payload = append(f.Payload[:0], payload...)

	return f
}

// Return the borrowed value to the pool.
// It should be called exactly once in the same context where the value
// was obtained, and the value should not be used after calling it.
func (f *Frame) Return() {
	if f.pool == nil {
		// prevent attempts at returning a frame that was not obtained from a pool
		return
	}
	f.pool.put(f)
}
