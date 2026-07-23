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

package log_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/johnknl/alog/internal/testutil"
	"github.com/johnknl/alog/pkg/log"
	"github.com/stretchr/testify/require"
)

const (
	fuzzMaxOps          = 128
	truncateSegmentSize = 8 * 1024
	consumeSegmentSize  = 1 << 20
)

const (
	consumeOpAppendOne uint8 = iota
	consumeOpAppendTwo
	consumeOpConsumeAll
	consumeOpConsumeWithStop
	consumeOpAssert
	consumeOpReopen
)

const (
	truncateOpAppendOne uint8 = iota
	truncateOpAppendTwo
	truncateOpSync
	truncateOpBefore
	truncateOpAfter
	truncateOpAssert
	truncateOpReopen
)

var errStop = errors.New("stop")

type truncateFuzzModel struct {
	rows [][]byte
	head uint64
}

func (m *truncateFuzzModel) isEmpty() bool {
	return len(m.rows) == 0
}

func (m *truncateFuzzModel) trackAppend(payloads ...[]byte) {
	for _, payload := range payloads {
		m.rows = append(m.rows, append([]byte(nil), payload...))
	}
}

func (m *truncateFuzzModel) beforeOp(op byte) (seq uint64, shift uint64) {
	shift = uint64(op) % uint64(len(m.rows)+1)
	seq = m.head + shift
	return seq, shift
}

func (m *truncateFuzzModel) applyBeforeShift(shift uint64) {
	if shift >= uint64(len(m.rows)) {
		m.head += uint64(len(m.rows))
		m.rows = m.rows[:0]
		return
	}

	m.rows = m.rows[shift:]
	m.head += shift
}

func (m *truncateFuzzModel) afterOp(op byte) (seq uint64, keep uint64) {
	keep = uint64(op) % uint64(len(m.rows))
	seq = m.head + keep
	return seq, keep
}

func (m *truncateFuzzModel) applyAfterKeep(keep uint64) {
	m.rows = m.rows[:keep+1]
}

func (m *truncateFuzzModel) assertMatches(t *testing.T, l *log.Log) {
	t.Helper()
	gotSeq := make([]uint64, 0)
	gotPayload := make([][]byte, 0)
	err := l.Range(0, 0, func(seq uint64, payload []byte) error {
		gotSeq = append(gotSeq, seq)
		gotPayload = append(gotPayload, append([]byte(nil), payload...))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, len(m.rows), len(gotPayload))
	for i := range m.rows {
		require.Equal(t, m.head+uint64(i), gotSeq[i])
		require.Equal(t, m.rows[i], gotPayload[i])
	}
}

type consumeFuzzModel struct {
	rows       [][]byte
	nextUnread uint64
}

func (m *consumeFuzzModel) expectedSeqAfterAppend(n int) uint64 {
	return uint64(len(m.rows) + n - 1)
}

func (m *consumeFuzzModel) trackAppend(payloads ...[]byte) {
	for _, payload := range payloads {
		m.rows = append(m.rows, append([]byte(nil), payload...))
	}
}

func (m *consumeFuzzModel) fromSeq(op byte) uint64 {
	return uint64(op) % (uint64(len(m.rows)) + 1)
}

func (m *consumeFuzzModel) consumeStart(from uint64) uint64 {
	return testutil.MaxUint64(from, m.nextUnread)
}

func (m *consumeFuzzModel) recordConsumedAll(consumed []uint64) {
	if len(consumed) > 0 {
		m.nextUnread = consumed[len(consumed)-1] + 1
	}
}

func (m *consumeFuzzModel) recordConsumedUntilStop(processed []uint64, start uint64) {
	if len(processed) > 0 {
		m.nextUnread = processed[len(processed)-1] + 1
		return
	}

	m.nextUnread = start
}

func (m *consumeFuzzModel) assertMatches(t *testing.T, l *log.Log) {
	t.Helper()
	seqs := make([]uint64, 0)
	err := l.Consume(0, func(seq uint64, payload []byte) error {
		seqs = append(seqs, seq)
		if seq >= uint64(len(m.rows)) {
			return fmt.Errorf("seq %d beyond model length %d", seq, len(m.rows))
		}
		if len(payload) > 0 && len(m.rows[seq]) > 0 {
			require.Equal(t, m.rows[seq], payload)
		}
		return nil
	})
	require.NoError(t, err)

	if m.nextUnread >= uint64(len(m.rows)) {
		require.Len(t, seqs, 0)
		return
	}
	require.Equal(t, uint64(len(m.rows))-m.nextUnread, uint64(len(seqs)))
	for i, seq := range seqs {
		require.Equal(t, m.nextUnread+uint64(i), seq)
	}
}

// FuzzLog_ConsumeStateMachine exercises append/consume/reopen behavior on the Log API.
func FuzzLog_ConsumeStateMachine(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5})

	f.Fuzz(func(t *testing.T, ops []byte) {
		// bound operation stream size for stable runtime and shrinking
		ops = fuzzBoundOps(ops, fuzzMaxOps)

		// start from an empty on-disk log and model
		dir := t.TempDir()
		l, err := loadFuzzLog(dir, consumeSegmentSize, 1)
		require.NoError(t, err)

		model := &consumeFuzzModel{rows: make([][]byte, 0)}

		reopen := func() {
			l = reopenFuzzLog(t, l, dir, consumeSegmentSize, 1)
			model.assertMatches(t, l)
		}

		// drive append/consume/reopen transitions and keep model in sync
		for i, op := range ops {
			switch op % 6 {
			case consumeOpAppendOne:
				// append a single payload and validate assigned sequence
				payload := []byte{byte(i), op}
				expectedSeq := model.expectedSeqAfterAppend(1)
				seq := appendToLog(t, l, payload)
				model.trackAppend(payload)
				require.NoError(t, l.Sync())
				require.Equal(t, expectedSeq, seq)
			case consumeOpAppendTwo:
				// append a two-item batch and validate terminal sequence
				payloadA := []byte{byte(i), op, 0x11}
				payloadB := []byte{byte(i), op, 0x22}
				expectedSeq := model.expectedSeqAfterAppend(2)
				seq := appendToLog(t, l, payloadA, payloadB)
				model.trackAppend(payloadA, payloadB)
				require.NoError(t, l.Sync())
				require.Equal(t, expectedSeq, seq)
			case consumeOpConsumeAll:
				// consume from derived offset and advance unread cursor
				from := model.fromSeq(op)
				consumed := make([]uint64, 0)
				err = l.Consume(from, func(seq uint64, _ []byte) error {
					consumed = append(consumed, seq)
					return nil
				})
				require.NoError(t, err)
				model.recordConsumedAll(consumed)
			case consumeOpConsumeWithStop:
				// consume until callback stop error and persist cursor position
				from := model.fromSeq(op)
				start := model.consumeStart(from)

				// if nothing is readable, callback is never invoked
				if start >= uint64(len(model.rows)) {
					err = l.Consume(from, func(uint64, []byte) error {
						return errStop
					})
					require.NoError(t, err)
					continue
				}

				failAt := start + uint64(op%4)
				processed := make([]uint64, 0)
				err = l.Consume(from, func(seq uint64, _ []byte) error {
					if seq == failAt {
						return errStop
					}
					processed = append(processed, seq)
					return nil
				})
				require.Error(t, err)
				model.recordConsumedUntilStop(processed, start)
			case consumeOpAssert:
				// assert persisted consume view matches model state
				model.assertMatches(t, l)
			case consumeOpReopen:
				// reopen and verify state survives restart
				reopen()
			}
		}

		// final state must match after the full op stream
		model.assertMatches(t, l)
		require.NoError(t, l.Close())
	})
}

// FuzzLog_TruncateStateMachine exercises append/sync/truncate/reopen workflows on the Log API.
func FuzzLog_TruncateStateMachine(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})

	f.Fuzz(func(t *testing.T, ops []byte) {
		// bound operation stream size for stable runtime and shrinking
		ops = fuzzBoundOps(ops, fuzzMaxOps)

		// start from an empty on-disk log and model
		dir := t.TempDir()
		l, err := loadFuzzLog(dir, truncateSegmentSize, 0)
		require.NoError(t, err)

		model := &truncateFuzzModel{rows: make([][]byte, 0)}

		reopen := func() {
			l = reopenFuzzLog(t, l, dir, truncateSegmentSize, 0)
			model.assertMatches(t, l)
		}

		// drive the same transitions on disk and in model state
		for i, op := range ops {
			switch op % 7 {
			case truncateOpAppendOne:
				// append one payload and extend model tail
				payload := []byte{byte(i), op}
				appendToLog(t, l, payload)
				model.trackAppend(payload)
			case truncateOpAppendTwo:
				// append two payloads in one call and extend model tail
				payloadA := []byte{byte(i), op, 0xA1}
				payloadB := []byte{byte(i), op, 0xB2}
				appendToLog(t, l, payloadA, payloadB)
				model.trackAppend(payloadA, payloadB)
			case truncateOpSync:
				// force durability checkpoint between mutating operations
				require.NoError(t, l.Sync())
			case truncateOpBefore:
				// nothing to truncate from an empty model
				if model.isEmpty() {
					continue
				}

				// truncate prefix and mirror shift in model
				seq, shift := model.beforeOp(op)
				require.NoError(t, l.TruncateBefore(seq))
				model.applyBeforeShift(shift)
			case truncateOpAfter:
				// nothing to truncate from an empty model
				if model.isEmpty() {
					continue
				}

				// truncate suffix and mirror retained tail in model
				seq, keep := model.afterOp(op)
				require.NoError(t, l.TruncateAfter(seq))
				model.applyAfterKeep(keep)
			case truncateOpAssert:
				// assert range view remains aligned with model
				model.assertMatches(t, l)
			case truncateOpReopen:
				// reopen and verify truncation state survives restart
				reopen()
			}
		}

		// final state must match after the full op stream
		model.assertMatches(t, l)
		require.NoError(t, l.Close())
	})
}

func loadFuzzLog(dir string, segmentSize int64, maxSegments int) (*log.Log, error) {
	opts := log.DefaultOptions()
	opts.Storage.MaxDiskSize = 0
	opts.Storage.MaxSegmentSize = segmentSize
	opts.Storage.MaxSegments = maxSegments

	return log.Load(dir, opts)
}

func fuzzBoundOps(ops []byte, maxOps int) []byte {
	if len(ops) > maxOps {
		return ops[:maxOps]
	}

	return ops
}

func appendToLog(t *testing.T, l *log.Log, payloads ...[]byte) uint64 {
	t.Helper()

	seq, err := l.Append(payloads...)
	require.NoError(t, err)

	return seq
}

func reopenFuzzLog(t *testing.T, current *log.Log, dir string, segmentSize int64, maxSegments int) *log.Log {
	t.Helper()
	require.NoError(t, current.Close())
	l, err := loadFuzzLog(dir, segmentSize, maxSegments)
	require.NoError(t, err)

	return l
}
