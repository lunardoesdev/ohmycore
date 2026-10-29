//go:build cgo

package export

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"unsafe"
)

type resp struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Error   string `json:"error"`
}

//export CGoInvoke
func CGoInvoke(requestJSON *C.char) *C.char {
	req := C.GoString(requestJSON)
	var r struct {
		APIVersion int             `json:"apiVersion"`
		Method     string          `json:"method"`
		Payload    json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(req), &r); err != nil {
		b, _ := json.Marshal(resp{Success: false, Error: err.Error()})
		return C.CString(string(b))
	}
	if r.APIVersion != 3 {
		b, _ := json.Marshal(resp{Success: false, Error: "unsupported apiVersion"})
		return C.CString(string(b))
	}
	b, _ := json.Marshal(resp{Success: true, Data: map[string]string{"method": r.Method}})
	return C.CString(string(b))
}

//export CGoFree
func CGoFree(p *C.char) {
	C.free(unsafe.Pointer(p))
}

func Invoke(requestJSON string) string {
	p := CGoInvoke(C.CString(requestJSON))
	s := C.GoString(p)
	CGoFree(p)
	return s
}
