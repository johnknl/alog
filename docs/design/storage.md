# Storage 

This document describes the on-disk layout, disk-usage guarantees and durability 
behavior of `alog`.

## Core Terms

- frame: a 16-byte header followed by payload bytes (payload max: `math.MaxUint32`)
- segment: one `.bin` file with a 64-byte segment header followed by zero or more frames
- sequence number (`uint64`): global record id
- index (`uint32`): per-segment frame index
- read offset (`uint64`): persisted logical head within a segment (for head truncation)

Sequence mapping:

```text
sequence = segment_start_sequence + frame_index
```

## Bounded Size

`alog` retention is controlled by `StorageOptions`:

- `MaxDiskSize`: maximum retained physical size of segment files
- `MaxSegmentSize`: per-segment rotation threshold
- `MaxSegments`: optional independent segment-count guard

The retained-byte invariant is:

```text
sum(segment file sizes) <= MaxDiskSize
```

when `MaxDiskSize > 0`.

`MaxDiskSize <= 0` means retained disk usage is unbounded. `MaxSegmentSize <= 0`
means a segment does not rotate based on size.

When a bounded `MaxDiskSize` would be exceeded, the chain reaps complete oldest
segments before appending, without requiring `Stat` on the append hot path.

If an incoming append batch cannot fit in an empty bounded segment
(`HeaderSize + appendBytes > MaxSegmentSize`), append is rejected without
reaping retained history.

`MaxSegments` remains an optional cap to limit segment count and open files;
it is not the primary retained-byte budget model.

## Frame Layout

Each frame is encoded big-endian as:

```text
offset  size  field
0       4     payload length
4       4     index
8       4     reserved
12      4     CRC32C
16      ...   payload bytes
```

CRC details:

- CRC algorithm: CRC32C (Castagnoli)
- CRC input: first 12 bytes of frame header + payload
- On read/scan, CRC mismatch returns `ErrInvalidChecksum`

## Segment Layout

Segment files are named by start sequence (zero-padded 20 digits):

```text
00000000000000000000.bin
00000000000000128473.bin
...
```

Ordering is derived from the segment header start-sequence, not file name.

### Segment Header (64 bytes)

```text
offset  size  field
0       4     magic ("ALOG")
4       4     version
8       8     start sequence
16      16    metadata slot A
32      16    metadata slot B
48      16    reserved
```

Metadata slot format (16 bytes):

```text
offset  size  field
0       8     read offset
8       2     generation
10      2     reserved
12      4     CRC32C (slot checksum)
```

Two metadata slots are used for robust read-offset persistence:

- On load, both slots are validated by checksum
- If both are valid, the newer generation wins (wrap-safe comparison)
- On update, the inactive slot is written with generation+1

## Durability Semantics

### Segment creation

Creating a new segment (`segment.Create`) performs:

1. write segment header
2. `fsync` segment file
3. `fsync` parent directory

This ensures both file data and directory entry durability.

### Appends

`segment.Append(syncWrite, payloads...)` writes frame headers+payloads as one batch
(`net.Buffers` / writev-style path).

- If `syncWrite == true`, append calls `fsync` before returning
- If append/sync fails, segment is trimmed back to the last known valid offset

High-level API behavior:

- direct `Log.Append(...)`: does **not** force sync
- `Log.Sync()`: explicit durability barrier for direct writes
- `Writer.Append(...)`: batched concurrent appends, batch is synced before ack
- `WAL.Append(...)`: appends then syncs immediately (durable per call)

### Read-offset persistence

`SetReadOffset` writes the next readable byte offset to segment header metadata.
This is used by `TruncateBefore` to persist logical head advancement.

### Truncation durability

`segment.Trim()` truncates file to tracked offset and then syncs the file.
