# Scanner

<!-- EXAMPLE:ExampleScanner_Seek_andStopAt:start -->
## Seek

The `Scanner` sequentially accesses frames by sequence number.
`Seek` positions the start sequence and `StopAt` applies an exclusive upper bound,
giving a half-open scan window [start, end).

Go reference: [Scanner.Seek](https://pkg.go.dev/github.com/johnknl/alog/pkg/log#Scanner.Seek).

The following example shows scanning a range of records with Seek and StopAt.

```go
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
```
<!-- EXAMPLE:ExampleScanner_Seek_andStopAt:end -->

<!-- EXAMPLE:ExampleNewLimitedScanner:start -->
## NewLimitedScanner

`NewLimitedScanner` applies a payload read limit while preserving sequence and
frame-boundary traversal.
This is useful for previews, metadata passes, and other workflows that need
sequence ordering without materializing full payloads.

Go reference: [NewLimitedScanner](https://pkg.go.dev/github.com/johnknl/alog/pkg/log#NewLimitedScanner).

The following example shows scanning with payload truncation per record.

```go
dir, err := os.MkdirTemp("", "alog-example-limited-scan-")
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

if _, err = l.Append([]byte("alphabet"), []byte("xy")); err != nil {
	fmt.Printf("append: %v\n", err)
	return
}

s := log.NewLimitedScanner(l, 3)
for s.Next() {
	seq, frame := s.Borrow()
	fmt.Printf("%d:%s\n", seq, frame.Payload)
	frame.Return()
}
if err = s.Err(); err != nil {
	fmt.Printf("scan: %v\n", err)
}

// Output:
// 0:alp
// 1:xy
```
<!-- EXAMPLE:ExampleNewLimitedScanner:end -->
