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

package journal

import "github.com/johnknl/alog/pkg/log"

// Journal represents a persistent append-only log that can be used to store and
// retrieve records in a sequential manner.
type Journal struct {
	log *log.Log
}

// New creates a new Journal instance with the given directory and segment size.
// startSequence applies when creating a new empty log; existing logs keep their
// on-disk sequence space.
func New(directory string, maxSegmentSize int64, startSequence uint64) (*Journal, error) {
	l, err := log.New(log.Options{
		Storage: log.StorageOptions{
			StartSequence:  startSequence,
			MaxDiskSize:    0,
			MaxSegmentSize: maxSegmentSize,
			MaxSegments:    0, // No limit on the number of segments
		},
	})
	if err != nil {
		return nil, err
	}

	if err := l.Load(directory); err != nil {
		return nil, err
	}

	return &Journal{log: l}, nil
}

// Range over the archive from start to end passing the sequence numbers and payloads
// to passed callback. The payload slices should not be used outside of the callback.
func (a *Journal) Range(startSeq uint64, endSeq uint64, fn func(seq uint64, payload []byte) error) error {
	return a.log.Range(startSeq, endSeq, fn)
}

// Append appends the given payloads to the log and returns the sequence number of the last appended record.
func (a *Journal) Append(payloads ...[]byte) (uint64, error) {
	return a.log.Append(payloads...)
}

// FirstSequence returns the first retained sequence number and whether any records exist.
func (a *Journal) FirstSequence() (uint64, bool) {
	return a.log.FirstSequence()
}

// LastSequence returns the last retained sequence number and whether any records exist.
func (a *Journal) LastSequence() (uint64, bool) {
	return a.log.LastSequence()
}

// TruncateBefore removes all records with sequence numbers less than the specified seq from the log.
func (a *Journal) TruncateBefore(seq uint64) error {
	return a.log.TruncateBefore(seq)
}

// TruncateAfter removes all records with sequence numbers greater than the specified seq from the log.
func (a *Journal) TruncateAfter(seq uint64) error {
	return a.log.TruncateAfter(seq)
}

// Sync flushes any buffered data to disk, ensuring that all appended records are persisted.
func (a *Journal) Sync() error {
	return a.log.Sync()
}

// Close closes the underlying log resources.
func (a *Journal) Close() error {
	return a.log.Close()
}
