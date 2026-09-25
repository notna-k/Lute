// Package joblog names per-job log files and reads them in pages from either end.
package joblog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// maxLinesPerRequest keeps one response under the gRPC message size limit.
	maxLinesPerRequest = 500
	maxLineBytes       = 1 << 20
	readChunkSize      = 256 * 1024
)

// FileName is the name of a job's log file inside the job logs directory.
func FileName(jobID string) string {
	return "job-" + jobID + ".log"
}

// Path is the job's log file in dir; it rejects ids that would place the file elsewhere.
func Path(dir, jobID string) (string, error) {
	name := FileName(jobID)
	if jobID == "" || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid job id %q", jobID)
	}
	return filepath.Join(dir, name), nil
}

// Result is the outcome of reading a chunk of a job log file.
type Result struct {
	Lines      []string
	NextAnchor int64
	FileSize   int64
	HasMore    bool
	Err        string
}

// ReadTail returns up to limit lines, oldest-first, ending before byte beforeExclusive (0 = end of file).
// NextAnchor is the offset of the oldest line returned, to pass back for the next older page.
func ReadTail(path string, limit int, beforeExclusive int64) Result {
	if limit < 1 {
		limit = 1
	}
	if limit > maxLinesPerRequest {
		limit = maxLinesPerRequest
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Err: "log file not found"}
		}
		return Result{Err: err.Error()}
	}
	defer func() { _ = f.Close() }()

	st, err := f.Stat()
	if err != nil {
		return Result{Err: err.Error()}
	}
	fileSize := st.Size()
	if fileSize == 0 {
		return Result{FileSize: 0}
	}

	end := beforeExclusive
	if end == 0 || end > fileSize {
		end = fileSize
	}
	if end <= 0 {
		return Result{FileSize: fileSize}
	}
	end = trimTrailingNewlines(f, end)
	if end <= 0 {
		return Result{FileSize: fileSize}
	}

	lines, oldestStart, err := readBackwardLines(f, end, limit)
	if err != nil {
		return Result{FileSize: fileSize, Err: err.Error()}
	}
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	for i := range lines {
		if len(lines[i]) > maxLineBytes {
			lines[i] = lines[i][:maxLineBytes]
		}
	}

	hasMore := oldestStart > 0
	return Result{
		Lines:      lines,
		NextAnchor: oldestStart,
		FileSize:   fileSize,
		HasMore:    hasMore,
	}
}

// readBackwardLines returns up to n lines newest-first, and the offset of the oldest one.
func readBackwardLines(f *os.File, endExclusive int64, n int) (lines []string, oldestStart int64, err error) {
	chunk := make([]byte, readChunkSize)
	pos := endExclusive
	var pending []byte
	oldestStart = -1

outer:
	for len(lines) < n && pos > 0 {
		start := pos - int64(len(chunk))
		if start < 0 {
			start = 0
		}
		size := int(pos - start)
		if size == 0 {
			break
		}
		_, err := f.ReadAt(chunk[:size], start)
		if err != nil && err != io.EOF {
			return nil, -1, err
		}
		pos = start
		block := chunk[:size]
		segEnd := len(block)
		i := segEnd - 1
		for i >= 0 && len(lines) < n {
			if block[i] == '\n' {
				segment := block[i+1 : segEnd]
				full := make([]byte, 0, len(segment)+len(pending))
				full = append(full, segment...)
				full = append(full, pending...)
				lineStart := start + int64(i) + 1
				lines = append(lines, string(full))
				if len(lines) == n {
					oldestStart = lineStart
					break outer
				}
				pending = nil
				segEnd = i
			}
			i--
		}
		pending = append(block[:segEnd], pending...)
	}
	if len(lines) < n && len(pending) > 0 {
		lines = append(lines, string(pending))
		oldestStart = 0
	}
	if oldestStart < 0 {
		oldestStart = 0
	}
	return lines, oldestStart, nil
}

func trimTrailingNewlines(f *os.File, end int64) int64 {
	var b [1]byte
	for end > 0 {
		_, err := f.ReadAt(b[:], end-1)
		if err != nil {
			break
		}
		if b[0] != '\n' {
			break
		}
		end--
	}
	return end
}

// ReadHead returns up to limit lines starting at byte startOffset.
// NextAnchor is the offset just past the last line returned, to pass back for the next page.
func ReadHead(path string, limit int, startOffset int64) Result {
	if limit < 1 {
		limit = 1
	}
	if limit > maxLinesPerRequest {
		limit = maxLinesPerRequest
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Err: "log file not found"}
		}
		return Result{Err: err.Error()}
	}
	defer func() { _ = f.Close() }()

	st, err := f.Stat()
	if err != nil {
		return Result{Err: err.Error()}
	}
	fileSize := st.Size()
	if startOffset < 0 {
		startOffset = 0
	}
	if startOffset >= fileSize {
		return Result{FileSize: fileSize, NextAnchor: fileSize, HasMore: false}
	}

	chunk := make([]byte, readChunkSize)
	pos := startOffset
	var lineBuf []byte

	var lines []string
	for len(lines) < limit && pos < fileSize {
		readStart := pos
		toRead := int64(len(chunk))
		if readStart+toRead > fileSize {
			toRead = fileSize - readStart
		}
		n, err := f.ReadAt(chunk[:toRead], readStart)
		if err != nil && err != io.EOF {
			return Result{FileSize: fileSize, Err: err.Error()}
		}
		if n == 0 {
			break
		}
		seg := chunk[:n]
		off := 0
		for off < len(seg) && len(lines) < limit {
			idx := -1
			for i := off; i < len(seg); i++ {
				if seg[i] == '\n' {
					idx = i
					break
				}
			}
			if idx < 0 {
				tail := seg[off:]
				if len(lineBuf)+len(tail) > maxLineBytes {
					lineBuf = append(lineBuf, tail[:maxLineBytes-len(lineBuf)]...)
					lines = append(lines, string(lineBuf))
					lineBuf = lineBuf[:0]
					pos = fileSize
					break
				}
				lineBuf = append(lineBuf, tail...)
				pos = readStart + int64(n)
				break
			}
			piece := seg[off:idx]
			full := append(lineBuf, piece...)
			lineBuf = lineBuf[:0]
			if len(full) > maxLineBytes {
				full = full[:maxLineBytes]
			}
			lines = append(lines, string(full))
			pos = readStart + int64(idx+1)
			off = idx + 1
		}
		if len(lines) >= limit {
			break
		}
	}
	if len(lines) < limit && len(lineBuf) > 0 {
		if len(lineBuf) > maxLineBytes {
			lineBuf = lineBuf[:maxLineBytes]
		}
		lines = append(lines, string(lineBuf))
		pos = fileSize
	}

	nextAnchor := pos
	hasMore := nextAnchor < fileSize

	return Result{
		Lines:      lines,
		NextAnchor: nextAnchor,
		FileSize:   fileSize,
		HasMore:    hasMore,
	}
}
