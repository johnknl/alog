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
	"bytes"
	"math"
	"testing"

	"github.com/johnknl/alog/internal/frame"
	"github.com/stretchr/testify/require"
)

func TestScanner_NextBorrowValue(t *testing.T) {
	t.Parallel()

	chain := newLoadedChain(t, 2, 1024)
	t.Cleanup(func() { require.NoError(t, chain.Close()) })

	s := chain.Tail()
	require.NoError(t, s.Append([]byte("one"), []byte("two")))

	scanner := NewScanner(s, frame.NewPool(16, 1024), math.MaxUint32)

	require.True(t, scanner.Next())
	b := scanner.Borrow()
	require.Equal(t, "one", string(b.Payload))
	b.Return()

	require.True(t, scanner.Next())
	seq, payload := scanner.Value()
	require.Equal(t, uint64(1), seq)
	require.Equal(t, []byte("two"), payload)

	require.False(t, scanner.Next())
	require.NoError(t, scanner.Err())
}

func TestScanner_Seek(t *testing.T) {
	t.Parallel()

	chain := newLoadedChain(t, 2, 1024)
	t.Cleanup(func() { require.NoError(t, chain.Close()) })

	s := chain.Tail()
	require.NoError(t, s.Append([]byte("a"), []byte("b"), []byte("c")))

	scanner := NewScanner(s, frame.NewPool(16, 1024), math.MaxUint32)

	require.NoError(t, scanner.Seek(2))
	require.True(t, scanner.Next())
	seq, payload := scanner.Value()
	require.Equal(t, uint64(2), seq)
	require.Equal(t, []byte("c"), payload)

	require.NoError(t, scanner.Seek(0))
	require.True(t, scanner.Next())
	seq, payload = scanner.Value()
	require.Equal(t, uint64(0), seq)
	require.Equal(t, []byte("a"), payload)
}

func TestScanner_SeekOutOfBounds(t *testing.T) {
	t.Parallel()

	chain := newLoadedChain(t, 2, 1024)
	t.Cleanup(func() { require.NoError(t, chain.Close()) })

	scanner := NewScanner(chain.Tail(), frame.NewPool(16, 1024), math.MaxUint32)
	require.ErrorIs(t, scanner.Seek(^uint64(0)), ErrOutOfBounds)
}

func TestScanner_SeekBeyondEndPositionsAtEnd(t *testing.T) {
	t.Parallel()

	chain := newLoadedChain(t, 2, 1024)
	t.Cleanup(func() { require.NoError(t, chain.Close()) })

	s := chain.Tail()
	require.NoError(t, s.Append([]byte("a"), []byte("b")))

	scanner := NewScanner(s, frame.NewPool(16, 1024), math.MaxUint32)
	require.NoError(t, scanner.Seek(9999))
	require.False(t, scanner.Next())
	require.NoError(t, scanner.Err())
}

func TestScanner_SeekAfterReadOffsetAdvance(t *testing.T) {
	t.Parallel()

	chain := newLoadedChain(t, 2, 1024*1024)
	t.Cleanup(func() { require.NoError(t, chain.Close()) })

	s := chain.Tail()
	for i := range 1024 {
		require.NoError(t, s.Append([]byte{byte(i % 251)}))
	}

	set := NewScanner(s, frame.NewPool(16, 1024), math.MaxUint32)
	require.NoError(t, set.Seek(512))
	require.NoError(t, s.SetReadOffset(set.ReadOffset()))

	scanner := NewScanner(s, frame.NewPool(16, 1024), math.MaxUint32)
	require.NoError(t, scanner.Seek(512))
	require.True(t, scanner.Next())
	seq, _ := scanner.Value()
	require.Equal(t, uint64(512), seq)

	require.NoError(t, scanner.Seek(700))
	require.True(t, scanner.Next())
	seq, _ = scanner.Value()
	require.Equal(t, uint64(700), seq)
}

func TestScanner_LimitedPayloadReading(t *testing.T) {
	t.Parallel()

	chain := newLoadedChain(t, 2, 1024*1024)
	t.Cleanup(func() { require.NoError(t, chain.Close()) })

	large := bytes.Repeat([]byte{'x'}, 64)
	second := []byte("tail")

	s := chain.Tail()
	require.NoError(t, s.Append(large, second))

	scanner := NewScanner(s, frame.NewPool(16, 1024), 8)

	require.True(t, scanner.Next())
	seq, payload := scanner.Value()
	require.Equal(t, uint64(0), seq)
	require.Equal(t, large[:8], payload)

	require.True(t, scanner.Next())
	seq, payload = scanner.Value()
	require.Equal(t, uint64(1), seq)
	require.Equal(t, second, payload)

	require.False(t, scanner.Next())
	require.NoError(t, scanner.Err())
}
