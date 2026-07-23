# TSDB Layer

A time-series abstraction on top of alog while preserving alog's
core physical sequencing model.

The TSDB layer adds logical time ordering and admission policy without pushing
timestamp semantics down into alog core storage primitives.

## Ordering

Two orderings coexist:

```text
physical order = alog sequence
logical order  = (timestamp, sequence)
```

Physical order is authoritative for storage. Logical order is authoritative for
query semantics.

## Layering

- alog core remains timestamp-agnostic.
- TSDB concerns live in a dedicated package layer.
- TSDB metadata is owned by TSDB, not segment/frame internals.

This keeps alog reusable for non-time-series workloads and keeps TSDB
evolution nicely isolated.

## Envelope

TSDB data is embedded inside alog payload bytes.

Draft envelope:

```text
8-byte timestamp (int64, big-endian, unix nanos) + payload
```

Tie-breaking uses alog sequence; sequence is not duplicated in TSDB record
bytes.

## Admission

TSDB write acceptance is bounded by configurable disorder tolerance.

- Maintain a monotonic `maxTimestamp` watermark source.
- Define `writeWatermark = maxTimestamp - MaxDisorder`.
- Accept timestamps on/after the watermark; reject older timestamps.

This keeps ingestion tolerant to bounded disorder while keeping old-history writes
explicitly out of the normal path.

## Reads

Readers emit deterministic logical order by resequencing bounded disorder.

- Input stream is physical alog order.
- Output stream is globally ordered by `(timestamp, sequence)`.
- Emission uses a read watermark derived from observed max timestamp.

This is enforced by admission, not best-effort.

## Restart and State Reconstruction

Admission semantics must survive restart.

Design requirement:

- Reconstructed TSDB state must never reject/accept in ways that contradict
  durable stored data.

Initial strategy may reconstruct from retained data; persistent TSDB-side
metadata is an optimization layer, not a requirement for conceptual correctness.

## Concurrency

Timestamp admission and physical append ordering must share one serialization
model.

Design preference:

- Use one ordered TSDB write path that owns watermark checks and commits.

Avoid split validation/enqueue patterns that can introduce watermark races between
admission and persistence ordering.

## Durability

TSDB should present one explicit durability contract (for example, durable append
ack or explicit-sync behavior) rather than an implicit blend of multiple alog
facades.

Watermark progression must not get ahead of durability guarantees.

## Query Model

Range queries are logically time-based but physically scan/resequence/filter.

Sparse time indexing is a performance accelerator, not canonical truth.

- Index entries map time buckets to approximate physical positions.
- Queries seek conservatively before requested start time and resequence forward.

## Observability Requirements

TSDB should expose internal ordering pressure so operators can size
`MaxDisorder` and buffers.

Useful signals include watermark position, rejection counts, and resequencer
buffer behavior.
