package main

func main() {}

//go:wasmimport quill http_download
func hostDownload(reqPtr uint32, reqLen uint32) uint64

//go:wasmexport malloc
func malloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

//go:wasmexport dl
func dl(reqPtr uint32, reqLen uint32) uint64 {
	return hostDownload(reqPtr, reqLen)
}
