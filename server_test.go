// Copyright 2010 The Go Authors.  All rights reserved.
// Copyright 2015 Peter Nguyen. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonrpc2

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
)

type ArithArgs struct {
	A, B int
}

type ArithSubArgs struct {
	A int `json:"minuend"`
	B int `json:"subtrahend"`
}

type Reply struct {
	C int
}

type Arith int

type ArithAddResp struct {
	Id     interface{} `json:"id"`
	Result Reply       `json:"result"`
	Error  interface{} `json:"error"`
}

func (t *Arith) Add(rwc io.ReadWriteCloser, args *ArithArgs, reply *Reply) error {
	reply.C = args.A + args.B
	return nil
}

func (t *Arith) Sub(rwc io.ReadWriteCloser, args []int, reply *int) error {
	*reply = args[0] - args[1]
	return nil
}

func (t *Arith) SubWithStruct(rwc io.ReadWriteCloser, args *ArithSubArgs, reply *int) error {
	*reply = args.A - args.B
	return nil
}

func (t *Arith) WithCustomError(rwc io.ReadWriteCloser, args []int, reply *int) error {
	return &Error{
		Code: -32003,
		Msg:  "Custom error",
	}
}

func (t *Arith) Error(rwc io.ReadWriteCloser, args *ArithArgs, reply *Reply) error {
	panic("ERROR")
}

type Notification int

func (n *Notification) Update(rwc io.ReadWriteCloser, args []int, reply *Reply) error {
	return nil
}

func (n *Notification) FooBar(rwc io.ReadWriteCloser, args []int, reply *Reply) error {
	return nil
}

func init() {
	Register(new(Arith))
	Register(new(Notification))
}

func TestServer(t *testing.T) {

	// Send hand-coded requests to server, parse responses.
	var tests = []struct {
		req, resp string
		reopen    bool // Whether to reopen the connection
	}{
		// rpc call with positional parameters:
		{
			`{"jsonrpc": "2.0", "method": "Arith.Sub", "params": [42, 23], "id": 1}`,
			`{"jsonrpc":"2.0","id":1,"result":19}`,
			false,
		},
		{
			`{"jsonrpc": "2.0", "method": "Arith.Sub", "params": [23, 42], "id": 2}`,
			`{"jsonrpc":"2.0","id":2,"result":-19}`,
			false,
		},

		// rpc call with named parameters:
		{
			`{"jsonrpc": "2.0", "method": "Arith.SubWithStruct", "params": {"subtrahend": 23, "minuend": 42}, "id": 3}`,
			`{"jsonrpc":"2.0","id":3,"result":19}`,
			false,
		},
		{
			`{"jsonrpc": "2.0", "method": "Arith.SubWithStruct", "params": {"minuend": 42, "subtrahend": 23}, "id": 4}`,
			`{"jsonrpc":"2.0","id":4,"result":19}`,
			false,
		},

		// rpc call with custom error in response
		{
			`{"jsonrpc": "2.0", "method": "Arith.WithCustomError", "params": [2, 2], "id": 1}`,
			`{"jsonrpc":"2.0","id":1,"error":{"code":-32003,"message":"Custom error"}}`,
			false,
		},
		{
			`{"jsonrpc": "2.0", "method": "Arith.SubWithStruct", "params": {"minuend": 42, "subtrahend": 23}, "id": 4}`,
			`{"jsonrpc":"2.0","id":4,"result":19}`,
			false,
		},

		// Notifications
		{
			`{"jsonrpc": "2.0", "method": "Notification.Update", "params": [1,2,3,4,5]}`,
			``,
			false,
		},
		{
			`{"jsonrpc": "2.0", "method": "Notification.Foobar"}`,
			``,
			false,
		},

		// rpc call of non-existent method:
		{
			`{"jsonrpc": "2.0", "method": "Arith.Foobar", "id": "1"}`,
			`{"jsonrpc":"2.0","id":"1","error":{"code":-32601,"message":"Method not found"}}`,
			false,
		},

		// rpc call with invalid JSON:
		{
			`{"jsonrpc": "2.0", "method": "foobar, "params": "bar", "baz]`,
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error","data":"server cannot decode request: invalid character 'p' after object key:value pair"}}`,
			false,
		},

		// rpc call with invalid Request object:
		{
			`{"jsonrpc": "2.0", "method": 1, "params": "bar"}`,
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"Invalid Request"}}`,
			true,
		},

		// rpc call Batch, invalid JSON:
		{
			`[
				{"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"},
				{"jsonrpc": "2.0", "method"
			]`,
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error","data":"server cannot decode request: invalid character ']' after object key"}}`,
			true,
		},

		// rpc call with an empty Array:
		{
			`[]`,
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"Invalid Request"}}`,
			true,
		},

		// // rpc call with an invalid Batch (but not empty):
		// {
		// 	`[1]`,
		// 	`[
		// 		{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"Invalid Request"}}
		// 	]`,
		// },
		//
		// // rpc call with invalid Batch:
		// {
		// 	`[1,2,3]`,
		// 	`[
		// 		{"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null},
		// 		{"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null},
		// 		{"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null}
		// 	]`,
		// },
		//
		// // rpc call Batch:
		// {
		// 	`[
		// 		{"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"},
		// 		{"jsonrpc": "2.0", "method": "notify_hello", "params": [7]},
		// 		{"jsonrpc": "2.0", "method": "subtract", "params": [42,23], "id": "2"},
		// 		{"foo": "boo"},
		// 		{"jsonrpc": "2.0", "method": "foo.get", "params": {"name": "myself"}, "id": "5"},
		// 		{"jsonrpc": "2.0", "method": "get_data", "id": "9"}
		// 	]`,
		// 	`[
		// 		{"jsonrpc": "2.0", "result": 7, "id": "1"},
		// 		{"jsonrpc": "2.0", "result": 19, "id": "2"},
		// 		{"jsonrpc": "2.0", "error": {"code": -32600, "message": "Invalid Request"}, "id": null},
		// 		{"jsonrpc": "2.0", "error": {"code": -32601, "message": "Method not found"}, "id": "5"},
		// 		{"jsonrpc": "2.0", "result": ["hello", 5], "id": "9"}
		// 	]`,
		// },
		//
		// // rpc call Batch (all notifications):
		// {
		// 	`[
		// 		{"jsonrpc": "2.0", "method": "notify_sum", "params": [1,2,4]},
		// 		{"jsonrpc": "2.0", "method": "notify_hello", "params": [7]}
		// 	]`,
		// 	``,
		// },
	}

	var cli, srv net.Conn
	cli, srv = net.Pipe()
	defer cli.Close()
	go ServeConn(srv)

	for _, tt := range tests {
		if tt.reopen {
			cli.Close()
			cli, srv = net.Pipe()
			go ServeConn(srv)
		}

		go func() {
			_, err := cli.Write([]byte(tt.req))
			if err != nil {
				t.Error(err)
			}
		}()

		s, err := bufio.NewReader(cli).ReadString('\n')
		if err != nil && err != io.EOF {
			t.Error(err)
		}

		s = strings.TrimSpace(s)

		if s != tt.resp {
			t.Errorf("Request %s => %s, want %s", tt.req, s, tt.resp)
		}
	}
}