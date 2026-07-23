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
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	opts := DefaultOptions()
	require.Equal(t, int64(10*1024*1024*1024), opts.Storage.MaxDiskSize)
	require.Equal(t, 0, opts.Storage.MaxSegments)
	require.Equal(t, int64(128*1024*1024), opts.Storage.MaxSegmentSize)
	require.False(t, opts.Storage.SyncOnAppend)
	require.Equal(t, 4*1024, opts.Pool.DefaultSize)
	require.Equal(t, 1024*1024, opts.Pool.MaxSize)
}

func TestLog_New(t *testing.T) {
	t.Parallel()

	t.Run("accepts unbounded segments", func(t *testing.T) {
		t.Parallel()

		opts := DefaultOptions()
		opts.Storage.MaxDiskSize = 0
		opts.Storage.MaxSegmentSize = 0

		log, err := New(opts)
		require.NoError(t, err)
		require.NoError(t, log.Close())
	})
}

func TestLog_LoadAppendScan(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	opts := DefaultOptions()
	opts.Storage.MaxSegments = 2
	opts.Storage.MaxSegmentSize = 1024

	log, err := Load(dir, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	lastSeq, err := log.Append([]byte("alpha"), []byte("beta"))
	require.NoError(t, err)
	require.Equal(t, uint64(1), lastSeq)

	s := NewScanner(log)
	var got []string
	for s.Next() {
		seq, frame := s.Borrow()
		got = append(got, string(frame.Payload)+":"+strconv.FormatUint(seq, 10))
	}
	require.NoError(t, s.Err())
	require.Equal(t, []string{"alpha:0", "beta:1"}, got)
}

func TestLog_Sync(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	opts := DefaultOptions()

	log, err := Load(dir, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })
	_, err = log.Append([]byte("sync-me"))
	require.NoError(t, err)
	require.NoError(t, log.Sync())
}

func TestIsSegmentFull(t *testing.T) {
	t.Parallel()

	require.True(t, IsSegmentFull(ErrSegmentFull))
	require.True(t, IsSegmentFull(errors.Join(errors.New("x"), ErrSegmentFull)))
	require.True(t, IsSegmentFull(errors.Join(errors.New("x"), ErrBatchTooLarge)))
	require.False(t, IsSegmentFull(errors.New("other")))
}

func TestCapacityErrorPredicates(t *testing.T) {
	t.Parallel()

	require.True(t, IsDiskFull(ErrDiskFull))
	require.True(t, IsDiskFull(errors.Join(errors.New("x"), ErrDiskFull)))
	require.False(t, IsDiskFull(errors.New("other")))

	require.True(t, IsBatchTooLarge(ErrBatchTooLarge))
	require.True(t, IsBatchTooLarge(errors.Join(errors.New("x"), ErrBatchTooLarge)))
	require.False(t, IsBatchTooLarge(errors.New("other")))
}

func TestLog_DiskSize(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	opts := DefaultOptions()
	opts.Storage.MaxSegmentSize = 1024

	log, err := Load(dir, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })
	require.Equal(t, int64(64), log.DiskSize())

	_, err = log.Append([]byte("abc"))
	require.NoError(t, err)
	require.Equal(t, int64(64+16+3), log.DiskSize())
}

func TestLog_Consume(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	log, err := Load(dir, DefaultOptions())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })
	require.NoError(t, err)

	_, err = log.Append([]byte("a"), []byte("b"), []byte("c"))
	require.NoError(t, err)

	seen := make([]string, 0, 3)
	err = log.Consume(0, func(seq uint64, payload []byte) error {
		seen = append(seen, string(payload))
		require.Equal(t, uint64(len(seen)-1), seq)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, seen)

	seen = seen[:0]
	err = log.Consume(0, func(_ uint64, payload []byte) error {
		seen = append(seen, string(payload))
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, seen)
}

func TestLog_ConsumeStopsOnCallbackError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	log, err := Load(dir, DefaultOptions())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })
	require.NoError(t, err)

	_, err = log.Append([]byte("a"), []byte("b"), []byte("c"))
	require.NoError(t, err)

	stopErr := errors.New("stop")
	err = log.Consume(0, func(seq uint64, _ []byte) error {
		if seq == 1 {
			return stopErr
		}
		return nil
	})
	require.ErrorIs(t, err, stopErr)

	seen := make([]string, 0, 2)
	err = log.Consume(0, func(_ uint64, payload []byte) error {
		seen = append(seen, string(payload))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"b", "c"}, seen)
}

func TestLog_RestartConsumeCursorFromDiskHead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	log1, err := Load(dir, DefaultOptions())
	require.NoError(t, err)
	t.Cleanup(func() {
		if log1 != nil {
			require.NoError(t, log1.Close())
		}
	})
	require.NoError(t, err)

	_, err = log1.Append([]byte("evt-0"), []byte("evt-1"), []byte("evt-2"), []byte("evt-3"))
	require.NoError(t, err)

	consumed := make([]string, 0, 2)
	err = log1.Consume(0, func(seq uint64, payload []byte) error {
		if seq < 2 {
			consumed = append(consumed, string(payload))
			return nil
		}

		return context.Canceled
	})

	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, log1.Close())
	log1 = nil

	log2, err := Load(dir, DefaultOptions())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log2.Close()) })
	require.NoError(t, err)

	rest := make([]string, 0, 2)
	err = log2.Consume(0, func(_ uint64, payload []byte) error {
		rest = append(rest, string(payload))
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, []string{"evt-2", "evt-3"}, rest)
}
