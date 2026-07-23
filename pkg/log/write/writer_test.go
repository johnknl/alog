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

package write

import (
	"context"
	"errors"
	"testing"
	"time"

	logpkg "github.com/johnknl/alog/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestWriter_Append(t *testing.T) {
	t.Parallel()

	l := newLoadedLog(t, 2, 1024)
	t.Cleanup(func() { require.NoError(t, l.Close()) })

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	w := NewWriter(l, 1, 1, time.Hour)
	w.Start(ctx)

	seq0, err := w.Append(ctx, []byte("a"))
	require.NoError(t, err)
	require.Equal(t, uint64(0), seq0)

	seq1, err := w.Append(ctx, []byte("b"))
	require.NoError(t, err)
	require.Equal(t, uint64(1), seq1)
}

func TestWriter_AppendContextCanceled(t *testing.T) {
	t.Parallel()

	l := newLoadedLog(t, 2, 1024)
	t.Cleanup(func() { require.NoError(t, l.Close()) })

	ctx, cancel := context.WithCancel(t.Context())
	w := NewWriter(l, 1, 1, time.Hour)
	w.Start(ctx)

	canceledCtx, canceled := context.WithCancel(t.Context())
	canceled()

	_, err := w.Append(canceledCtx, []byte("x"))
	require.ErrorIs(t, err, context.Canceled)

	cancel()
}

func TestWriter_Stopped(t *testing.T) {
	t.Parallel()

	l := newLoadedLog(t, 2, 1024)
	t.Cleanup(func() { require.NoError(t, l.Close()) })

	ctx, cancel := context.WithCancel(t.Context())
	w := NewWriter(l, 1, 1, time.Hour)
	w.Start(ctx)
	cancel()

	require.Eventually(t, func() bool {
		appendCtx, appendCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer appendCancel()
		_, err := w.Append(appendCtx, []byte("x"))
		return errors.Is(err, ErrStopped)
	}, time.Second, 10*time.Millisecond)
}

func TestWriter_SegmentFull(t *testing.T) {
	t.Parallel()

	l := newLoadedLog(t, 1, 100)
	t.Cleanup(func() { require.NoError(t, l.Close()) })

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	w := NewWriter(l, 1, 1, time.Hour)
	w.Start(ctx)

	_, err := w.Append(ctx, []byte("a"))
	require.NoError(t, err)
	_, err = w.Append(ctx, []byte("b"))
	require.NoError(t, err)
	_, err = w.Append(ctx, []byte("c"))
	require.ErrorIs(t, err, logpkg.ErrSegmentFull)
}

func TestStartWriter(t *testing.T) {
	t.Parallel()

	l := newLoadedLog(t, 2, 1024)
	t.Cleanup(func() { require.NoError(t, l.Close()) })

	ctx, cancel := context.WithCancel(t.Context())
	w := StartWriter(ctx, l, logpkg.WriteBufferOptions{
		MaxLength: 1,
		MaxSize:   1,
		MaxDelay:  time.Hour,
	})

	seq0, err := w.Append(ctx, []byte("a"))
	require.NoError(t, err)
	seq1, err := w.Append(ctx, []byte("b"))
	require.NoError(t, err)
	require.Equal(t, uint64(0), seq0)
	require.Equal(t, uint64(1), seq1)

	cancel()

	require.Eventually(t, func() bool {
		appendCtx, appendCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer appendCancel()
		_, appendErr := w.Append(appendCtx, []byte("c"))
		return errors.Is(appendErr, ErrStopped)
	}, time.Second, 10*time.Millisecond)
}

func newLoadedLog(t *testing.T, maxSegments int, maxSegmentSize int64) *logpkg.Log {
	t.Helper()

	l, err := logpkg.Load(t.TempDir(), logpkg.Options{
		Storage: logpkg.StorageOptions{
			MaxDiskSize:    0,
			MaxSegmentSize: maxSegmentSize,
			MaxSegments:    maxSegments,
		},
	})
	require.NoError(t, err)

	return l
}
