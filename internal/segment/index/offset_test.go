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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOffsetIndex_TruncateAfterKeepsFloorEntry(t *testing.T) {
	t.Parallel()

	var idx OffsetIndex
	idx.Set(0, 64)
	idx.Set(256, 4096)
	idx.Set(512, 8192)

	idx.TruncateAfter(256)

	frameIdx, offset, ok := idx.ForFrameIndex(256)
	require.True(t, ok)

	require.Equal(t, uint32(256), frameIdx)
	require.Equal(t, int64(4096), offset)
}

func TestOffsetIndex_SparseSetOnlyAddsStrideBoundaries(t *testing.T) {
	t.Parallel()

	var idx OffsetIndex
	idx.SparseSet(1, 128)
	idx.SparseSet(255, 2048)
	idx.SparseSet(256, 4096)

	frameIdx, offset, ok := idx.ForFrameIndex(300)
	require.True(t, ok)
	require.Equal(t, uint32(256), frameIdx)
	require.Equal(t, int64(4096), offset)

	_, _, ok = idx.ForFrameIndex(255)
	require.False(t, ok)
}

func TestOffsetIndex_TruncateBeforeKeepsFloorEntry(t *testing.T) {
	t.Parallel()

	var idx OffsetIndex
	idx.Set(0, 64)
	idx.Set(256, 4096)
	idx.Set(512, 8192)

	idx.TruncateBefore(300)

	frameIdx, offset, ok := idx.ForFrameIndex(300)
	require.True(t, ok)
	require.Equal(t, uint32(256), frameIdx)
	require.Equal(t, int64(4096), offset)

	frameIdx, offset, ok = idx.ForFrameIndex(512)
	require.True(t, ok)
	require.Equal(t, uint32(512), frameIdx)
	require.Equal(t, int64(8192), offset)
}

func TestOffsetIndex_ForOffsetAndSetUpdate(t *testing.T) {
	t.Parallel()

	var idx OffsetIndex
	idx.Set(256, 5000)
	idx.Set(0, 64)
	idx.Set(512, 9000)
	idx.Set(256, 6000)

	frameIdx, offset, ok := idx.ForOffset(5999)
	require.True(t, ok)
	require.Equal(t, uint32(0), frameIdx)
	require.Equal(t, int64(64), offset)

	frameIdx, offset, ok = idx.ForOffset(6000)
	require.True(t, ok)
	require.Equal(t, uint32(256), frameIdx)
	require.Equal(t, int64(6000), offset)

	frameIdx, offset, ok = idx.ForOffset(10000)
	require.True(t, ok)
	require.Equal(t, uint32(512), frameIdx)
	require.Equal(t, int64(9000), offset)
}
