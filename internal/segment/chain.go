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
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/johnknl/alog/internal/frame"
	"github.com/johnknl/alog/internal/storage"
)

// ErrSegmentFull is returned when the segment is full
// and a new one could not be created.
var (
	ErrSegmentFull   = errors.New("segment capacity exhausted")
	ErrDiskFull      = errors.New("max disk size exceeded")
	ErrBatchTooLarge = errors.New("append batch too large for empty segment")
)

const segmentFileExt = ".bin"

// Chain is responsible for loading existing segments from disk and managing them.
type Chain struct {
	fs             storage.FileSystem
	framePool      *frame.Pool
	directory      string
	segments       []*Segment
	diskSize       int64
	maxDiskSize    int64
	maxSegments    int
	maxSegmentSize int64
	syncOnAppend   bool
	mu             sync.RWMutex
}

// NewChain creates a new Chain with the given options. It validates the options and
// returns an error if they are invalid.
func NewChain(
	maxSegmentSize int64,
	maxDiskSize int64,
	maxSegments int,
	syncOnAppend bool,
	fs storage.FileSystem,
) (*Chain, error) {
	if fs == nil {
		fs = &storage.OSFileSystem{}
	}

	return &Chain{
		maxDiskSize:    maxDiskSize,
		maxSegments:    maxSegments,
		maxSegmentSize: maxSegmentSize,
		syncOnAppend:   syncOnAppend,
		fs:             fs,
	}, nil
}

// DiskSize returns the accounted physical size of retained segment files.
func (chain *Chain) DiskSize() int64 {
	chain.mu.RLock()
	defer chain.mu.RUnlock()

	return chain.diskSize
}

// ActiveSegments returns the list of readable segments.
func (chain *Chain) ActiveSegments() []*Segment {
	chain.mu.RLock()
	defer chain.mu.RUnlock()

	segments := make([]*Segment, len(chain.segments))
	copy(segments, chain.segments)
	return segments
}

// Head returns the first segment managed by the log.
func (chain *Chain) Head() *Segment {
	chain.mu.RLock()
	defer chain.mu.RUnlock()

	if len(chain.segments) == 0 {
		return nil
	}

	return chain.segments[0]
}

// Tail returns the last segment managed by the log.
func (chain *Chain) Tail() *Segment {
	chain.mu.RLock()
	defer chain.mu.RUnlock()

	return chain.segments[len(chain.segments)-1]
}

// Load scans the location for existing segments and load them into memory
// or creates the first segment if none exist.
func (chain *Chain) Load( // nolint:gocyclo // TODO: refactor to reduce complexity
	storageLocation string,
	pool *frame.Pool,
	fs storage.FileSystem,
) (err error) {
	if fs == nil {
		fs = chain.fs
	}
	if fs == nil {
		fs = &storage.OSFileSystem{}
	}
	chain.fs = fs

	chain.mu.Lock()
	defer chain.mu.Unlock()

	var files []os.DirEntry
	var trimSegment *Segment
	var s *Segment

	chain.framePool = pool
	chain.directory = storageLocation
	chain.segments = chain.segments[:0]
	chain.diskSize = 0

	defer func() {
		if err != nil {
			for _, segment := range chain.segments {
				err = errors.Join(err, segment.Close())
			}
		}
	}()

	switch chain.maxSegments {
	case 1:
		var stat os.FileInfo
		stat, err = chain.fs.Stat(storageLocation)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to stat directory %s: %w", storageLocation, err)
			}

			err = chain.fs.MkdirAll(storageLocation, 0o750)
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", chain.directory, err)
			}
		}
		if stat != nil && !stat.IsDir() {
			return fmt.Errorf("path %s is not a directory", storageLocation)
		}

		segmentPath := filepath.Join(storageLocation, segmentFileName(0))
		s, err = Load(segmentPath, pool, fs, chain.syncOnAppend)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				files, err = chain.fs.ReadDir(storageLocation)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						break
					}

					return err
				}

				for _, file := range files {
					if file.IsDir() || file.Type()&os.ModeSymlink != 0 || filepath.Ext(file.Name()) != segmentFileExt {
						continue
					}

					s, err = Load(filepath.Join(storageLocation, file.Name()), pool, fs, chain.syncOnAppend)
					if err != nil {
						return err
					}

					chain.segments = append(chain.segments, s)
				}

				if len(chain.segments) > 1 {
					return fmt.Errorf("too many segments: %d, max allowed: 1", len(chain.segments))
				}

				if len(chain.segments) == 0 {
					break
				}

				trimSegment = chain.segments[0]
				break
			}
			return err
		}
		// allow trimming if needed
		trimSegment = s
		chain.segments = append(chain.segments, s)

		files, err = chain.fs.ReadDir(storageLocation)
		if err != nil {
			return err
		}

		logCount := 0
		for _, file := range files {
			if file.IsDir() || file.Type()&os.ModeSymlink != 0 || filepath.Ext(file.Name()) != segmentFileExt {
				continue
			}
			logCount++
		}

		if logCount > 1 {
			return fmt.Errorf("too many segments: %d, max allowed: 1", logCount)
		}

	default:
		var stat os.FileInfo
		stat, err = chain.fs.Stat(storageLocation)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to stat directory %s: %w", storageLocation, err)
			}
			err = chain.fs.MkdirAll(storageLocation, 0750)
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", chain.directory, err)
			}
		}
		if stat != nil && !stat.IsDir() {
			return fmt.Errorf("path %s is not a directory", storageLocation)
		}

		files, err = chain.fs.ReadDir(storageLocation)
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if file.Type()&os.ModeSymlink != 0 {
				continue
			}
			if filepath.Ext(file.Name()) != segmentFileExt {
				continue
			}

			segmentPath := filepath.Join(storageLocation, file.Name())
			s, err = Load(segmentPath, pool, fs, chain.syncOnAppend)
			if err != nil {
				if trimSegment == nil && errors.Is(err, io.ErrUnexpectedEOF) {
					trimSegment = s
					goto Continue
				}

				return err
			}
		Continue:
			chain.segments = append(chain.segments, s)
		}

		if chain.maxSegments > 0 && len(chain.segments) > chain.maxSegments {
			return fmt.Errorf("too many segments: %d, max allowed: %d", len(chain.segments), chain.maxSegments)
		}

		// sort segments by start sequence number
		sort.Slice(chain.segments, func(i, j int) bool {
			return chain.segments[i].StartSequence() < chain.segments[j].StartSequence()
		})

		// validate that the segments are contiguous and do not overlap
		for i := 1; i < len(chain.segments); i++ {
			prevSegment := chain.segments[i-1]
			currSegment := chain.segments[i]

			if prevSegment.NextSequence() != currSegment.StartSequence() {
				return fmt.Errorf("segments are not contiguous: segment %d ends at %d, but segment %d starts at %d",
					i-1, prevSegment.NextSequence(), i, currSegment.StartSequence())
			}
		}

	}

	if len(chain.segments) == 0 {
		// if no segments exist, create the first segment
		firstSegmentPath := filepath.Join(chain.directory, segmentFileName(0))
		s, err = Create(firstSegmentPath, 0, pool, fs, chain.syncOnAppend)
		if err != nil {
			return err
		}

		chain.segments = append(chain.segments, s)
		chain.diskSize = s.Size()

		return nil
	}

	// if the last segment is incomplete, trim it
	if trimSegment != nil {
		if trimSegment != chain.segments[len(chain.segments)-1] {
			return fmt.Errorf("segment %s incomplete: %w", trimSegment.Name(), io.ErrUnexpectedEOF)
		}

		if err = trimSegment.Trim(); err != nil {
			return fmt.Errorf("failed to trim segment %s: %w", trimSegment.Name(), err)
		}
	}

	for _, s := range chain.segments {
		chain.diskSize += s.Size()
	}

	if chain.maxDiskSize > 0 && chain.diskSize > chain.maxDiskSize {
		return fmt.Errorf("loaded disk size %d exceeds MaxDiskSize %d: %w", chain.diskSize, chain.maxDiskSize, ErrDiskFull)
	}

	return nil
}

// Append appends payloads atomically with chain-level admission and accounting.
func (chain *Chain) Append(payload ...[]byte) (uint64, error) {
	chain.mu.Lock()
	defer chain.mu.Unlock()
	if len(payload) == 0 {
		return 0, nil
	}

	appendSize := AppendSize(payload...)
	if chain.maxSegmentSize > 0 && HeaderSize+appendSize > chain.maxSegmentSize {
		return 0, errors.Join(ErrSegmentFull, ErrBatchTooLarge)
	}

	lastSegment := chain.segments[len(chain.segments)-1]
	rotate := chain.maxSegmentSize > 0 && lastSegment.Size()+appendSize > chain.maxSegmentSize

	for {
		required := appendSize
		if rotate {
			required += HeaderSize
		}

		needReap := false
		if rotate && chain.maxSegments > 0 && len(chain.segments) >= chain.maxSegments {
			needReap = true
		}
		if chain.maxDiskSize > 0 && chain.diskSize+required > chain.maxDiskSize {
			needReap = true
		}

		if !needReap {
			break
		}

		if len(chain.segments) <= 1 {
			if chain.maxDiskSize > 0 && chain.diskSize+required > chain.maxDiskSize {
				return 0, ErrDiskFull
			}

			return 0, ErrSegmentFull
		}

		if err := chain.reapOldestLocked(); err != nil {
			return 0, err
		}

		lastSegment = chain.segments[len(chain.segments)-1]
		rotate = chain.maxSegmentSize > 0 && lastSegment.Size()+appendSize > chain.maxSegmentSize
	}

	if rotate {
		nextStartSeq := lastSegment.NextSequence()
		newSegmentPath := filepath.Join(chain.directory, segmentFileName(nextStartSeq))

		newSegment, err := Create(newSegmentPath, nextStartSeq, chain.framePool, chain.fs, chain.syncOnAppend)
		if err != nil {
			return 0, err
		}

		chain.segments = append(chain.segments, newSegment)
		chain.diskSize += newSegment.Size()
		lastSegment = newSegment
	}

	startSeq := lastSegment.NextSequence()
	if err := lastSegment.Append(payload...); err != nil {
		return 0, err
	}

	chain.diskSize += appendSize

	return startSeq + uint64(len(payload)) - 1, nil
}

// reapOldestLocked removes the oldest segment.
// Caller must hold chain.mu.
func (chain *Chain) reapOldestLocked() error {
	oldest := chain.segments[0]
	oldestSize := oldest.Size()

	if err := chain.fs.Remove(oldest.Name()); err != nil {
		return err
	}

	chain.segments = chain.segments[1:]
	chain.diskSize -= oldestSize

	return oldest.Close()
}

// TruncateBefore finds the segment that contains the given sequence number,
// removes all segments before it, and sets the start sequence of that segment
// to the given sequence number if needed.
func (chain *Chain) TruncateBefore(seq uint64) error {
	chain.mu.Lock()
	defer chain.mu.Unlock()

	if len(chain.segments) == 0 {
		return nil
	}

	keepIndex := sort.Search(len(chain.segments), func(i int) bool {
		return chain.segments[i].NextSequence() > seq
	})
	if keepIndex >= len(chain.segments) || chain.segments[keepIndex].StartSequence() > seq {
		keepIndex = -1
	}

	if keepIndex == -1 {
		if seq < chain.segments[len(chain.segments)-1].NextSequence() {
			return nil
		}

		lastIndex := len(chain.segments) - 1
		for i := range lastIndex {
			seg := chain.segments[i]
			segSize := seg.Size()
			path := seg.Name()
			if err := seg.Close(); err != nil {
				return err
			}
			if err := chain.fs.Remove(path); err != nil {
				return err
			}
			chain.diskSize -= segSize
		}

		last := chain.segments[lastIndex]
		if err := last.SetReadOffset(last.writeOffset); err != nil {
			return err
		}

		chain.segments = chain.segments[lastIndex:]

		return nil
	}

	for i := 0; i < keepIndex; i++ {
		seg := chain.segments[i]
		segSize := seg.Size()
		path := seg.Name()
		if err := seg.Close(); err != nil {
			return err
		}
		if err := chain.fs.Remove(path); err != nil {
			return err
		}
		chain.diskSize -= segSize
	}

	chain.segments = chain.segments[keepIndex:]

	segment := chain.segments[0]
	if segment.StartSequence() < seq {
		// find the read offset within the segment and persist it
		scanner := NewScanner(segment, nil)

		if err := scanner.Seek(seq); err != nil {
			return fmt.Errorf("failed to seek to sequence %d in segment %s: %w", seq, segment.Name(), err)
		}

		if err := segment.SetReadOffset(scanner.ReadOffset()); err != nil {
			return fmt.Errorf("failed to truncate segment %s: %w", segment.Name(), err)
		}
	}

	return nil
}

// TruncateAfter finds the segment that contains the given sequence number,
// removes all segments after it, and truncates that segment to the given sequence
// number if needed.
func (chain *Chain) TruncateAfter(seq uint64) error {
	chain.mu.Lock()
	defer chain.mu.Unlock()

	if len(chain.segments) == 0 {
		return nil
	}

	keepIdx := sort.Search(len(chain.segments), func(i int) bool {
		return chain.segments[i].NextSequence() > seq
	})
	if keepIdx >= len(chain.segments) || chain.segments[keepIdx].StartSequence() > seq {
		keepIdx = -1
	}

	if keepIdx == -1 {
		if seq < chain.segments[0].StartSequence() {
			for _, seg := range chain.segments {
				segSize := seg.Size()
				path := seg.Name()
				if err := seg.Close(); err != nil {
					return err
				}
				if err := chain.fs.Remove(path); err != nil {
					return err
				}
				chain.diskSize -= segSize
			}

			chain.segments = chain.segments[:0]

			firstSegmentPath := filepath.Join(chain.directory, segmentFileName(0))
			newSeg, err := Create(firstSegmentPath, 0, chain.framePool, chain.fs, chain.syncOnAppend)
			if err != nil {
				return err
			}

			chain.segments = append(chain.segments, newSeg)
			chain.diskSize += newSeg.Size()
		}

		return nil
	}

	for i := keepIdx + 1; i < len(chain.segments); i++ {
		seg := chain.segments[i]
		segSize := seg.Size()
		path := seg.Name()
		if err := seg.Close(); err != nil {
			return err
		}
		if err := chain.fs.Remove(path); err != nil {
			return err
		}
		chain.diskSize -= segSize
	}
	chain.segments = chain.segments[:keepIdx+1]

	keep := chain.segments[keepIdx]
	oldSize := keep.Size()
	if err := keep.Truncate(seq + 1); err != nil {
		return err
	}
	chain.diskSize -= oldSize - keep.Size()

	if keep.startOffset > keep.writeOffset {
		if err := keep.SetReadOffset(keep.writeOffset); err != nil {
			return err
		}
	}

	return nil
}

// Sync flushes file contents and metadata of all active segments to stable storage.
func (chain *Chain) Sync() error {
	chain.mu.RLock()
	segments := make([]*Segment, len(chain.segments))
	copy(segments, chain.segments)
	chain.mu.RUnlock()

	for _, seg := range segments {
		if err := seg.Sync(); err != nil {
			return err
		}
	}

	return nil
}

// Close closes all segments in the log and returns any errors that occurred during closing.
func (chain *Chain) Close() error {
	chain.mu.Lock()
	defer chain.mu.Unlock()

	var err error
	for _, segment := range chain.segments {
		err = errors.Join(err, segment.Close())
	}

	return err
}

func segmentFileName(startSeq uint64) string {
	return fmt.Sprintf("%020d%s", startSeq, segmentFileExt)
}
