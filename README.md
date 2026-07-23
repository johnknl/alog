# alog

`alog` ("a-log") is a fast, general-purpose, append-only log for Go.

## Features

- Sequence numbering
- Indexed segment seeks
- Head- and tail truncation
- Frame integrity checks
- Crash recovery
- Optional implicit write durability (i.e. `fsync`)
- Optional disk-usage guarantees and automatic segment reaping
- Optional concurrent appends with opportunistic grouping

For detailed usage and design details, check out the [user docs](https://johnknl.github.io/alog/).

## Usage

The API is described in the [API docs](https://johnknl.github.io/alog/api/).

The main `alog` API is around [Log](https://johnknl.github.io/alog/api/log/) and various helper types. 
This supports many different use cases, and it may not always be straightforward how `alog` supports 
yours. Because of this, `alog` provides some high-level abstractions:

- [WAL](https://johnknl.github.io/alog/api/wal/): sequential durable append + consuming reader
- [Archive](https://johnknl.github.io/alog/api/archive/): concurrent appends + ranged non-mutating reads
- [Journal](https://johnknl.github.io/alog/api/journal/): thin abstraction over log with explicit sync and truncation

Rendered GoDoc is available at [pkg.go.dev](https://pkg.go.dev/github.com/johnknl/alog).

## Status

`alog` should be considered _beta_. The design is mostly stable; current efforts are around optimization and 
validation / testing.

Aside from the usual tests and benchmarks, `alog` also does:

- Testing unhappy paths and edge cases using a mocked file system
- Mutation testing using [Gremlins](https://github.com/go-gremlins/gremlins)
- Fuzz testing on Log API write-path workflows and low-level storage/frame components

## License

MIT
