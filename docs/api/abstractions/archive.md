# Archive

<!-- EXAMPLE:ExampleArchive:start -->
`Archive` is the high-level API for concurrent appends plus non-destructive
range reads.
Use it when many producers write at once and you want buffered concurrent
appends.
`Archive` combines write buffering with retention controls suitable for
long-running event streams.

Go reference: [Archive](https://pkg.go.dev/github.com/johnknl/alog/pkg/archive#Archive).

The following example shows buffered appends and range reads through Archive.

```go
dir, err := os.MkdirTemp("", "alog-example-archive-")
if err != nil {
	fmt.Printf("mkdir temp: %v\n", err)
	return
}
defer os.RemoveAll(dir) //nolint:errcheck

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

a, err := archive.New(
	ctx,
	dir,
	2,
	1024,
	log.WriteBufferOptions{MaxLength: 16, MaxSize: 4 << 10, MaxDelay: 5 * time.Millisecond},
)
if err != nil {
	fmt.Printf("new archive: %v\n", err)
	return
}

seq0, err := a.Append(ctx, []byte("one"))
if err != nil {
	fmt.Printf("append one: %v\n", err)
	return
}
seq1, err := a.Append(ctx, []byte("two"))
if err != nil {
	fmt.Printf("append two: %v\n", err)
	return
}

fmt.Println(seq0, seq1)

err = a.Range(0, 0, func(seq uint64, payload []byte) error {
	fmt.Printf("%d:%s\n", seq, payload)
	return nil
})
if err != nil {
	fmt.Printf("range: %v\n", err)
	return
}

if err = a.Close(); err != nil {
	fmt.Printf("close: %v\n", err)
}

// Output:
// 0 1
// 0:one
// 1:two
```
<!-- EXAMPLE:ExampleArchive:end -->
