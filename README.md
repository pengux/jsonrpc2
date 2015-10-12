# JSONRPC2 implementation in Go
Based on the net/rpc and net/rpc/json package, it's a incomplete implementation of [JSON-RPC 2.0](http://www.jsonrpc.org/specification).

## Service methods with context
One of the differences witht he stdlib is that each method of a registered service must have an io.ReadWriteCloser as first argument. This enable the methods to have access to contexts. Here's an example:

```go

```
