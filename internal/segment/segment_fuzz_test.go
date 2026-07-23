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
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/johnknl/alog/internal/frame"
	testutil "github.com/johnknl/alog/internal/testutil"
	"github.com/stretchr/testify/require"
)

// FuzzSegment_LoadFromBytes loads arbitrary segment bytes. Most random inputs
// should fail due to header/frame validation; on rare successful loads, every
// scanned frame must still validate and sequence numbers must remain contiguous.
func FuzzSegment_LoadFromBytes(f *testing.F) {
	f.Add([]byte{})
	seedHeader := NewHeader(0)
	f.Add(seedHeader[:])

	f.Fuzz(func(t *testing.T, raw []byte) {
		// bound arbitrary segment bytes to keep fuzz cases memory-safe
		raw = testutil.BoundedFuzzBytes(raw, 1<<20, uint32(len(raw)))

		path := filepath.Join(t.TempDir(), "fuzz-segment.bin")
		require.NoError(t, os.WriteFile(path, raw, 0o600))

		// loading most likely fails
		s, err := Load(path, frame.NewPool(64, 1<<20), nil, false)
		if err != nil {
			require.True(t,
				errors.Is(err, io.EOF) ||
					errors.Is(err, io.ErrUnexpectedEOF) ||
					errors.Is(err, ErrInvalidSegmentHeader) ||
					errors.Is(err, frame.ErrInvalidChecksum) ||
					errors.Is(err, frame.ErrInvalidFrameHeader) ||
					errors.Is(err, frame.ErrInvalidFrameIndex),
			)
			if s != nil {
				require.NoError(t, s.Close())
			}
			return
		}

		t.Cleanup(func() { require.NoError(t, s.Close()) })

		// this is extremely unlikely, but if load succeeds then internal
		// validation must also accept the readable frame window.
		require.NoError(t, s.BuildIndex())
	})
}

// FuzzSegment_TailTruncationRecovery truncates persisted tails and verifies
// reload behavior remains safe and bounded.
func FuzzSegment_TailTruncationRecovery(f *testing.F) {
	f.Add(uint8(8), uint16(0))
	f.Add(uint8(8), uint16(7))

	f.Fuzz(func(t *testing.T, nFrames uint8, cut uint16) {
		// create a bounded segment and then truncate to an arbitrary tail offset
		n := int(nFrames) + 1
		path := filepath.Join(t.TempDir(), "tail.bin")
		pool := frame.NewPool(64, 1<<20)

		s, err := Create(path, 0, pool, nil, false)
		require.NoError(t, err)
		for i := range n {
			require.NoError(t, s.Append([]byte{byte(i)}))
		}
		require.NoError(t, s.Close())

		st, err := os.Stat(path)
		require.NoError(t, err)
		size := st.Size()

		// normalize cut position to current file size before truncating
		trunc := int64(cut)
		if size > 0 {
			trunc %= size
		}

		require.NoError(t, os.Truncate(path, trunc))

		// reload must either fail safely or recover a coherent sequence window
		loaded, loadErr := Load(path, pool, nil, false)
		if loadErr != nil {
			require.True(t,
				errors.Is(loadErr, io.EOF) ||
					errors.Is(loadErr, io.ErrUnexpectedEOF) ||
					errors.Is(loadErr, ErrInvalidSegmentHeader) ||
					errors.Is(loadErr, frame.ErrInvalidChecksum) ||
					errors.Is(loadErr, frame.ErrInvalidFrameHeader) ||
					errors.Is(loadErr, frame.ErrInvalidFrameIndex),
			)
			if loaded != nil {
				require.NoError(t, loaded.Close())
			}
			return
		}
		t.Cleanup(func() { require.NoError(t, loaded.Close()) })

		// if the segment loads successfully, the next sequence must be
		// greater than or equal to the start sequence, even if the tail was truncated
		require.GreaterOrEqual(t, loaded.NextSequence(), loaded.StartSequence())
		require.GreaterOrEqual(t, loaded.StartSequence(), loaded.BaseSequence())

		// successful reloads must pass the same internal frame validation used
		// during load and maintain a bounded number of frames.
		require.NoError(t, loaded.BuildIndex())
		require.LessOrEqual(t, int(loaded.NextSequence()-loaded.StartSequence()), n)
	})
}
