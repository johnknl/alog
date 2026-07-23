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

package segment

import (
	"errors"
	"io"
	"math"
	"os"

	"github.com/johnknl/alog/internal/frame"
)

var (
	// ErrEndOfSegment is returned when the scanner reaches the end of the segment.
	ErrEndOfSegment = io.EOF

	// ErrOutOfBounds is returned when the requested sequence number is out of bounds.
	ErrOutOfBounds = errors.New("out of bounds")

	// ErrSegmentReaped is returned when trying to read from a reaped segment.
	ErrSegmentReaped = errors.New("segment reaped")
)

// Scanner is used to scan through the records in a segment.
type Scanner struct {
	reader   *frame.Reader
	err      error
	segment  *Segment
	borrowed *frame.Frame
	offset   int64
	index    uint32
}

// NewScanner creates a new Scanner for the given segment.
func NewScanner(s *Segment, p *frame.Pool) *Scanner {
	startOffset := max(s.ReadStart(), int64(HeaderSize))

	return &Scanner{
		segment: s,
		offset:  startOffset,
		index:   s.ReadStartIndex(),
		reader:  frame.NewReader(s.file, p),
	}
}

// Seek positions the scanner so the next Scan starts at seq.
//
// If seq is beyond the last record, Seek positions at end-of-segment.
func (s *Scanner) Seek(seq uint64) error {
	if seq < s.segment.StartSequence() {
		return ErrOutOfBounds
	}

	if seq-s.segment.BaseSequence() > math.MaxUint32 {
		return ErrOutOfBounds
	}

	if seq > s.segment.NextSequence() {
		idx, offset, err := s.segment.Find(s.segment.NextSequence())
		if err != nil {
			return err
		}

		s.index = idx
		s.offset = offset

		return nil
	}

	idx, offset, err := s.segment.Find(seq)
	if err != nil {
		return err
	}

	s.offset = offset
	s.index = idx

	return nil
}

// ReadOffset returns the current offset of the scanner in the segment file.
func (s *Scanner) ReadOffset() int64 {
	return s.offset
}

// Borrow returns the current record's borrowed frame.
// The caller must call Return() on the borrowed frame when done with it.
func (s *Scanner) Borrow() *frame.Frame {
	return s.borrowed
}

// Value copies the current record's payload and returns it along with its sequence number.
func (s *Scanner) Value() (uint64, []byte) {
	h, p := s.borrowed.Value()
	seq := uint64(h.Index()) + s.segment.header.StartSequence()

	return seq, p
}

// Err returns the first non-EOF error that was encountered by the Scanner.
func (s *Scanner) Err() error {
	return s.err
}

// Next advances the scanner to the next record and returns true if
// there is a next record to read.
func (s *Scanner) Next() bool {
	if s.err != nil {
		return false
	}

	borrowed, err := s.Peek()
	if err != nil {
		if !errors.Is(err, ErrEndOfSegment) {
			s.err = err
		}

		return false
	}

	s.borrowed = borrowed

	s.offset += frame.HeaderSize + int64(len(borrowed.Payload))
	s.index++

	return true
}

// Peek reads the next record from the segment without moving position.
// It returns ErrEndOfSegment if there are no more records to read.
func (s *Scanner) Peek() (*frame.Frame, error) {
	borrowed, err := s.reader.Read(s.index, s.offset)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrEndOfSegment
		}

		if errors.Is(err, os.ErrClosed) {
			return nil, ErrSegmentReaped
		}

		return nil, err
	}

	return borrowed, nil
}
