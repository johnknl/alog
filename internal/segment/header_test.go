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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSegmentHeader_New(t *testing.T) {
	t.Parallel()

	t.Run("encodes magic version and start sequence", func(t *testing.T) {
		t.Parallel()

		const start = uint64(987654321)

		h := NewHeader(start)

		require.Equal(t, segmentMagic, h.Magic())
		require.Equal(t, segmentVersion, h.Version())
		require.Equal(t, start, h.StartSequence())
	})
}

func TestSegmentHeader_Valid(t *testing.T) {
	t.Parallel()

	t.Run("returns true for generated header", func(t *testing.T) {
		t.Parallel()

		h := NewHeader(1)
		require.True(t, h.Valid())
	})

	t.Run("returns false for invalid magic", func(t *testing.T) {
		t.Parallel()

		h := NewHeader(1)
		h[0] ^= 0xFF
		require.False(t, h.Valid())
	})

	t.Run("returns false for invalid version", func(t *testing.T) {
		t.Parallel()

		h := NewHeader(1)
		h[4] ^= 0xFF
		require.False(t, h.Valid())
	})
}

func TestMetaSlot_ReadOffset(t *testing.T) {
	t.Parallel()

	t.Run("returns offset for valid int64", func(t *testing.T) {
		t.Parallel()

		var m MetaSlot
		binary.BigEndian.PutUint64(m[0:8], uint64(HeaderSize))

		offset, err := m.ReadOffset()
		require.NoError(t, err)
		require.Equal(t, int64(HeaderSize), offset)
	})

	t.Run("returns error for offset overflow", func(t *testing.T) {
		t.Parallel()

		var m MetaSlot
		binary.BigEndian.PutUint64(m[0:8], ^uint64(0))

		_, err := m.ReadOffset()
		require.ErrorIs(t, err, ErrInvalidSegmentHeader)
	})
}
