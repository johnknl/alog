# API

alog's `Log` API is the main interface to the append-only log. [Abstractions](./abstractions/index.md) are
thin wrappers around it.

See also:

- [Configuration](./config.md)
- [Scanner](./scanner.md)
- [Writer](./writer.md)


<!-- EXAMPLE:ExampleLog_Append_andSync:start -->
## Log

### Append

It is useful when your caller defines commit boundaries explicitly and wants
to force persistence before acknowledging work.

Go reference: [Log.Append](https://pkg.go.dev/github.com/johnknl/alog/pkg/log#Log.Append).

The following example shows appending a batch and syncing it to durable storage.

```go
dir, err := os.MkdirTemp("", "alog-example-append-")
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

lastSeq, err := l.Append([]byte("alpha"), []byte("beta"), []byte("gamma"))
if err != nil {
	fmt.Printf("append: %v\n", err)
	return
}

if err = l.Sync(); err != nil {
	fmt.Printf("sync: %v\n", err)
	return
}

fmt.Println(lastSeq)

// Output:
// 2
```
<!-- EXAMPLE:ExampleLog_Append_andSync:end -->

