package main

import (
	"encoding/json"
	"unsafe"
)

func main() {}

//go:wasmimport quill http_request
func hostHTTPRequest(reqPtr uint32, reqLen uint32) uint64

//go:wasmexport malloc
func malloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

//go:wasmexport get
func get(urlPtr uint32, urlLen uint32) uint64 {
	url := readString(urlPtr, urlLen)
	req := map[string]string{"method": "GET", "url": url}
	reqJSON, _ := json.Marshal(req)
	return callHost(reqJSON)
}

//go:wasmexport post
func post(urlPtr uint32, urlLen uint32) uint64 {
	url := readString(urlPtr, urlLen)
	req := map[string]string{"method": "POST", "url": url}
	reqJSON, _ := json.Marshal(req)
	return callHost(reqJSON)
}

//go:wasmexport request
func request(reqPtr uint32, reqLen uint32) uint64 {
	return hostHTTPRequest(reqPtr, reqLen)
}

func callHost(reqJSON []byte) uint64 {
	p := malloc(uint32(len(reqJSON)))
	offset := uint32(uintptr(unsafe.Pointer(p)))
	copy(unsafe.Slice(p, uint32(len(reqJSON))), reqJSON)
	return hostHTTPRequest(offset, uint32(len(reqJSON)))
}

func readString(ptr, len uint32) string {
	p := (*byte)(unsafe.Pointer(uintptr(ptr)))
	b := unsafe.Slice(p, len)
	return string(b)
}
