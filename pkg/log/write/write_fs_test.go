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
	"os"
	"testing"
	"time"

	"github.com/johnknl/alog/internal/frame"
	"github.com/johnknl/alog/internal/segment"
	storagemocks "github.com/johnknl/alog/internal/storage/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWriter_AppendPropagatesSyncFailure(t *testing.T) {
	t.Parallel()

	pool := frame.NewPool(1024, 10)
	fs := storagemocks.NewMockFileSystem(t)
	file := storagemocks.NewMockFile(t)

	storageDir := t.TempDir()
	segPath := storageDir + "/00000000000000000000.bin"

	fs.EXPECT().Stat(storageDir).Return(nil, os.ErrNotExist)
	fs.EXPECT().MkdirAll(storageDir, os.FileMode(0o750)).Return(nil)
	fs.EXPECT().ReadDir(storageDir).Return([]os.DirEntry{}, nil)
	fs.EXPECT().Create(segPath).Return(file, nil)

	file.EXPECT().Write(mock.Anything).RunAndReturn(func(b []byte) (int, error) {
		return len(b), nil
	})
	file.EXPECT().Sync().Return(nil).Once()
	fs.EXPECT().SyncDir(storageDir).Return(nil)

	file.EXPECT().Seek(mock.Anything, mock.Anything).Return(int64(64), nil)
	file.EXPECT().WriteTo(mock.Anything).Return(0, nil)
	file.EXPECT().Sync().Return(errors.New("simulated fsync fault"))
	file.EXPECT().Truncate(int64(64)).Return(nil)
	file.EXPECT().Sync().Return(nil)
	file.EXPECT().Close().Return(nil)

	chain, err := segment.NewChain(0, 1024, 0, 2, true, fs)
	require.NoError(t, err)
	require.NoError(t, chain.Load(storageDir, pool, fs))
	t.Cleanup(func() { require.NoError(t, chain.Close()) })

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	w := NewWriter(chain, 1, 1, time.Hour)
	w.Start(ctx)

	_, err = w.Append(ctx, []byte("payload"))
	require.ErrorContains(t, err, "simulated fsync fault")
}
