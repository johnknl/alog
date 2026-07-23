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

func newLoadedChain(t *testing.T, maxSegments int, maxSegmentSize int64) *Chain {
	t.Helper()
	return newLoadedChainWithDiskBudget(t, maxSegments, maxSegmentSize, 0)
}

func newLoadedChainWithDiskBudget(t *testing.T, maxSegments int, maxSegmentSize int64, maxDiskSize int64) *Chain {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(dir, 0o750))

	pool := frame.NewPool(1024, 10)
	chain, err := NewChain(maxSegmentSize, maxDiskSize, maxSegments, false, nil)
	require.NoError(t, err)
	require.NoError(t, chain.Load(dir, pool, nil))

	return chain
}

func createSegmentAt(t *testing.T, dir string, startSeq uint64) *Segment {
	t.Helper()

	pool := frame.NewPool(1024, 10)
	path := filepath.Join(dir, segmentFileName(startSeq))
	s, err := Create(path, startSeq, pool, nil, false)
	require.NoError(t, err)

	return s
}
