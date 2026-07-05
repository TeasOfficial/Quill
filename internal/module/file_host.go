package module

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero/api"
)

type dlReq struct {
	URL     string `json:"url"`
	Path    string `json:"path"`
	Threads int    `json:"threads,omitempty"`
}

type extractReq struct {
	File string `json:"file"`
	Dest string `json:"dest"`
}

func hostDownload(ctx context.Context, mod api.Module, reqPtr uint32, reqLen uint32) uint64 {
	mem := mod.Memory()
	reqBytes, ok := mem.Read(reqPtr, reqLen)
	if !ok {
		return packFail(mod, "failed to read request")
	}

	var req dlReq
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return packFail(mod, "invalid request: "+err.Error())
	}
	if req.URL == "" || req.Path == "" {
		return packFail(mod, "url and path are required")
	}
	if req.Threads <= 0 {
		req.Threads = 4
	}

	os.MkdirAll(filepath.Dir(req.Path), 0755)

	size, err := downloadFile(req.URL, req.Path, req.Threads)
	if err != nil {
		return packFail(mod, err.Error())
	}

	return packOK(mod, map[string]interface{}{
		"path": req.Path,
		"size": float64(size),
	})
}

func hostExtract(ctx context.Context, mod api.Module, reqPtr uint32, reqLen uint32) uint64 {
	mem := mod.Memory()
	reqBytes, ok := mem.Read(reqPtr, reqLen)
	if !ok {
		return packFail(mod, "failed to read request")
	}

	var req extractReq
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return packFail(mod, "invalid request: "+err.Error())
	}
	if req.File == "" || req.Dest == "" {
		return packFail(mod, "file and dest are required")
	}

	os.MkdirAll(req.Dest, 0755)

	count, err := extractArchive(req.File, req.Dest)
	if err != nil {
		return packFail(mod, err.Error())
	}

	return packOK(mod, map[string]interface{}{
		"dest":  req.Dest,
		"count": float64(count),
	})
}

func downloadFile(url, path string, threads int) (int64, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	headReq, _ := http.NewRequest("HEAD", url, nil)
	headReq.Header.Set("User-Agent", "Quill/1.0")
	headResp, err := client.Do(headReq)
	if err != nil {
		return streamDownload(url, path)
	}
	headResp.Body.Close()

	contentLength := headResp.ContentLength
	acceptRanges := headResp.Header.Get("Accept-Ranges") == "bytes"

	if contentLength <= 0 || !acceptRanges || threads <= 1 || contentLength < 5*1024*1024 {
		return streamDownload(url, path)
	}

	return multiDownload(url, path, contentLength, threads)
}

func streamDownload(url, path string) (int64, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	return n, err
}

func multiDownload(url, path string, total int64, threads int) (int64, error) {
	chunkSize := total / int64(threads)
	var wg sync.WaitGroup
	errs := make([]error, threads)
	chunks := make([]string, threads)

	downloadChunk := func(i int) {
		defer wg.Done()
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == threads-1 {
			end = total - 1
		}

		tmpPath := path + fmt.Sprintf(".part%d", i)
		chunks[i] = tmpPath

		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
		req.Header.Set("User-Agent", "Quill/1.0")

		resp, err := client.Do(req)
		if err != nil {
			errs[i] = err
			return
		}
		defer resp.Body.Close()

		f, err := os.Create(tmpPath)
		if err != nil {
			errs[i] = err
			return
		}
		defer f.Close()

		_, errs[i] = io.Copy(f, resp.Body)
	}

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go downloadChunk(i)
	}
	wg.Wait()

	for _, e := range errs {
		if e != nil {
			cleanupChunks(chunks)
			return 0, fmt.Errorf("multi-download: %w", e)
		}
	}

	out, err := os.Create(path)
	if err != nil {
		cleanupChunks(chunks)
		return 0, fmt.Errorf("assemble: %w", err)
	}
	defer out.Close()

	var written int64
	for _, chunk := range chunks {
		f, err := os.Open(chunk)
		if err != nil {
			cleanupChunks(chunks)
			return 0, fmt.Errorf("read chunk: %w", err)
		}
		n, _ := io.Copy(out, f)
		written += n
		f.Close()
	}
	cleanupChunks(chunks)

	return written, nil
}

func cleanupChunks(chunks []string) {
	for _, c := range chunks {
		if c != "" {
			os.Remove(c)
		}
	}
}

func extractArchive(file, dest string) (int, error) {
	lower := strings.ToLower(file)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(file, dest)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(file, dest)
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		return extractTarBz2(file, dest)
	case strings.HasSuffix(lower, ".tar"):
		return extractTar(file, dest)
	case strings.HasSuffix(lower, ".gz"):
		return extractGz(file, dest)
	case strings.HasSuffix(lower, ".bz2"):
		return extractBz2(file, dest)
	default:
		return 0, fmt.Errorf("unsupported archive format: %s", filepath.Ext(file))
	}
}

func extractZip(file, dest string) (int, error) {
	r, err := zip.OpenReader(file)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	count := 0
	for _, f := range r.File {
		if err := writeZipEntry(f, dest); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func writeZipEntry(f *zip.File, dest string) error {
	target := filepath.Join(dest, f.Name)
	if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(filepath.Separator)) {
		return fmt.Errorf("zip entry escapes dest: %s", f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0755)
	}
	os.MkdirAll(filepath.Dir(target), 0755)
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func extractTar(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return writeTarEntries(tar.NewReader(f), dest)
}

func extractTarGz(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gzr.Close()
	return writeTarEntries(tar.NewReader(gzr), dest)
}

func extractTarBz2(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return writeTarEntries(tar.NewReader(bzip2.NewReader(f)), dest)
}

func writeTarEntries(tr *tar.Reader, dest string) (int, error) {
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		target := filepath.Join(dest, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(filepath.Separator)) {
			return count, fmt.Errorf("tar entry escapes dest: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, err := os.Create(target)
			if err != nil {
				return count, err
			}
			io.Copy(out, tr)
			out.Close()
		}
		count++
	}
	return count, nil
}

func extractGz(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gzr.Close()
	name := strings.TrimSuffix(filepath.Base(file), ".gz")
	if name == filepath.Base(file) {
		name += ".out"
	}
	out, err := os.Create(filepath.Join(dest, name))
	if err != nil {
		return 0, err
	}
	defer out.Close()
	io.Copy(out, gzr)
	return 1, nil
}

func extractBz2(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	name := strings.TrimSuffix(filepath.Base(file), ".bz2")
	if name == filepath.Base(file) {
		name += ".out"
	}
	out, err := os.Create(filepath.Join(dest, name))
	if err != nil {
		return 0, err
	}
	defer out.Close()
	_, err = io.Copy(out, bzip2.NewReader(f))
	if err != nil {
		return 0, err
	}
	return 1, nil
}
