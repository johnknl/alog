# WAL

<!-- EXAMPLE:ExampleWAL_Consume:start -->
## Consume

`WAL` is the durable append plus destructive consume abstraction.
Use it when records should be processed in order and removed after
successful processing.
WAL provides write-ahead semantics over the base log abstraction and exposes
a linear consume API for replay flows.

Go reference: [WAL.Consume](https://pkg.go.dev/github.com/johnknl/alog/pkg/wal#WAL.Consume).

The following example shows appending records and consuming them in-order.

```go
dir, err := os.MkdirTemp("", "alog-example-wal-")
if err != nil {
	fmt.Printf("mkdir temp: %v\n", err)
	return
}
defer os.RemoveAll(dir) //nolint:errcheck

w, err := wal.New(dir, 1024)
if err != nil {
	fmt.Printf("new wal: %v\n", err)
	return
}

if _, err = w.Append([]byte("one"), []byte("two")); err != nil {
	fmt.Printf("wal append: %v\n", err)
	return
}

err = w.Consume(0, func(seq uint64, payload []byte) error {
	fmt.Printf("%d:%s\n", seq, payload)
	return nil
})
if err != nil {
	fmt.Printf("wal consume: %v\n", err)
	return
}

if err = w.Close(); err != nil {
	fmt.Printf("wal close: %v\n", err)
}

// Output:
// 0:one
// 1:two
```
<!-- EXAMPLE:ExampleWAL_Consume:end -->
