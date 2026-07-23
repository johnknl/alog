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
	"iter"

	"github.com/johnknl/alog/internal/frame"
	"github.com/johnknl/alog/internal/segment"
)

var (
	// ErrSegmentReaped is returned when a segment has been reaped and is no longer available for scanning.
	ErrSegmentReaped = segment.ErrSegmentReaped
)

// Scanner is a scanner that borrows frames from the log segments.
type Scanner struct {
	err          error
	log          *Log
	current      *segment.Segment
	segScanner   *segment.Scanner
	borrowed     *frame.Frame
	segments     []*segment.Segment
	segmentIndex int
	endSeq       uint64
}

// NewScanner creates a new scanner for the given log.
func NewScanner(log *Log) *Scanner {
	return &Scanner{
		log:      log,
		segments: log.segments(),
	}
}

// Segment returns the current segment being scanned.
func (s *Scanner) Segment() *segment.Segment {
	return s.current
}

// Err returns the error that occurred during scanning, if any.
func (s *Scanner) Err() error {
	return s.err
}

// Next advances the scanner to the next frame in the log segments.
func (s *Scanner) Next() bool {
	if s.err != nil {
		return false
	}

	segments := s.segments

	for s.segmentIndex < len(segments) {
		if s.endSeq > 0 && segments[s.segmentIndex].StartSequence() >= s.endSeq {
			return false
		}

		if s.segScanner == nil {
			s.current = segments[s.segmentIndex]
			s.segScanner = segment.NewScanner(s.current, s.log.framePool)
		}

		if s.segScanner.Next() {
			s.borrowed = s.segScanner.Borrow()
			if s.endSeq > 0 {
				seq, _ := s.Borrow()
				if seq >= s.endSeq {
					s.borrowed.Return()
					s.borrowed = nil
					return false
				}
			}

			return true
		}

		if err := s.segScanner.Err(); err != nil {
			if errors.Is(err, segment.ErrSegmentReaped) {
				err = ErrSegmentReaped
			}

			s.err = err

			return false
		}

		s.segmentIndex++
		s.current = nil
		s.segScanner = nil
	}

	return false
}

// Seek positions the scanner so the next Next call starts at seq.
//
// If seq is beyond the end of the retained log, Next will return false.
func (s *Scanner) Seek(startSeq uint64) {
	segments := s.segments
	s.segmentIndex = len(segments)
	s.current = nil
	s.segScanner = nil
	s.borrowed = nil
	s.err = nil

	for i, seg := range segments {
		if startSeq < seg.NextSequence() {
			s.segmentIndex = i
			s.current = seg
			s.segScanner = segment.NewScanner(seg, s.log.framePool)

			seekSeq := max(startSeq, seg.StartSequence())

			if err := s.segScanner.Seek(seekSeq); err != nil {
				s.err = err
			}
			return
		}
	}
}

// Borrow returns the next sequence number and a borrowed frame from the log segments.
// If you use this method directly, make sure to call Return() on the frame when the
// value will no longer be used.
func (s *Scanner) Borrow() (uint64, *frame.Frame) {
	return s.currentStartSequence() + uint64(s.borrowed.Header.Index()), s.borrowed
}

func (s *Scanner) currentStartSequence() uint64 {
	if s.current == nil {
		return 0
	}

	return s.current.BaseSequence()
}

// StopAt sets an exclusive end sequence bound for scanning.
//
// If endSeq is 0, scanning continues until EOF of the last segment.
func (s *Scanner) StopAt(endSeq uint64) {
	s.endSeq = endSeq
}

// Iter returns an iterator that yields sequence numbers and borrowed payloads from the log segments.
// It will return each frame to the pool after yield returns (at the end of each iteration)
func (s *Scanner) Iter() iter.Seq2[uint64, []byte] {
	return func(yield func(uint64, []byte) bool) {
		for s.Next() {
			seq, borrowed := s.Borrow()

			if !yield(seq, borrowed.Payload) {
				borrowed.Return()
				return
			}

			borrowed.Return()
		}
	}
}
