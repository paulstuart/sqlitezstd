// Package core provides shared functionality for sqlitezstd drivers.
package core

import (
	"errors"
	"io"
)

var (
	errInvalidSize            = errors.New("readerat: invalid size")
	errSeekToInvalidWhence    = errors.New("readerat: seek to invalid whence")
	errSeekToNegativePosition = errors.New("readerat: seek to negative position")
)

// ReadSeeker is an io.ReadSeeker implementation based on an io.ReaderAt.
// This is used to wrap HTTP range readers and other ReaderAt implementations
// to provide seeking functionality for the Zstandard seekable reader.
type ReadSeeker struct {
	ReaderAt io.ReaderAt
	Size     int64
	offset   int64
}

// Read implements io.Reader.
func (r *ReadSeeker) Read(p []byte) (int, error) {
	if r.Size < 0 {
		return 0, errInvalidSize
	}
	if r.Size <= r.offset {
		return 0, io.EOF
	}
	length := r.Size - r.offset
	if int64(len(p)) > length {
		p = p[:length]
	}
	if len(p) == 0 {
		return 0, nil
	}

	actual, err := r.ReaderAt.ReadAt(p, r.offset)
	r.offset += int64(actual)
	if (err == nil) && (r.offset == r.Size) {
		err = io.EOF
	}
	return actual, err
}

// Seek implements io.Seeker.
func (r *ReadSeeker) Seek(offset int64, whence int) (int64, error) {
	if r.Size < 0 {
		return 0, errInvalidSize
	}

	switch whence {
	case io.SeekStart:
		// No-op.
	case io.SeekCurrent:
		offset += r.offset
	case io.SeekEnd:
		offset += r.Size
	default:
		return 0, errSeekToInvalidWhence
	}

	if offset < 0 {
		return 0, errSeekToNegativePosition
	}
	r.offset = offset
	return r.offset, nil
}
