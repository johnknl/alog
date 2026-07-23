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
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanner_NextAndBorrow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	opts := DefaultOptions()
	opts.Storage.MaxSegmentSize = 1024
	opts.Storage.MaxSegments = 2

	log, err := Load(dir, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	_, err = log.Append([]byte("one"), []byte("two"))
	require.NoError(t, err)

	s := NewScanner(log)

	require.True(t, s.Next())
	seq0, b0 := s.Borrow()
	require.Equal(t, uint64(0), seq0)
	require.Equal(t, "one", string(b0.Payload))
	b0.Return()

	require.True(t, s.Next())
	seq1, b1 := s.Borrow()
	require.Equal(t, uint64(1), seq1)
	require.Equal(t, "two", string(b1.Payload))
	b1.Return()

	require.False(t, s.Next())
	require.NoError(t, s.Err())
}

func TestScanner_SeekAndStopAt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	opts := DefaultOptions()
	opts.Storage.MaxSegmentSize = 110
	opts.Storage.MaxSegments = 4

	log, err := Load(dir, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	for _, payload := range []string{"a", "b", "c", "d", "e", "f"} {
		_, err = log.Append([]byte(payload))
		require.NoError(t, err)
	}

	s := NewScanner(log)
	s.Seek(1)
	s.StopAt(3)

	got := make([]string, 0, 2)
	for s.Next() {
		seq, borrowed := s.Borrow()
		got = append(got, string(borrowed.Payload)+":"+strconv.FormatUint(seq, 10))
		borrowed.Return()
	}

	require.NoError(t, s.Err())
	require.Equal(t, []string{"b:1", "c:2"}, got)
}

func TestScanner_SeekAtSegmentBoundary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	opts := DefaultOptions()
	opts.Storage.MaxSegmentSize = 110
	opts.Storage.MaxSegments = 4

	log, err := Load(dir, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	for _, payload := range []string{"a", "b", "c", "d"} {
		_, err = log.Append([]byte(payload))
		require.NoError(t, err)
	}

	s := NewScanner(log)
	s.Seek(2)

	require.True(t, s.Next())
	seq, frame := s.Borrow()
	require.Equal(t, uint64(2), seq)
	require.Equal(t, []byte("c"), frame.Payload)

	require.True(t, s.Next())
	seq, frame = s.Borrow()
	require.Equal(t, uint64(3), seq)
	require.Equal(t, []byte("d"), frame.Payload)

	require.False(t, s.Next())
	require.NoError(t, s.Err())
}

func TestScanner_BorrowSequenceAfterTruncateBefore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	opts := DefaultOptions()
	opts.Storage.MaxSegmentSize = 1024
	opts.Storage.MaxSegments = 2

	log, err := Load(dir, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	_, err = log.Append([]byte("a"), []byte("b"), []byte("c"), []byte("d"))
	require.NoError(t, err)

	require.NoError(t, log.TruncateBefore(2))

	s := NewScanner(log)
	require.True(t, s.Next())
	seq, frame := s.Borrow()
	require.Equal(t, uint64(2), seq)
	require.Equal(t, []byte("c"), frame.Payload)
	frame.Return()

	require.True(t, s.Next())
	seq, frame = s.Borrow()
	require.Equal(t, uint64(3), seq)
	require.Equal(t, []byte("d"), frame.Payload)
	frame.Return()

	require.False(t, s.Next())
	require.NoError(t, s.Err())
}
