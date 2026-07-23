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

package frame

import (
	"encoding/binary"
	"hash/crc32"
	"math"
)

const (
	// HeaderSize is the size of a frame header in bytes
	HeaderSize = 16
)

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

// Header is a 16-byte big-endian encoded array, representing a frame header.
//
// offset  size  field
// 0       4     payload length
// 4       4     index (seq = segment start + index)
// 8       4     reserved
// 12      4     CRC32C
type Header [16]byte

// NextOffset returns the next offset after the current frame, given the current offset
func (h Header) NextOffset(offset int64) int64 {
	return offset + HeaderSize + int64(h.PayloadLength())
}

// PayloadLength returns the payload length from the frame header
func (h Header) PayloadLength() uint32 {
	return binary.BigEndian.Uint32(h[0:4])
}

// Index returns the index from the frame header
func (h Header) Index() uint32 {
	return binary.BigEndian.Uint32(h[4:8])
}

// CRC32C returns the CRC32C checksum from the frame header
func (h Header) CRC32C() uint32 {
	return binary.BigEndian.Uint32(h[12:16])
}

// Validate checks if the frame header is valid for the given payload
func (h Header) Validate(payload []byte) error {
	size := len(payload)
	if size > math.MaxUint32 {
		return ErrPayloadTooLarge
	}

	if h.PayloadLength() != uint32(size) {
		return ErrInvalidFrameHeader
	}

	if h.CRC32C() != sumCRC32C(h, payload) {
		return ErrInvalidChecksum
	}

	return nil
}

func sumCRC32C(header Header, payload []byte) uint32 {
	summer := crc32.New(crc32cTable)
	_, _ = summer.Write(header[:12])
	_, _ = summer.Write(payload)
	return summer.Sum32()
}

// NewHeader constructs a new FrameHeader
func NewHeader(index uint32, payload []byte) Header {
	var h Header

	// #nosec G115 -- payload length is validated by callers before header construction.
	binary.BigEndian.PutUint32(h[0:4], uint32(len(payload)))
	binary.BigEndian.PutUint32(h[4:8], index)
	binary.BigEndian.PutUint32(h[12:16], sumCRC32C(h, payload))

	return h
}
