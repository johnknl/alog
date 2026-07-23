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

package archive

import (
	"context"
	"testing"
	"time"

	"github.com/johnknl/alog/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestArchive_ConcurrentAppendAndRange(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	archive, err := New(
		ctx,
		t.TempDir(),
		4,
		8*1024,
		log.WriteBufferOptions{MaxLength: 32, MaxSize: 32 * 1024, MaxDelay: time.Millisecond},
	)
	require.NoError(t, err)

	const producers = 4
	const perProducer = 50
	errCh := make(chan error, producers)

	for p := range producers {
		producer := p
		go func() {
			for i := range perProducer {
				payload := []byte{byte('A' + producer), byte('0' + (i % 10))}
				if _, appendErr := archive.Append(ctx, payload); appendErr != nil {
					errCh <- appendErr
					return
				}
			}
			errCh <- nil
		}()
	}

	for range producers {
		require.NoError(t, <-errCh)
	}

	count := 0
	err = archive.Range(0, 0, func(seq uint64, payload []byte) error {
		require.Len(t, payload, 2)
		require.GreaterOrEqual(t, seq, uint64(0))
		count++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, producers*perProducer, count)
}
