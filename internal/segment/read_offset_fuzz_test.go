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
	"testing"

	"github.com/johnknl/alog/internal/frame"
	"github.com/stretchr/testify/require"
)

// FuzzSegment_ReadOffsetRecovery persists candidate read offsets and verifies
// reload start-sequence recovery.
func FuzzSegment_ReadOffsetRecovery(f *testing.F) {
	f.Add(uint8(8), uint32(0))
	f.Add(uint8(8), uint32(1024))

	f.Fuzz(func(t *testing.T, nFrames uint8, inputOffset uint32) {
		// synthesize a bounded segment with deterministic one-byte payload frames
		n := int(nFrames) + 1
		path := fmt.Sprintf("%s/offset.bin", t.TempDir())
		pool := frame.NewPool(64, 1<<20)

		s, err := Create(path, 10, pool, nil, false)
		require.NoError(t, err)
		for i := range n {
			require.NoError(t, s.Append([]byte{byte(i)}))
		}

		// map all frame offsets to their corresponding sequence numbers
		// nFrames is at most 256, so this map will be manageable
		validOffsetToSeq := map[int64]uint64{}
		sc := NewScanner(s, frame.NewPool(64, 1<<20))
		for seq := s.StartSequence(); seq <= s.NextSequence(); seq++ {
			require.NoError(t, sc.Seek(seq))
			validOffsetToSeq[sc.ReadOffset()] = seq
		}

		_, eof, err := s.Find(s.NextSequence())
		require.NoError(t, err)

		// normalize test input offset to the current file envelope
		offset := int64(inputOffset)
		if eof > 0 {
			offset %= (eof + int64(frame.HeaderSize))
		}

		// reload must either recover expected start sequence or fail safely
		err = s.SetReadOffset(offset)
		require.NoError(t, s.Close())

		loaded, loadErr := Load(path, pool, nil, false)
		if err == nil {
			require.NoError(t, loadErr)
			t.Cleanup(func() { require.NoError(t, loaded.Close()) })

			// make sure the loaded segment's start sequence matches
			// the previously observed sequence for the given offset
			wantSeq, ok := validOffsetToSeq[offset]
			require.True(t, ok)
			require.Equal(t, wantSeq, loaded.StartSequence())
			return
		}

		require.True(t,
			errors.Is(err, ErrInvalidSegmentHeader) ||
				errors.Is(err, frame.ErrInvalidFrameIndex) ||
				errors.Is(err, frame.ErrInvalidFrameHeader) ||
				errors.Is(err, frame.ErrInvalidChecksum),
		)
		if loadErr == nil {
			require.NoError(t, loaded.Close())
		}
	})
}
