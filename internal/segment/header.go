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
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math"

	"github.com/johnknl/alog/internal/storage"
)

const (
	segmentMagic   uint32 = 0x414C4F47 // "ALOG"
	segmentVersion uint32 = 1

	// HeaderSize is the size of a segment header in bytes
	HeaderSize = 64
)

var (
	// ErrInvalidSegmentHeader is returned when a segment header is invalid
	ErrInvalidSegmentHeader = errors.New("invalid segment header")
)

func sumCRC32C(offset [8]byte, generation [2]byte, reserved [2]byte) uint32 {
	summer := crc32.New(crc32cTable)
	_, _ = summer.Write(offset[:])
	_, _ = summer.Write(generation[:])
	_, _ = summer.Write(reserved[:])
	return summer.Sum32()
}

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

// MetaSlot is a 16-byte big-endian encoded array, representing a segment metadata slot.
//
// offset  size  field
// 0       8     read byte offset
// 8       2     generation
// 10      2     reserved
// 12      4     crc
type MetaSlot [16]byte

// BumpGeneration updates the generation number of the metadata slot and recalculates the checksum.
func (m MetaSlot) BumpGeneration() MetaSlot {
	newSlot := m
	binary.BigEndian.PutUint16(newSlot[8:10], m.Generation()+1)
	binary.BigEndian.PutUint32(newSlot[12:16], newSlot.Checksum())

	return newSlot
}

// ReadOffset returns the read offset from the metadata slot.
func (m MetaSlot) ReadOffset() (int64, error) {
	uoffset := binary.BigEndian.Uint64(m[0:8])
	if uoffset > math.MaxInt64 {
		return 0, ErrInvalidSegmentHeader
	}

	return int64(uoffset), nil
}

// Generation returns the generation number from the metadata slot
func (m MetaSlot) Generation() uint16 {
	return binary.BigEndian.Uint16(m[8:10])
}

// CRC returns the stored CRC checksum from the metadata slot
func (m MetaSlot) CRC() uint32 {
	return binary.BigEndian.Uint32(m[12:16])
}

// Checksum calculates the CRC checksum for the metadata slot
func (m MetaSlot) Checksum() uint32 {
	return sumCRC32C([8]byte(m[0:8]), [2]byte(m[8:10]), [2]byte(m[10:12]))
}

// Valid checks if the metadata slot is valid by comparing the stored CRC with the calculated checksum
func (m MetaSlot) Valid() bool {
	return m.Checksum() == m.CRC()
}

// NewerThan compares generation numbers as signed integers to handle wrap-around.
// #nosec G115 -- uint16 subtraction intentionally wraps; int16 sign interprets recency across wrap.
func (m MetaSlot) NewerThan(other MetaSlot) bool {
	return int16(m.Generation()-other.Generation()) > 0
}

// Header is a 64-byte big-endian encoded array, representing a segment header.
//
// The segment header is designed for durability, with two copies of
// the writable metadata. This allows for recovery in case of corruption
// or partial writes.
//
// On load the segment will read both copies and use a valid segment with
// the highest generation number. On write the segment will durably write
// the unused copy with a generation number one higher than the current copy.
//
// offset  size  field
// 0       4     magic
// 4       4     version
// 8       8     start sequence
// 16      8     read byte offset 1
// 20      2     generation 1
// 24      2     reserved 1
// 28      4     crc 1
// 32      8     read byte offset 2
// 36      2     generation 2
// 40      2     reserved 2
// 44      4     crc 2
// 48      16    reserved
type Header [64]byte

// Magic returns the magic number from the segment header
func (h Header) Magic() uint32 {
	return binary.BigEndian.Uint32(h[0:4])
}

// Version returns the format version from the segment header
func (h Header) Version() uint32 {
	return binary.BigEndian.Uint32(h[4:8])
}

// StartSequence returns the first sequence number in the segment
func (h Header) StartSequence() uint64 {
	return binary.BigEndian.Uint64(h[8:16])
}

// Valid checks if the magic and version headers match the
// library constants
func (h Header) Valid() bool {
	return h.Magic() == segmentMagic &&
		h.Version() == segmentVersion
}

// Meta returns the newest valid metadata from the segment header.
func (h Header) Meta() (MetaSlot, error) {
	a := MetaSlot(h[16:32])
	b := MetaSlot(h[32:48])

	vA := a.Valid()
	vB := b.Valid()

	switch {
	case vA && vB:
		if a.NewerThan(b) {
			return a, nil
		}

		return b, nil

	case vA:
		return a, nil

	case vB:
		return b, nil

	default:
		return MetaSlot{}, ErrInvalidSegmentHeader
	}
}

// WriteReadOffset writes the read offset to the segment header
// This will use the dual metadata slots mechanism to ensure durability.
func (h Header) WriteReadOffset(offset int64, f storage.File, slotPtr *MetaSlot) error {
	var headerOffset int64 = 16
	if slotPtr.Generation()%2 == 0 {
		headerOffset = 32
	}

	if offset < 0 {
		return fmt.Errorf("negative read offset: %d", offset)
	}

	slotVal := slotPtr.BumpGeneration()
	binary.BigEndian.PutUint64(slotVal[0:8], uint64(offset))
	binary.BigEndian.PutUint32(slotVal[12:16], slotVal.Checksum())

	if _, err := f.WriteAt(slotVal[:], headerOffset); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	*slotPtr = slotVal
	return nil
}

// NewHeader constructs a SegmentHeader for a new segment.
func NewHeader(startSequence uint64) Header {
	var h Header

	binary.BigEndian.PutUint32(h[0:4], segmentMagic)
	binary.BigEndian.PutUint32(h[4:8], segmentVersion)
	binary.BigEndian.PutUint64(h[8:16], startSequence)

	binary.BigEndian.PutUint64(h[16:24], HeaderSize)
	crc1 := (MetaSlot)(h[16:32]).Checksum()

	binary.BigEndian.PutUint32(h[28:32], crc1)

	return h
}
