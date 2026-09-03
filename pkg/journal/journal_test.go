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
//

package journal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJournal_RestartAndTruncateWorkflow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	j1, err := New(dir, 512, 0)
	require.NoError(t, err)

	_, err = j1.Append([]byte("cmd-0"), []byte("cmd-1"), []byte("cmd-2"), []byte("cmd-3"))
	require.NoError(t, err)
	require.NoError(t, j1.Sync())

	seen := make([]string, 0, 4)
	err = j1.Range(0, 0, func(_ uint64, payload []byte) error {
		seen = append(seen, string(payload))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"cmd-0", "cmd-1", "cmd-2", "cmd-3"}, seen)

	require.NoError(t, j1.TruncateBefore(2))
	require.NoError(t, j1.TruncateAfter(2))
	require.NoError(t, j1.Sync())
	require.NoError(t, j1.log.Close())

	j2, err := New(dir, 512, 0)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, j2.log.Close()) })

	seen = seen[:0]
	err = j2.Range(0, 0, func(_ uint64, payload []byte) error {
		seen = append(seen, string(payload))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"cmd-2"}, seen)
}
