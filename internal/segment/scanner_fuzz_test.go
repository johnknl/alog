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
	"testing"

	"github.com/johnknl/alog/internal/frame"
	"github.com/stretchr/testify/require"
)

// FuzzScanner_SeekBehavior fuzzes segment shape/read-start/query combinations
// and verifies Seek() behavior.
func FuzzScanner_SeekBehavior(f *testing.F) {
	f.Add(uint64(10), []byte{1, 2, 3, 4}, uint64(10))
	f.Add(uint64(10), []byte{1, 2, 3, 4}, uint64(14))

	f.Fuzz(func(t *testing.T, base uint64, shape []byte, query uint64) {
		// normalize sequence base and ensure at least one payload shape byte
		base %= 1024
		if len(shape) == 0 {
			shape = []byte{1}
		}

		path := t.TempDir() + "/find.bin"
		pool := frame.NewPool(64, 1<<20)
		s, err := Create(path, base, pool, nil, false)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, s.Close()) })

		// materialize a bounded payload sequence to shape scanner behavior
		for i := 0; i < len(shape) && i < 32; i++ {
			payloadLen := int(shape[i]%16) + 1
			payload := make([]byte, payloadLen)
			for j := range payload {
				payload[j] = byte(i + j)
			}
			require.NoError(t, s.Append(payload))
		}

		// optionally shift the readable window by persisting a read offset
		if s.NextSequence() > s.StartSequence()+1 {
			cutSeq := s.StartSequence() + uint64(shape[0])%(s.NextSequence()-s.StartSequence())
			scanner := NewScanner(s, frame.NewPool(64, 1<<20))
			require.NoError(t, scanner.Seek(cutSeq))
			require.NoError(t, s.SetReadOffset(scanner.ReadOffset()))
		}

		// seek outcome should match query position relative to bounds
		scanner := NewScanner(s, frame.NewPool(64, 1<<20))
		seekErr := scanner.Seek(query)

		if query < s.StartSequence() {
			require.ErrorIs(t, seekErr, ErrOutOfBounds)
			return
		}

		require.NoError(t, seekErr)
		if query >= s.NextSequence() {
			require.False(t, scanner.Next())
			require.NoError(t, scanner.Err())
			return
		}

		require.True(t, scanner.Next())
		seq, _ := scanner.Value()
		require.Equal(t, query, seq)
	})
}
