# Writer

Use `StartWriter` for concurrent append workloads with grouped buffering.

<!-- EXAMPLE:Example_concurrentWriter:start -->
```go
func Example_concurrentWriter() {
	dir, err := os.MkdirTemp("", "alog-example-writer-")
	if err != nil {
		fmt.Printf("mkdir temp: %v\n", err)
		return
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	l, err := log.Load(dir, log.DefaultOptions())
	if err != nil {
		fmt.Printf("load log: %v\n", err)
		return
	}
	defer l.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := write.StartWriter(ctx, l, log.WriteBufferOptions{
		MaxLength: 16,
		MaxSize:   4 << 10,
		MaxDelay:  5 * time.Millisecond,
	})

	seq0, err := writer.Append(ctx, []byte("one"))
	if err != nil {
		fmt.Printf("append one: %v\n", err)
		return
	}

	seq1, err := writer.Append(ctx, []byte("two"))
	if err != nil {
		fmt.Printf("append two: %v\n", err)
		return
	}

	fmt.Println(seq0, seq1)

	// Output:
	// 0 1
}
```
<!-- EXAMPLE:Example_concurrentWriter:end -->

