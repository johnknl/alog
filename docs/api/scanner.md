# Scanner

The `Scanner` can be used to sequentially access frames in the log by sequence number.

Use `Seek(startSeq)` and `StopAt(endSeq)` for half-open ranges `[startSeq, endSeq)`.
`StopAt(0)` means scan until EOF.

Different instances of `Scanner` share the same frame pool. When you use `Scanner`,
the caller is responsible for returning frames to the pool. `Scanner` is used by `Log.Range()`
which automatically returns frames at the end of each callback invocation.

Internally `Scanner` uses `segment.Scanner` to scan within a single segment. The latter
maintains an index for fast seek times.

<!-- EXAMPLE:ExampleScanner_Seek_andStopAt:start -->
```go
func ExampleScanner_Seek_andStopAt() {
	dir, err := os.MkdirTemp("", "alog-example-scan-")
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

	for _, v := range []string{"a", "b", "c", "d", "e"} {
		if _, err = l.Append([]byte(v)); err != nil {
			fmt.Printf("append: %v\n", err)
			return
		}
	}

	s := log.NewScanner(l)
	s.Seek(2)
	s.StopAt(4)

	for s.Next() {
		seq, frame := s.Borrow()
		fmt.Printf("%d:%s\n", seq, frame.Payload)
		frame.Return()
	}
	if err = s.Err(); err != nil {
		fmt.Printf("scan: %v\n", err)
	}

	// Output:
	// 2:c
	// 3:d
}
```
<!-- EXAMPLE:ExampleScanner_Seek_andStopAt:end -->

