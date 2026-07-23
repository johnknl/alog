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

package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOSFileSystem_CreateOpenAndWriteTo(t *testing.T) {
	t.Parallel()

	fs := &OSFileSystem{}
	path := filepath.Join(t.TempDir(), "file.bin")

	f, err := fs.Create(path)
	require.NoError(t, err)

	n, err := f.WriteTo([][]byte{[]byte("a"), []byte("bc")})
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())

	g, err := fs.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, g.Close()) })

	buf := make([]byte, 3)
	_, err = g.ReadAt(buf, 0)
	require.NoError(t, err)
	require.Equal(t, []byte("abc"), buf)
}

func TestOSFileSystem_CreateIsExclusive(t *testing.T) {
	t.Parallel()

	fs := &OSFileSystem{}
	path := filepath.Join(t.TempDir(), "exists.bin")

	f, err := fs.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	_, err = fs.Create(path)
	require.ErrorIs(t, err, os.ErrExist)
}

func TestOSFileSystem_SyncDirMissingPath(t *testing.T) {
	t.Parallel()

	fs := &OSFileSystem{}
	err := fs.SyncDir(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}
