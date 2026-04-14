package joblog

import (
	"io"
	"os"
)

const (
	// MaxLinesPerRequest caps lines returned in one response (gRPC message size).
	MaxLinesPerRequest = 500
	// MaxLineBytes truncates a single line beyond this size.
	MaxLineBytes = 1 << 20
	// readChunkSize is the buffer size for backward file reads.
	readChunkSize = 256 * 1024
)

// Result is the outcome of reading a chunk of a job log file.
type Result struct {
	Lines      []string
	NextAnchor int64
	FileSize   int64
	HasMore    bool
	Err        string
}

// ReadTail reads up to `limit` complete lines strictly before byte `beforeExclusive`.
// If beforeExclusive is 0, reads from end of file. Lines are returned oldest-first
// within the chunk. NextAnchor is the file offset of the first byte of the oldest
// line returned (for the next "older" request). HasMore is true if more lines exist
// before NextAnchor.
func ReadTail(path string, limit int, beforeExclusive int64) Result {
	if limit < 1 {
		limit = 1
	}
	if limit > MaxLinesPerRequest {
		limit = MaxLinesPerRequest
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
	// reverse to oldest-first
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	for i := range lines {
		if len(lines[i]) > MaxLineBytes {
			lines[i] = lines[i][:MaxLineBytes]
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

// readBackwardLines reads up to n lines ending strictly before endExclusive.
// Returns lines newest-first and the file offset of the first byte of the oldest line.
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

// ReadHead reads up to `limit` complete lines starting at byte `startOffset`.
// Lines are returned oldest-first. NextAnchor is the offset after the last consumed
// newline (or end of file). HasMore is true if more bytes remain after NextAnchor.
func ReadHead(path string, limit int, startOffset int64) Result {
	if limit < 1 {
		limit = 1
	}
	if limit > MaxLinesPerRequest {
		limit = MaxLinesPerRequest
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
				if len(lineBuf)+len(tail) > MaxLineBytes {
					lineBuf = append(lineBuf, tail[:MaxLineBytes-len(lineBuf)]...)
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
			if len(full) > MaxLineBytes {
				full = full[:MaxLineBytes]
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
		if len(lineBuf) > MaxLineBytes {
			lineBuf = lineBuf[:MaxLineBytes]
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
