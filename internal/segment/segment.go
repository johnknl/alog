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
	"fmt"
	"io"
	"math"
	"path/filepath"

	"github.com/johnknl/alog/internal/frame"
	"github.com/johnknl/alog/internal/segment/index"
	"github.com/johnknl/alog/internal/storage"
)

// Segment represents a segment in the append-only log.
// It should not be used concurrently by multiple goroutines.
type Segment struct {
	file        storage.File
	reader      *frame.Reader
	offsetIndex index.OffsetIndex

	writeOffset  int64
	startOffset  int64
	writeIndex   uint32
	startIndex   uint32
	syncOnAppend bool

	header Header
	meta   MetaSlot
}

// Name returns the name of the segment file.
func (s *Segment) Name() string {
	return s.file.Name()
}

func newSegment(file storage.File, header Header, pool *frame.Pool, syncOnAppend bool) *Segment {
	s := &Segment{
		file:         file,
		reader:       frame.NewReader(file, pool, math.MaxUint32),
		header:       header,
		writeOffset:  HeaderSize, // start after the header
		startOffset:  HeaderSize,
		startIndex:   0,
		syncOnAppend: syncOnAppend,
	}

	s.anchorSparseIndex()

	return s
}

// Load loads a segment from the given path and returns a Segment instance.
func Load(path string, pool *frame.Pool, fs storage.FileSystem, syncOnAppend bool) (segment *Segment, err error) {
	if fs == nil {
		fs = &storage.OSFileSystem{}
	}

	file, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if segment != nil {
				// do not close the file if we are returning a segment
				return
			}

			err = errors.Join(err, file.Close())
		}
	}()

	var header Header
	_, err = file.ReadFull(header[:])
	if err != nil {
		return nil, err
	}

	if !header.Valid() {
		return nil, ErrInvalidSegmentHeader
	}

	meta, err := header.Meta()
	if err != nil {
		return nil, err
	}

	segment = newSegment(file, header, pool, syncOnAppend)
	segment.meta = meta

	startOffset, err := meta.ReadOffset()
	if err != nil {
		return nil, err
	}

	metaOffset := max(startOffset, int64(HeaderSize))
	readIdx, err := segment.indexForOffset(metaOffset)
	if err != nil {
		return nil, err
	}

	segment.startOffset = metaOffset
	segment.startIndex = readIdx

	// refresh sparse anchors now that logical read start is known
	segment.anchorSparseIndex()

	idx, offset, scanErr := segment.scanFramesFromReadStart()
	if scanErr != nil {
		if errors.Is(scanErr, io.ErrUnexpectedEOF) {
			// return the segment so caller may trim
			// we can't do this here because we don't know
			// if this is the last segment or not
			// since ordering is derived from segment
			// headers and not file names, the caller
			// also does not know until all segments
			// are loaded

			// set the write offset and index to the
			// last valid frame
			// using Append() after this will overwrite
			// (part of) the torn frame
			// trimming is not really required
			// for integrity since size checks are based
			// on write offset, not Stat()
			segment.writeOffset = offset
			segment.writeIndex = idx
			segment.offsetIndex.Set(segment.writeIndex, segment.writeOffset)

			return segment, scanErr
		}

		return nil, scanErr
	}

	segment.writeOffset = offset
	segment.writeIndex = idx
	// keep terminal anchor for exclusive-tail seek and exact tail lookups
	segment.offsetIndex.Set(segment.writeIndex, segment.writeOffset)

	return segment, nil
}

// BuildIndex scans the readable frame range, validates per-frame integrity,
// and rebuilds sparse index checkpoints as a side effect.
func (s *Segment) BuildIndex() error {
	_, _, err := s.scanFramesFromReadStart()
	return err
}

func (s *Segment) scanFramesFromReadStart() (idx uint32, offset int64, err error) {
	idx = s.startIndex
	offset = s.startOffset

	for {
		borrowed, readErr := s.reader.Read(idx, offset)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return idx, offset, nil
			}

			return idx, offset, readErr
		}

		if err = borrowed.Validate(); err != nil {
			borrowed.Return()
			return idx, offset, err
		}

		s.offsetIndex.SparseSet(idx, offset)
		offset = borrowed.Header.NextOffset(offset)
		idx++
		borrowed.Return()
	}
}

// anchorSparseIndex anchors the sparse index with the current start and write positions.
func (s *Segment) anchorSparseIndex() {
	s.offsetIndex.Set(0, HeaderSize)
	s.offsetIndex.Set(s.startIndex, s.startOffset)
	s.offsetIndex.Set(s.writeIndex, s.writeOffset)
}

func (s *Segment) offsetForIndex(target uint32) (int64, error) {
	if target == 0 {
		return HeaderSize, nil
	}

	idx, offset, ok := s.offsetIndex.ForFrameIndex(target)
	if !ok {
		idx = 0
		offset = HeaderSize
	}

	// scan forward from sparse floor to exact target index
	var h frame.Header
	var err error

	for idx < target {
		err = frame.ReadHeader(s.file, idx, offset, &h)
		if err != nil {
			return 0, err
		}

		offset = h.NextOffset(offset)
		idx++
	}

	return offset, nil
}

func (s *Segment) indexForOffset(target int64) (uint32, error) {
	if target == HeaderSize {
		return 0, nil
	}

	idx, offset, ok := s.offsetIndex.ForOffset(target)
	if !ok {
		idx = 0
		offset = HeaderSize
	}
	// scan forward from sparse floor until crossing target offset

	var h frame.Header
	var err error

	for offset < target {
		err = frame.ReadHeader(s.file, idx, offset, &h)
		if err != nil {
			return 0, err
		}

		offset = h.NextOffset(offset)
		idx++
	}

	if offset != target {
		return 0, ErrInvalidSegmentHeader
	}

	return idx, nil
}

// AppendSize returns the physical growth in bytes for appending payloads.
func AppendSize(payloads ...[]byte) int64 {
	var size int64
	for _, payload := range payloads {
		size += frame.HeaderSize + int64(len(payload))
	}

	return size
}

// ProjectedSize returns the projected size of the segment after
// appending the given payloads.
func (s *Segment) ProjectedSize(payloads ...[]byte) int64 {
	return s.writeOffset + AppendSize(payloads...)
}

// Size returns the physical size represented by this segment.
func (s *Segment) Size() int64 {
	return s.writeOffset
}

// Find the given sequence number in the segment and returns
// the offset of the frame header for that sequence number.
func (s *Segment) Find(seq uint64) (idx uint32, offset int64, err error) {
	if seq < s.StartSequence() {
		return 0, 0, ErrOutOfBounds
	}

	diff := seq - s.header.StartSequence()
	if diff > math.MaxUint32 {
		return 0, 0, ErrOutOfBounds
	}

	idx = uint32(diff)
	if idx > s.writeIndex {
		return 0, 0, ErrOutOfBounds
	}

	offset, err = s.offsetForIndex(idx)
	return
}

// Truncate truncates the tail of the segment file to the given seq.
func (s *Segment) Truncate(seq uint64) error {
	idx, offset, err := s.Find(seq)
	if err != nil {
		return err
	}

	if err := s.file.Truncate(offset); err != nil {
		return err
	}

	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync segment file after truncation: %w", err)
	}

	s.writeOffset = offset
	s.writeIndex = idx

	if s.startIndex > s.writeIndex {
		s.startIndex = s.writeIndex
		s.startOffset = s.writeOffset
	}

	s.offsetIndex.TruncateAfter(s.writeIndex)
	s.anchorSparseIndex()

	return nil
}

// Trim truncates the segment file to the current offset,
// effectively removing any data not consistent with current
// state tracking of the segment.
func (s *Segment) Trim() error {
	err := s.file.Truncate(s.writeOffset)
	if err != nil {
		return err
	}

	if err = s.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync segment file after truncation: %w", err)
	}

	return nil
}

// Create a new segment file at the given path with the specified start sequence number.
// This also syncs the parent directory.
func Create(
	path string,
	startSeq uint64,
	pool *frame.Pool,
	fs storage.FileSystem,
	syncOnAppend bool,
) (*Segment, error) {
	if fs == nil {
		fs = &storage.OSFileSystem{}
	}

	header := NewHeader(startSeq)

	file, err := fs.Create(path)
	if err != nil {
		return nil, err
	}

	_, err = file.Write(header[:])
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}

	if err = file.Sync(); err != nil {
		return nil, errors.Join(err, file.Close())
	}

	if syncErr := fs.SyncDir(filepath.Dir(path)); syncErr != nil {
		return nil, errors.Join(syncErr, file.Close())
	}

	segment := newSegment(file, header, pool, syncOnAppend)
	meta, err := header.Meta()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	segment.meta = meta

	return segment, nil
}

// NextSequence returns the next sequence number that will be assigned to the next appended data.
func (s *Segment) NextSequence() uint64 {
	return s.header.StartSequence() + uint64(s.writeIndex)
}

// StartSequence returns the starting sequence number of the segment.
func (s *Segment) StartSequence() uint64 {
	return s.header.StartSequence() + uint64(s.startIndex)
}

// BaseSequence returns the first physical sequence number of the segment header.
func (s *Segment) BaseSequence() uint64 {
	return s.header.StartSequence()
}

// SetReadOffset persists the read offset.
func (s *Segment) SetReadOffset(offset int64) error {
	readIndex, err := s.indexForOffset(offset)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return ErrInvalidSegmentHeader
		}

		return err
	}

	if err := s.header.WriteReadOffset(offset, s.file, &s.meta); err != nil {
		return err
	}

	s.startOffset = offset
	s.startIndex = readIndex

	s.offsetIndex.TruncateBefore(s.startIndex)
	s.anchorSparseIndex()

	return nil
}

// ReadStart returns the read offset of the segment
func (s *Segment) ReadStart() int64 {
	return s.startOffset
}

// ReadStartIndex returns the first readable index in the segment.
func (s *Segment) ReadStartIndex() uint32 {
	return s.startIndex
}

// Append appends data to the segment.
//
// This should not be called concurrently with other append operations on the same segment.
// This internally uses the writev syscall to avoid allocating extra slices while still writing
// the full batch using less syscalls (depending on IOV_MAX).
//
// A failing write or sync will put the segment in an unrecoverable state, but it will
// automatically be truncated to the last valid offset before returning the error.
func (s *Segment) Append(payloads ...[]byte) error {
	if len(payloads) == 0 {
		return nil
	}

	var idx = s.writeIndex
	var offset = s.writeOffset
	bufs := make([][]byte, 0, len(payloads)*2)

	for _, payload := range payloads {
		length := len(payload)
		if length > math.MaxUint32 {
			return frame.ErrPayloadTooLarge
		}

		header := frame.NewHeader(idx, payload)

		bufs = append(bufs, header[:])
		bufs = append(bufs, payload)

		idx++
		offset += int64(len(header) + length)
	}

	persist := func() error {
		if _, err := s.file.Seek(s.writeOffset, io.SeekStart); err != nil {
			return err
		}

		_, err := s.file.WriteTo(bufs) // using writev syscall, see writev_unix.go
		if err != nil {
			return err
		}

		if s.syncOnAppend {
			if err = s.file.Sync(); err != nil {
				return fmt.Errorf("failed to sync segment file after append: %w", err)
			}
		}

		return nil
	}

	if err := persist(); err != nil {
		if truncErr := s.Trim(); truncErr != nil {
			return errors.Join(err, truncErr)
		}

		return err
	}

	idxStart := s.writeIndex
	ofs := s.writeOffset
	for _, payload := range payloads {
		// sample sparse checkpoints from just-persisted append positions
		s.offsetIndex.SparseSet(idxStart, ofs)
		idxStart++
		ofs += frame.HeaderSize + int64(len(payload))
	}

	s.writeIndex = idx
	s.writeOffset = offset

	// keep terminal anchor up to date
	s.offsetIndex.Set(s.writeIndex, s.writeOffset)

	return nil
}

// Sync flushes file contents and metadata to stable storage.
func (s *Segment) Sync() error {
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync segment file: %w", err)
	}

	return nil
}

// Close closes the file handle
func (s *Segment) Close() error {
	return s.file.Close()
}
