package main

func main() {}

//go:wasmimport quill archive_extract
func hostExtract(reqPtr uint32, reqLen uint32) uint64

//go:wasmexport malloc
func malloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

//go:wasmexport extract
func extract(reqPtr uint32, reqLen uint32) uint64 {
	return hostExtract(reqPtr, reqLen)
}
