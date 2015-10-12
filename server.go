// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonrpc2

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
)

const Version = "2.0"

type serverCodec struct {
	dec *json.Decoder // for reading JSON values
	enc *json.Encoder // for writing JSON values
	c   io.ReadWriteCloser

	// temporary work space
	req serverRequest

	// JSON-RPC clients can use arbitrary json values as request IDs.
	// Package rpc expects uint64 request IDs.
	// We assign uint64 sequence numbers to incoming requests
	// but save the original request ID in the pending map.
	// When rpc responds, we use the sequence number in
	// the response to find the original request ID.
	mutex   sync.Mutex // protects seq, pending
	seq     uint64
	pending map[uint64]*json.RawMessage
}

// NewServerCodec returns a new ServerCodec using JSON-RPC on conn.
func NewServerCodec(conn io.ReadWriteCloser) ServerCodec {
	return &serverCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]*json.RawMessage),
	}
}

type serverRequest struct {
	Version string           `json:"jsonrpc"`
	Method  string           `json:"method"`
	Params  *json.RawMessage `json:"params,omitempty"`
	Id      *json.RawMessage `json:"id,omitempty"`
}

func (r *serverRequest) reset() {
	r.Version = ""
	r.Method = ""
	r.Params = nil
	r.Id = nil
}

type serverResponse struct {
	Version string           `json:"jsonrpc"`
	Id      *json.RawMessage `json:"id"`
	Result  interface{}      `json:"result,omitempty"`
	Error   interface{}      `json:"error,omitempty"`
}

func (c *serverCodec) ReadRequestHeader(r *Request) error {
	c.req.reset()
	if err := c.dec.Decode(&c.req); err != nil {
		return err
	}
	r.ServiceMethod = c.req.Method

	dot := strings.LastIndex(r.ServiceMethod, ".")
	if dot < 0 {
		return &Error{
			Code: ErrCodeInvalidReq,
			Msg:  ErrMsgInvalidReq,
			Data: "rpc: service/method request ill-formed: " + r.ServiceMethod,
		}
	}

	// JSON request id can be any JSON value;
	// RPC package expects uint64.  Translate to
	// internal uint64 and save JSON on the side.
	c.mutex.Lock()
	c.seq++
	c.pending[c.seq] = c.req.Id
	c.req.Id = nil
	r.Seq = c.seq
	c.mutex.Unlock()

	return nil
}

func (c *serverCodec) ReadRequestBody(x interface{}) error {
	if x == nil {
		return nil
	}
	if c.req.Params == nil {
		return &Error{
			Code: ErrCodeInvalidParams,
			Msg:  ErrMsgInvalidParams,
		}
	}

	err := json.Unmarshal(*c.req.Params, &x)
	if err != nil {
		return &Error{
			Code: ErrCodeParse,
			Msg:  ErrMsgParse,
		}
	}

	return nil
}

var null = json.RawMessage([]byte("null"))

func (c *serverCodec) WriteResponse(r *Response, x interface{}) error {
	c.mutex.Lock()
	b, ok := c.pending[r.Seq]
	if !ok {
		c.mutex.Unlock()

		// If there is an error, write it to response
		if r.Error != nil {
			resp := serverResponse{Version: Version, Id: &null}
			resp.Error = r.Error
			return c.enc.Encode(resp)
		}

		return errors.New("invalid sequence number in response")
	}
	delete(c.pending, r.Seq)
	c.mutex.Unlock()

	if b == nil {
		// Request has no id which means it is a notification
		// (http://www.jsonrpc.org/specification#notification)
		// Return empty response
		c.c.Write([]byte("\n"))
		return nil
	}
	resp := serverResponse{Version: Version, Id: b}
	if r.Error == nil {
		resp.Result = x
	} else {
		resp.Error = r.Error
	}
	return c.enc.Encode(resp)
}

func (c *serverCodec) Close() error {
	return c.c.Close()
}

func (c *serverCodec) ReadWriteCloser() io.ReadWriteCloser {
	return c.c
}

// ServeConn runs the JSON-RPC server on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
func ServeConn(conn io.ReadWriteCloser) {
	ServeCodec(NewServerCodec(conn))
}
