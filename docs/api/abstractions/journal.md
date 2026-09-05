# Journal

<!-- EXAMPLE:ExampleJournal:start -->
`Journal` is the low-policy API: sequential append, explicit `Sync`,
non-destructive `Range`, and explicit prefix/suffix truncation.
Use it when callers control retention and durability cadence explicitly.

Go reference: [Journal](https://pkg.go.dev/github.com/johnknl/alog/pkg/journal#Journal).

The following example shows append, truncation, range iteration, and sync for Journal.

```go
dir, err := os.MkdirTemp("", "alog-example-journal-")
if err != nil {
	fmt.Printf("mkdir temp: %v\n", err)
	return
}
defer os.RemoveAll(dir) //nolint:errcheck

j, err := journal.New(dir, 1024, 0)
if err != nil {
	fmt.Printf("new journal: %v\n", err)
	return
}

lastSeq, err := j.Append([]byte("cmd-1"), []byte("cmd-2"))
if err != nil {
	fmt.Printf("append: %v\n", err)
	return
}

_, err = j.Append([]byte("cmd-3"), []byte("cmd-4"))
if err != nil {
	fmt.Printf("append: %v\n", err)
	return
}

if err = j.TruncateBefore(1); err != nil {
	fmt.Printf("truncate before: %v\n", err)
	return
}

if err = j.TruncateAfter(2); err != nil {
	fmt.Printf("truncate after: %v\n", err)
	return
}

if err = j.Sync(); err != nil {
	fmt.Printf("sync: %v\n", err)
	return
}

fmt.Println(lastSeq)
err = j.Range(0, 0, func(seq uint64, payload []byte) error {
	fmt.Printf("%d:%s\n", seq, payload)
	return nil
})
if err != nil {
	fmt.Printf("range: %v\n", err)
	return
}

if err = j.Close(); err != nil {
	fmt.Printf("close: %v\n", err)
	return
}

// Output:
// 1
// 1:cmd-2
// 2:cmd-3
```
<!-- EXAMPLE:ExampleJournal:end -->
