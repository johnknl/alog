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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeader_NewHeader(t *testing.T) {
	t.Parallel()

	payload := []byte("hello")
	h := NewHeader(7, payload)

	require.Equal(t, uint32(len(payload)), h.PayloadLength())
	require.Equal(t, uint32(7), h.Index())
	require.NotZero(t, h.CRC32C())
}

func TestHeader_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid header and payload", func(t *testing.T) {
		t.Parallel()

		payload := []byte("ok")
		h := NewHeader(1, payload)
		require.NoError(t, h.Validate(payload))
	})

	t.Run("invalid payload length", func(t *testing.T) {
		t.Parallel()

		h := NewHeader(1, []byte("ok"))
		require.ErrorIs(t, h.Validate([]byte("different")), ErrInvalidFrameHeader)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		t.Parallel()

		payload := []byte("ok")
		h := NewHeader(1, payload)
		h[12] ^= 0xFF
		require.ErrorIs(t, h.Validate(payload), ErrInvalidChecksum)
	})
}
