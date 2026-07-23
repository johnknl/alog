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
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
)

// FileSystem abstracts filesystem operations used by alog packages.
type FileSystem interface {
	Open(name string) (File, error)
	Create(name string) (File, error)
	Stat(string) (os.FileInfo, error)
	MkdirAll(string, os.FileMode) error
	ReadDir(string) ([]os.DirEntry, error)
	Remove(string) error
	SyncDir(string) error
}

// File is an abstract open file handle used by segment and frame code.
type File interface {
	Name() string
	ReadAt([]byte, int64) (int, error)
	ReadFull([]byte) (int, error)
	Write([]byte) (int, error)
	WriteAt([]byte, int64) (int, error)
	WriteTo([][]byte) (int, error)
	Truncate(int64) error
	Seek(int64, int) (int64, error)
	Sync() error
	Close() error
}

var (
	_ File       = (*OSFile)(nil)
	_ FileSystem = (*OSFileSystem)(nil)
)

// OSFile adapts *os.File to the File interface.
type OSFile struct {
	f *os.File
}

// Close implements [File].
func (f *OSFile) Close() error {
	return f.f.Close()
}

// Name implements [File].
func (f *OSFile) Name() string {
	return f.f.Name()
}

// ReadAt implements [File].
func (f *OSFile) ReadAt(b []byte, off int64) (int, error) {
	return f.f.ReadAt(b, off)
}

// ReadFull implements [File].
func (f *OSFile) ReadFull(b []byte) (int, error) {
	return io.ReadFull(f.f, b)
}

// Seek implements [File].
func (f *OSFile) Seek(offset int64, whence int) (int64, error) {
	return f.f.Seek(offset, whence)
}

// Sync implements [File].
func (f *OSFile) Sync() error {
	return f.f.Sync()
}

// Truncate implements [File].
func (f *OSFile) Truncate(size int64) error {
	return f.f.Truncate(size)
}

// Write implements [File].
func (f *OSFile) Write(b []byte) (int, error) {
	return f.f.Write(b)
}

// WriteAt implements [File].
func (f *OSFile) WriteAt(b []byte, off int64) (int, error) {
	return f.f.WriteAt(b, off)
}

// WriteTo implements [File].
func (f *OSFile) WriteTo(parts [][]byte) (int, error) {
	buffers := net.Buffers(parts)
	n, err := buffers.WriteTo(f.f)
	if n > math.MaxInt {
		return 0, fmt.Errorf("write size exceeds int: %d", n)
	}

	return int(n), err
}

// Stat returns metadata for the underlying file.
func (f *OSFile) Stat() (os.FileInfo, error) {
	return f.f.Stat()
}

// OSFileSystem is a production FileSystem implementation backed by os package calls.
type OSFileSystem struct{}

// MkdirAll implements [FileSystem].
func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// ReadDir implements [FileSystem].
func (fs *OSFileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

// Remove implements [FileSystem].
func (fs *OSFileSystem) Remove(path string) error {
	return os.Remove(path)
}

// Stat implements [FileSystem].
func (fs *OSFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// SyncDir implements [FileSystem].
func (fs *OSFileSystem) SyncDir(path string) error {
	// #nosec G304 -- path comes from controlled storage root selection.
	dir, err := os.Open(path)
	if err != nil {
		return err
	}

	if err = dir.Sync(); err != nil {
		return errors.Join(err, dir.Close())
	}

	return dir.Close()
}

// Open opens an existing segment file in read-write mode.
func (fs *OSFileSystem) Open(name string) (File, error) {
	// Match segment load behavior: open read/write without create/append.
	f, err := os.OpenFile(name, os.O_RDWR, 0o600) // #nosec G304 -- storage root is caller controlled.
	if err != nil {
		return nil, err
	}

	return &OSFile{f: f}, nil
}

// Create creates a new segment file using exclusive create semantics.
func (fs *OSFileSystem) Create(name string) (File, error) {
	// Match segment create behavior: create exclusively read/write with 0600 mode.
	// #nosec G304 -- storage root is caller controlled.
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	return &OSFile{f: f}, nil
}
