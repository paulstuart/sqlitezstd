// Package modernc provides a Zstandard VFS adapter for modernc.org/sqlite.
// This adapter enables read-only access to Zstandard-compressed SQLite databases
// using the pure Go modernc.org/sqlite driver.
//
// Usage:
//
//	import _ "github.com/paulstuart/sqlitezstd/driver/modernc"
//
//	db, err := sql.Open("sqlite", "file:database.sqlite.zst?vfs=zstd")
package modernc

import (
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
	"github.com/psanford/httpreadat"
	_ "modernc.org/sqlite"
	"modernc.org/sqlite/vfs"

	"github.com/paulstuart/sqlitezstd"
)

// ZstdFS implements a read-only fs.FS backed by Zstandard compressed files.
// This adapter allows modernc.org/sqlite to read compressed databases.
type ZstdFS struct {
	openFiles map[string]*ZstdFile
	mu        sync.Mutex
}

// Open opens a compressed file for reading.
func (z *ZstdFS) Open(name string) (fs.File, error) {
	var (
		err    error
		reader io.ReadSeeker
	)

	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		uri, err := url.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}

		httpRanger := httpreadat.New(uri.String())
		size, err := httpRanger.Size()
		if err != nil {
			return nil, fmt.Errorf("failed to get size: %w", err)
		}

		reader = &sqlitezstd.ReadSeeker{
			ReaderAt: httpRanger,
			Size:     size,
		}
	} else {
		reader, err = os.Open(name)
		if err != nil {
			return nil, err
		}
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	seekable, err := seekable.NewReader(reader, decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to create seekable reader: %w", err)
	}

	file := &ZstdFile{
		name:     name,
		decoder:  decoder,
		reader:   reader,
		seekable: seekable,
	}

	z.mu.Lock()
	if z.openFiles == nil {
		z.openFiles = make(map[string]*ZstdFile)
	}
	z.openFiles[name] = file
	z.mu.Unlock()

	return file, nil
}

// ZstdFile represents an open Zstandard compressed database file for modernc driver.
type ZstdFile struct {
	name     string
	decoder  *zstd.Decoder
	reader   io.ReadSeeker
	seekable seekable.Reader
	offset   int64
}

// Stat returns file information.
func (z *ZstdFile) Stat() (fs.FileInfo, error) {
	size, err := z.seekable.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	_, err = z.seekable.Seek(z.offset, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return &fileInfo{name: z.name, size: size}, nil
}

// Read reads data from the compressed file.
func (z *ZstdFile) Read(p []byte) (int, error) {
	n, err := z.seekable.ReadAt(p, z.offset)
	z.offset += int64(n)
	return n, err
}

// Seek sets the offset for the next Read. It implements io.Seeker,
// which is required by modernc.org/sqlite/vfs for random access.
func (z *ZstdFile) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = z.offset + offset
	case io.SeekEnd:
		size, err := z.seekable.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, err
		}
		newOffset = size + offset
	}
	z.offset = newOffset
	return newOffset, nil
}

// Close closes the file.
func (z *ZstdFile) Close() error {
	_ = z.seekable.Close()
	if closer, ok := z.reader.(io.Closer); ok {
		_ = closer.Close()
	}
	return nil
}

// fileInfo implements fs.FileInfo.
type fileInfo struct {
	name string
	size int64
}

func (f *fileInfo) Name() string       { return f.name }
func (f *fileInfo) Size() int64        { return f.size }
func (f *fileInfo) Mode() fs.FileMode  { return 0444 }
func (f *fileInfo) ModTime() time.Time { return time.Time{} }
func (f *fileInfo) IsDir() bool        { return false }
func (f *fileInfo) Sys() interface{}   { return nil }

var (
	zstdFS  *ZstdFS
	vfsName string
	once    = sync.OnceValue(func() error {
		zstdFS = &ZstdFS{}
		name, _, err := vfs.New(zstdFS)
		if err != nil {
			return fmt.Errorf("could not register vfs: %w", err)
		}
		vfsName = name
		return nil
	})
)

// VFSName returns the registered VFS name. Since modernc.org/sqlite/vfs
// generates a unique name at registration time, this function provides
// access to that name for use in connection strings.
func VFSName() string {
	_ = once() // ensure VFS is registered
	return vfsName
}

func init() {
	err := once()
	if err != nil {
		panic(fmt.Sprintf("could not register vfs: %v", err))
	}
}
