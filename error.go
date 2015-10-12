package jsonrpc2

import "fmt"

type ErrCode int

const (
	ErrCodeParse          ErrCode = -32700
	ErrCodeInvalidReq     ErrCode = -32600
	ErrCodeMethodNotFound ErrCode = -32601
	ErrCodeInvalidParams  ErrCode = -32602
	ErrCodeInternal       ErrCode = -32603

	// Custom error codes: -32000 to -32099
	ErrCodeServer          ErrCode = -32001
	ErrCodeServiceNotFound ErrCode = -32002

	// Error messages
	ErrMsgParse          string = "Parse error"
	ErrMsgInvalidReq     string = "Invalid Request"
	ErrMsgMethodNotFound string = "Method not found"
	ErrMsgInvalidParams  string = "Invalid params"
	ErrMsgInternal       string = "Internal error"

	ErrMsgServiceNotFound string = "Service not found"
)

type Error struct {
	// A Number that indicates the error type that occurred.
	// This MUST be an integer.
	// Required
	Code ErrCode `json:"code"` /* required */

	// A String providing a short description of the error.
	// The message SHOULD be limited to a concise single sentence.
	// Required
	Msg string `json:"message"`

	// A Primitive or Structured value that contains additional information about the error.
	// The value of this member is defined by the Server (e.g. detailed error information, nested errors etc.).
	// Optional
	Data interface{} `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%v - %s: %v", e.Code, e.Msg, e.Data)
}
