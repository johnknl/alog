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
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/johnknl/alog/internal/frame"
	"github.com/stretchr/testify/require"
)

func TestSegment_CreateLoadRoundTrip(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 10)
	path := filepath.Join(t.TempDir(), "segment.bin")
	s, err := Create(path, 42, pool, nil, false)
	require.NoError(t, err)

	require.NoError(t, s.Append([]byte("a"), []byte("bb")))
	require.Equal(t, uint64(44), s.NextSequence())
	require.NoError(t, s.Close())

	loaded, err := Load(path, pool, nil, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, loaded.Close()) })

	require.Equal(t, uint64(42), loaded.StartSequence())
	require.Equal(t, uint64(44), loaded.NextSequence())
}

func TestSegment_ProjectedSize(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 10)
	path := filepath.Join(t.TempDir(), "size.bin")
	s, err := Create(path, 0, pool, nil, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	size := s.ProjectedSize([]byte("a"), []byte("bb"))
	want := HeaderSize + (2 * frame.HeaderSize) + int64(1+2)
	require.Equal(t, want, size)
}

func TestSegment_ReadFrontierPersistsAcrossLoad(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 10)
	path := filepath.Join(t.TempDir(), "segment.bin")
	s, err := Create(path, 10, pool, nil, false)
	require.NoError(t, err)

	require.NoError(t, s.Append([]byte("a"), []byte("b"), []byte("c")))

	scanner := NewScanner(s, frame.NewPool(16, 1024))
	require.NoError(t, scanner.Seek(12))
	require.NoError(t, s.SetReadOffset(scanner.ReadOffset()))
	require.NoError(t, s.Close())

	loaded, err := Load(path, pool, nil, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, loaded.Close()) })

	require.Equal(t, uint64(12), loaded.StartSequence())

	readScanner := NewScanner(loaded, frame.NewPool(16, 1024))
	require.True(t, readScanner.Next())
	seq, payload := readScanner.Value()
	require.Equal(t, uint64(12), seq)
	require.Equal(t, []byte("c"), payload)
}

func TestSegment_SetReadOffsetRejectsInvalidOffsetWithoutPersisting(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 10)
	path := filepath.Join(t.TempDir(), "segment.bin")
	s, err := Create(path, 10, pool, nil, false)
	require.NoError(t, err)

	require.NoError(t, s.Append([]byte("a"), []byte("b"), []byte("c")))

	_, eof, err := s.Find(s.NextSequence())
	require.NoError(t, err)
	invalid := eof + 1

	err = s.SetReadOffset(invalid)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidSegmentHeader)
	require.NoError(t, s.Close())

	loaded, err := Load(path, pool, nil, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, loaded.Close()) })

	require.Equal(t, uint64(10), loaded.StartSequence())

	st, statErr := os.Stat(path)
	require.NoError(t, statErr)
	require.GreaterOrEqual(t, st.Size(), int64(HeaderSize))

	scanner := NewScanner(loaded, frame.NewPool(16, 1024))
	require.True(t, scanner.Next())
	seq, _ := scanner.Value()
	require.Equal(t, uint64(10), seq)
	require.NoError(t, scanner.Err())
}

func TestSegment_Find(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 10)
	path := filepath.Join(t.TempDir(), "find.bin")
	s, err := Create(path, 10, pool, nil, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	require.NoError(t, s.Append([]byte("a"), []byte("bb"), []byte("ccc")))

	t.Run("first sequence", func(t *testing.T) {
		idx, off, err := s.Find(10)
		require.NoError(t, err)
		require.Equal(t, uint32(0), idx)
		require.Equal(t, int64(HeaderSize), off)
	})

	t.Run("middle sequence", func(t *testing.T) {
		idx, off, err := s.Find(11)
		require.NoError(t, err)
		require.Equal(t, uint32(1), idx)
		require.Equal(t, int64(HeaderSize)+frame.HeaderSize+int64(len([]byte("a"))), off)
	})

	t.Run("exclusive tail sequence", func(t *testing.T) {
		idx, off, err := s.Find(13)
		require.NoError(t, err)
		require.Equal(t, uint32(3), idx)

		want := int64(HeaderSize)
		want += frame.HeaderSize + int64(len([]byte("a")))
		want += frame.HeaderSize + int64(len([]byte("bb")))
		want += frame.HeaderSize + int64(len([]byte("ccc")))
		require.Equal(t, want, off)
	})

	t.Run("before start", func(t *testing.T) {
		_, _, err := s.Find(9)
		require.ErrorIs(t, err, ErrOutOfBounds)
	})

	t.Run("beyond tail", func(t *testing.T) {
		_, _, err := s.Find(14)
		require.ErrorIs(t, err, ErrOutOfBounds)
	})
}

func TestSegment_Truncate(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 10)
	path := filepath.Join(t.TempDir(), "truncate.bin")
	s, err := Create(path, 10, pool, nil, false)
	require.NoError(t, err)

	require.NoError(t, s.Append([]byte("a"), []byte("bb"), []byte("ccc")))
	require.Equal(t, uint64(13), s.NextSequence())

	require.NoError(t, s.Truncate(12))
	require.Equal(t, uint64(12), s.NextSequence())
	require.NoError(t, s.Close())

	loaded, err := Load(path, pool, nil, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, loaded.Close()) })

	require.Equal(t, uint64(12), loaded.NextSequence())

	sc := NewScanner(loaded, frame.NewPool(16, 1024))
	require.True(t, sc.Next())
	seq, payload := sc.Value()
	require.Equal(t, uint64(10), seq)
	require.Equal(t, []byte("a"), payload)

	require.True(t, sc.Next())
	seq, payload = sc.Value()
	require.Equal(t, uint64(11), seq)
	require.Equal(t, []byte("bb"), payload)

	require.False(t, sc.Next())
	require.NoError(t, sc.Err())
}

func TestSegment_LoadTornTailPreservesValidPrefix(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 1024)
	path := filepath.Join(t.TempDir(), "segment.bin")

	s, err := Create(path, 42, pool, nil, false)
	require.NoError(t, err)

	expected := [][]byte{
		[]byte("first"),
		[]byte("second"),
	}

	require.NoError(t, s.Append(expected...))
	require.NoError(t, s.Close())

	// append a torn frame to the end of the segment file, simulating a crash during write.
	appendTornFrame(t, path, 2, []byte("third"), frame.HeaderSize+2)

	loaded, err := Load(path, pool, nil, false)

	// short write detected
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.NotNil(t, loaded)

	// trim the torn frame and close the segment
	require.NoError(t, loaded.Trim())
	require.NoError(t, loaded.Close())

	// reload
	reloaded, err := Load(path, pool, nil, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reloaded.Close()) })

	// verify that the valid prefix is preserved
	require.Equal(t, uint64(44), reloaded.NextSequence())

	scanner := NewScanner(reloaded, frame.NewPool(1024, 1024))

	for i, expectedPayload := range expected {
		require.True(t, scanner.Next())

		seq, payload := scanner.Value()

		require.Equal(t, uint64(42+i), seq)
		require.Equal(t, expectedPayload, payload)
	}

	require.False(t, scanner.Next())
	require.NoError(t, scanner.Err())
}

func TestSegment_SyncOnAppend(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 10)
	path := filepath.Join(t.TempDir(), "segment.bin")
	s, err := Create(path, 0, pool, nil, true)
	require.NoError(t, err)
	require.NoError(t, s.Append([]byte("a"), []byte("b")))
	require.NoError(t, s.Close())

	loaded, err := Load(path, pool, nil, true)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, loaded.Close()) })

	require.Equal(t, uint64(2), loaded.NextSequence())
}

func appendTornFrame(
	t testing.TB,
	path string,
	index uint32,
	payload []byte,
	cut int,
) {
	t.Helper()

	header := frame.NewHeader(index, payload)

	frameBytes := make([]byte, 0, len(header)+len(payload))
	frameBytes = append(frameBytes, header[:]...)
	frameBytes = append(frameBytes, payload...)

	require.Greater(t, cut, 0)
	require.Less(t, cut, len(frameBytes))

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)

	_, err = file.Write(frameBytes[:cut])
	require.NoError(t, err)
	require.NoError(t, file.Sync())
	require.NoError(t, file.Close())
}
