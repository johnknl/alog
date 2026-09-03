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
	"os"
	"path/filepath"
	"testing"

	"github.com/johnknl/alog/internal/frame"
	"github.com/stretchr/testify/require"
)

func TestChain_NewChain(t *testing.T) {
	t.Parallel()

	t.Run("accepts unbounded segments without count limit", func(t *testing.T) {
		t.Parallel()

		_, err := NewChain(0, 0, 0, 0, false, nil)
		require.NoError(t, err)
	})

	t.Run("accepts unbounded single segment", func(t *testing.T) {
		t.Parallel()

		_, err := NewChain(0, 0, 0, 1, false, nil)
		require.NoError(t, err)
	})
}

func TestChain_Load(t *testing.T) {
	t.Parallel()

	t.Run("creates first segment when directory is empty", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChain(t, 2, 1024)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		require.Len(t, chain.ActiveSegments(), 1)
		require.Equal(t, uint64(0), chain.Tail().StartSequence())
	})

	t.Run("ignores non-segment files", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o644)
		require.NoError(t, err)

		pool := frame.NewPool(1024, 10)
		chain, err := NewChain(0, 1024, 0, 2, false, nil)
		require.NoError(t, err)
		require.NoError(t, chain.Load(dir, pool, nil))
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		require.Len(t, chain.ActiveSegments(), 1)
	})

	t.Run("ignores symlinked segment files", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		target := filepath.Join(t.TempDir(), "target.bin")
		err := os.WriteFile(target, []byte("not-a-segment"), 0o600)
		require.NoError(t, err)
		err = os.Symlink(target, filepath.Join(dir, "00000000000000000042.bin"))
		require.NoError(t, err)

		pool := frame.NewPool(1024, 10)
		chain, err := NewChain(0, 1024, 0, 2, false, nil)
		require.NoError(t, err)
		require.NoError(t, chain.Load(dir, pool, nil))
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		segments := chain.ActiveSegments()
		require.Len(t, segments, 1)
		require.Equal(t, filepath.Join(dir, "00000000000000000000.bin"), segments[0].Name())
	})

	t.Run("returns error when segments exceed max", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		s0 := createSegmentAt(t, dir, 0)
		require.NoError(t, s0.Append([]byte("a")))
		require.NoError(t, s0.Close())

		s1 := createSegmentAt(t, dir, 1)
		require.NoError(t, s1.Close())

		pool := frame.NewPool(1024, 10)
		chain, err := NewChain(0, 1024, 0, 1, false, nil)
		require.NoError(t, err)
		err = chain.Load(dir, pool, nil)
		require.Error(t, err)
	})

	t.Run("creates missing directory for single segment mode", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(t.TempDir(), "nested", "missing")
		pool := frame.NewPool(1024, 10)
		chain, err := NewChain(0, 1024, 0, 1, false, nil)
		require.NoError(t, err)
		require.NoError(t, chain.Load(dir, pool, nil))
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		require.Len(t, chain.ActiveSegments(), 1)
	})
}

func TestChain_TruncateBeforeTailNextSequence(t *testing.T) {
	t.Parallel()

	chain := newLoadedChain(t, 1, 1024)
	t.Cleanup(func() { require.NoError(t, chain.Close()) })

	seg := chain.Tail()
	require.NoError(t, seg.Append([]byte("a"), []byte("b"), []byte("c")))

	require.NoError(t, chain.TruncateBefore(seg.NextSequence()))

	scanner := NewScanner(chain.Head(), frame.NewPool(16, 1024))
	require.False(t, scanner.Next())
	require.NoError(t, scanner.Err())
}

func TestChain_Append(t *testing.T) {
	t.Parallel()

	t.Run("returns segment full for single segment", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChain(t, 1, 98)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		s := chain.Tail()
		require.NoError(t, s.Append([]byte("a"), []byte("b")))

		_, err := chain.Append([]byte("c"))
		require.ErrorIs(t, err, ErrSegmentFull)
	})

	t.Run("rotates to new segment when full", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChain(t, 3, 100)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		s := chain.Tail()
		require.NoError(t, s.Append([]byte("a"), []byte("b")))

		_, err := chain.Append([]byte("c"))
		require.NoError(t, err)
		next := chain.Tail()
		require.NotEqual(t, s.Name(), next.Name())
		require.Equal(t, uint64(2), next.StartSequence())
	})

	t.Run("reaps oldest segment at cap", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChain(t, 2, 100)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		require.NoError(t, chain.Tail().Append([]byte("a"), []byte("b")))
		_, err := chain.Append([]byte("c"), []byte("d"))
		require.NoError(t, err)

		_, err = chain.Append([]byte("e"))
		require.NoError(t, err)
		require.Len(t, chain.ActiveSegments(), 2)
		require.Equal(t, uint64(2), chain.ActiveSegments()[0].StartSequence())
	})
}

func TestChain_DiskBudget(t *testing.T) {
	t.Parallel()

	t.Run("tracks disk size across append and rotation", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChainWithDiskBudget(t, 0, 100, 0)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		require.Equal(t, int64(HeaderSize), chain.DiskSize())
		_, err := chain.Append([]byte("aa"))
		require.NoError(t, err)
		require.Equal(t, int64(HeaderSize)+AppendSize([]byte("aa")), chain.DiskSize())

		_, err = chain.Append([]byte("bbbbbbbbbbbbbbbbbbbb"))
		require.NoError(t, err)
		require.Equal(t, int64(2*HeaderSize)+AppendSize([]byte("aa"), []byte("bbbbbbbbbbbbbbbbbbbb")), chain.DiskSize())
	})

	t.Run("returns disk full when budget cannot be satisfied", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChainWithDiskBudget(t, 0, 128, HeaderSize+AppendSize([]byte("a")))
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		_, err := chain.Append([]byte("a"))
		require.NoError(t, err)
		_, err = chain.Append([]byte("b"))
		require.ErrorIs(t, err, ErrDiskFull)
	})

	t.Run("does not reap for impossible oversized append", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChainWithDiskBudget(t, 2, 98, 0)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		_, err := chain.Append([]byte("a"), []byte("b"))
		require.NoError(t, err)
		segmentsBefore := len(chain.ActiveSegments())
		diskBefore := chain.DiskSize()

		_, err = chain.Append(make([]byte, 128))
		require.ErrorIs(t, err, ErrSegmentFull)
		require.ErrorIs(t, err, ErrBatchTooLarge)
		require.Equal(t, segmentsBefore, len(chain.ActiveSegments()))
		require.Equal(t, diskBefore, chain.DiskSize())
	})

	t.Run("load fails when existing data exceeds budget", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		s := createSegmentAt(t, dir, 0)
		require.NoError(t, s.Append([]byte("payload")))
		require.NoError(t, s.Close())

		pool := frame.NewPool(1024, 10)
		chain, err := NewChain(0, 1024, HeaderSize+1, 0, false, nil)
		require.NoError(t, err)
		err = chain.Load(dir, pool, nil)
		require.ErrorIs(t, err, ErrDiskFull)
	})

	t.Run("truncate before tail keeps only tail and updates disk size", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChainWithDiskBudget(t, 3, 100, 0)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		_, err := chain.Append([]byte("a"), []byte("b"))
		require.NoError(t, err)
		_, err = chain.Append([]byte("c"), []byte("d"))
		require.NoError(t, err)

		before := chain.DiskSize()
		require.Greater(t, before, int64(HeaderSize))

		require.NoError(t, chain.TruncateBefore(10_000))
		require.Len(t, chain.ActiveSegments(), 1)
		require.Equal(t, chain.Tail().Size(), chain.DiskSize())
	})
}

func TestChain_TruncateAfter(t *testing.T) {
	t.Parallel()

	t.Run("truncates within retained segment and reaps newer segments", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChainWithDiskBudget(t, 0, 100, 0)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		_, err := chain.Append([]byte("a"), []byte("b"))
		require.NoError(t, err)
		_, err = chain.Append([]byte("c"), []byte("d"))
		require.NoError(t, err)
		_, err = chain.Append([]byte("e"), []byte("f"))
		require.NoError(t, err)

		require.Len(t, chain.ActiveSegments(), 3)
		before := chain.DiskSize()

		require.NoError(t, chain.TruncateAfter(2))

		segments := chain.ActiveSegments()
		require.Len(t, segments, 2)
		require.Equal(t, uint64(0), segments[0].StartSequence())
		require.Equal(t, uint64(2), segments[1].StartSequence())
		require.Equal(t, uint64(3), segments[1].NextSequence())
		require.Equal(t, chain.DiskSize(), segments[0].Size()+segments[1].Size())
		require.Less(t, chain.DiskSize(), before)
	})

	t.Run("recreates empty chain when truncating before shifted head", func(t *testing.T) {
		t.Parallel()

		chain := newLoadedChainWithDiskBudget(t, 0, 100, 0)
		t.Cleanup(func() { require.NoError(t, chain.Close()) })

		_, err := chain.Append([]byte("a"), []byte("b"))
		require.NoError(t, err)
		_, err = chain.Append([]byte("c"), []byte("d"))
		require.NoError(t, err)

		require.NoError(t, chain.TruncateBefore(2))
		require.Equal(t, uint64(2), chain.Head().StartSequence())

		require.NoError(t, chain.TruncateAfter(1))

		segments := chain.ActiveSegments()
		require.Len(t, segments, 1)
		require.Equal(t, uint64(0), segments[0].StartSequence())
		require.Equal(t, uint64(0), segments[0].NextSequence())
		require.Equal(t, int64(HeaderSize), chain.DiskSize())
	})
}
