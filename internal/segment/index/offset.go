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

import (
	"sort"
)

const sparseIndexStride uint32 = 256

// OffsetIndex stores sparse index-to-offset checkpoints.
type OffsetIndex struct {
	entries []Entry
}

// Entry stores one sparse checkpoint.
type Entry struct {
	index  uint32
	offset int64
}

// Reset clears entries.
func (s *OffsetIndex) Reset() {
	s.entries = s.entries[:0]
}

// SparseSet checkpoints at stride boundaries.
func (s *OffsetIndex) SparseSet(index uint32, offset int64) {
	if index%sparseIndexStride != 0 {
		return
	}

	s.Set(index, offset)
}

// Set inserts or updates the checkpoint for index.
func (s *OffsetIndex) Set(index uint32, offset int64) {
	n := len(s.entries)
	if n == 0 {
		s.entries = append(s.entries, Entry{index: index, offset: offset})
		return
	}

	last := s.entries[n-1]
	if index > last.index {
		s.entries = append(s.entries, Entry{index: index, offset: offset})
		return
	}

	i := sort.Search(n, func(i int) bool { return s.entries[i].index >= index })
	if i < n && s.entries[i].index == index {
		s.entries[i].offset = offset
		return
	}

	s.entries = append(s.entries, Entry{})
	copy(s.entries[i+1:], s.entries[i:])

	s.entries[i] = Entry{index: index, offset: offset}
}

// ForFrameIndex returns the last checkpoint index that is <= index.
func (s *OffsetIndex) ForFrameIndex(index uint32) (uint32, int64, bool) {
	return s.by(func(i int) bool {
		return s.entries[i].index > index
	})
}

// ForOffset return the last checkpoint offset that is <= offset.
func (s *OffsetIndex) ForOffset(offset int64) (uint32, int64, bool) {
	return s.by(func(i int) bool {
		return s.entries[i].offset > offset
	})
}

// TruncateAfter removes all checkpoints after index.
func (s *OffsetIndex) TruncateAfter(index uint32) {
	i := s.findFrameIndex(index)
	if i == -1 {
		return
	}

	s.entries = s.entries[:i+1]
}

// TruncateBefore removes all checkpoints before index.
func (s *OffsetIndex) TruncateBefore(index uint32) {
	i := s.findFrameIndex(index)
	if i == -1 {
		return
	}

	s.entries = s.entries[i:]
}

func (s *OffsetIndex) findFrameIndex(idx uint32) (entryIndex int) {
	n := len(s.entries)
	if n == 0 {
		return -1
	}

	i := sort.Search(n, func(i int) bool {
		return s.entries[i].index > idx
	})

	if i == 0 {
		return 0
	}

	return i - 1
}

func (s *OffsetIndex) by(cmp func(i int) bool) (idx uint32, offset int64, found bool) {
	n := len(s.entries)
	if n == 0 {
		return 0, 0, false
	}

	i := sort.Search(n, cmp)

	if i == 0 {
		return 0, 0, false
	}

	e := s.entries[i-1]

	return e.index, e.offset, true
}
