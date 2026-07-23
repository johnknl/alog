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
	"errors"
	"path/filepath"
	"testing"

	"github.com/johnknl/alog/internal/frame"
	storagemocks "github.com/johnknl/alog/internal/storage/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreate_ClosesFileWhenSyncDirFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "00000000000000000000.bin")
	fs := storagemocks.NewMockFileSystem(t)
	f := storagemocks.NewMockFile(t)

	fs.EXPECT().Create(path).Return(f, nil)
	f.EXPECT().Write(mock.Anything).RunAndReturn(func(b []byte) (int, error) {
		return len(b), nil
	})
	f.EXPECT().Sync().Return(nil)
	fs.EXPECT().SyncDir(dir).Return(errors.New("dir sync failure"))
	f.EXPECT().Close().Return(nil)

	_, err := Create(path, 0, frame.NewPool(16, 1024), fs, false)
	require.ErrorContains(t, err, "dir sync failure")
}

func TestLoad_PropagatesOpenFault(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("open denied")
	fs := storagemocks.NewMockFileSystem(t)
	fs.EXPECT().Open("irrelevant.bin").Return(nil, wantErr)

	_, err := Load("irrelevant.bin", frame.NewPool(16, 1024), fs, false)
	require.ErrorIs(t, err, wantErr)
}
