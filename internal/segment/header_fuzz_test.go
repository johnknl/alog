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
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzMetaSlot_Basics(f *testing.F) {
	f.Add(make([]byte, 16))
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, byte(HeaderSize), 0, 1, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, raw []byte) {
		// normalize to one full metadata slot buffer
		if len(raw) < 16 {
			raw = append(raw, make([]byte, 16-len(raw))...)
		}
		raw = raw[:16]

		var slot MetaSlot
		copy(slot[:], raw)

		// probe accessors and validation on arbitrary slot bytes
		_ = slot.Valid()
		_, _ = slot.ReadOffset()
		_ = slot.Generation()
		_ = slot.Checksum()

		// generation increments should produce a valid checksummed slot
		bumped := slot.BumpGeneration()
		require.Equal(t, slot.Generation()+1, bumped.Generation())
		require.True(t, bumped.Valid())

		// a valid read offset plus matching checksum should validate
		binary.BigEndian.PutUint64(slot[0:8], uint64(HeaderSize))
		binary.BigEndian.PutUint32(slot[12:16], slot.Checksum())
		require.True(t, slot.Valid())

		// an out-of-range read offset should be rejected
		binary.BigEndian.PutUint64(slot[0:8], math.MaxUint64)
		binary.BigEndian.PutUint32(slot[12:16], slot.Checksum())
		_, err := slot.ReadOffset()
		require.Error(t, err)
	})
}

func FuzzHeader_Basics(f *testing.F) {
	f.Add(make([]byte, HeaderSize))
	seed := NewHeader(7)
	f.Add(seed[:])

	f.Fuzz(func(t *testing.T, raw []byte) {
		// normalize to one full header-size buffer
		if len(raw) < HeaderSize {
			raw = append(raw, make([]byte, HeaderSize-len(raw))...)
		}

		raw = raw[:HeaderSize]

		var h Header
		copy(h[:], raw)

		// probe generic validation and metadata decoding on arbitrary bytes
		_ = h.Valid()

		if m, err := h.Meta(); err == nil {
			// if Meta() succeeds, the returned metadata must be valid
			require.True(t, m.Valid())
		}

		// generated headers must be self-consistent
		generated := NewHeader(11)
		require.True(t, generated.Valid())

		m, err := generated.Meta()
		require.NoError(t, err)
		require.True(t, m.Valid())

		// mutating header bytes should invalidate checksum/integrity
		mutated := generated
		mutated[0] ^= 0xFF

		require.False(t, mutated.Valid())
	})
}

func FuzzHeader_MetadataSlotSelection(f *testing.F) {
	f.Add(uint64(12), uint16(1), uint16(2), byte(0))

	f.Fuzz(func(t *testing.T, startSeq uint64, genA uint16, genB uint16, slotSelect byte) {
		// start from a known-good header with two valid metadata slots
		h := NewHeader(startSeq)

		a := MetaSlot(h[16:32]).BumpGeneration()
		b := MetaSlot(h[32:48]).BumpGeneration()

		// force explicit generation values and recompute checksums
		binary.BigEndian.PutUint16(a[8:10], genA)
		binary.BigEndian.PutUint32(a[12:16], a.Checksum())
		binary.BigEndian.PutUint16(b[8:10], genB)
		binary.BigEndian.PutUint32(b[12:16], b.Checksum())

		if slotSelect&1 != 0 {
			// corrupt slot A
			a[0] ^= 0x01
		}
		if slotSelect&2 != 0 {
			// corrupt slot B
			b[0] ^= 0x01
		}

		// write mutated slots back into the header image
		copy(h[16:32], a[:])
		copy(h[32:48], b[:])

		// have the header select the correct slot based on validity
		meta, err := h.Meta()

		// assert at least one slot is valid
		if !a.Valid() && !b.Valid() {
			require.Error(t, err)
			return
		}

		require.NoError(t, err)
		require.True(t, meta.Valid())
	})
}
