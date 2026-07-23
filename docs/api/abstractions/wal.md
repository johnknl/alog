# WAL

`WAL` is the durable append + destructive consume abstraction.

Use it when records should be processed in order and removed from retained history after successful processing.

## Usage Example

<!-- EXAMPLE:ExampleWAL_Consume:start -->
```go
func ExampleWAL_Consume() {
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
}
```
<!-- EXAMPLE:ExampleWAL_Consume:end -->

