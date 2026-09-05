# Configuration

## Rotation and Retention

`alog`s rotation and retention is governed by the combined values of three options:

| `MaxDiskSize` | `MaxSegmentSize` | `MaxSegments` | Behavior |
| --- | --- | --- | --- |
| `> 0` | `> 0` | `0` | Byte-budget retention with size-based rotation |
| `> 0` | `> 0` | `> 0` | Byte-budget retention plus segment-count guard |
| `<= 0` | `> 0` | `0` | Size-based rotation with unbounded retained bytes |
| `<= 0` | `> 0` | `1` | Single bounded segment |
| `<= 0` | `<= 0` | `0` | Fully unbounded segment chain |

<!-- EXAMPLE:ExampleOptions_default:start -->
## Options

### Default

This is a good starting point for most deployments: segment rotation is
bounded, retained bytes are capped, and pool defaults are already set.

Go reference: [Options](https://pkg.go.dev/github.com/johnknl/alog/pkg/log#Options).

The following example shows using `DefaultOptions` as a baseline.

```go
opts := log.DefaultOptions()
```
<!-- EXAMPLE:ExampleOptions_default:end -->

<!-- EXAMPLE:ExampleOptions_singleSegmentSync:start -->
### SingleSegmentSync

This configuration keeps exactly one segment and rejects appends when full,
while `SyncOnAppend` fsyncs each write before it returns.

Go reference: [Options](https://pkg.go.dev/github.com/johnknl/alog/pkg/log#Options).

The following example shows single-segment mode with synchronous durability.

```go
opts := log.DefaultOptions()
opts.Storage.MaxSegments = 1
opts.Storage.MaxSegmentSize = 64 * 1024 * 1024
opts.Storage.SyncOnAppend = true
```
<!-- EXAMPLE:ExampleOptions_singleSegmentSync:end -->

<!-- EXAMPLE:ExampleOptions_boundedRotationUnboundedDisk:start -->
### BoundedRotationUnboundedDisk

This rotates by segment size but leaves total retained bytes unbounded,
which can be useful when external retention handles pruning.

Go reference: [Options](https://pkg.go.dev/github.com/johnknl/alog/pkg/log#Options).

The following example shows bounded segment sizes with unbounded retained bytes.

```go
opts := log.DefaultOptions()
opts.Storage.MaxDiskSize = 0
opts.Storage.MaxSegmentSize = 128 * 1024 * 1024
opts.Storage.MaxSegments = 0
```
<!-- EXAMPLE:ExampleOptions_boundedRotationUnboundedDisk:end -->

