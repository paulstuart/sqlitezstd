// Package mattn provides a Zstandard VFS adapter for mattn/go-sqlite3.
// This adapter enables read-only access to Zstandard-compressed SQLite databases
// using the CGO-based mattn/go-sqlite3 driver.
//
// Note: This requires CGO to be enabled.
//
// Usage:
//
//	import _ "github.com/paulstuart/sqlitezstd/driver/mattn"
//
//	db, err := sql.Open("sqlite3", "database.sqlite.zst?vfs=zstd")
package mattn

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
	_ "github.com/mattn/go-sqlite3"
	"github.com/psanford/httpreadat"
	"github.com/psanford/sqlite3vfs"

	"github.com/paulstuart/sqlitezstd"
)

// ZstdVFS implements the VFS interface for Zstandard compressed databases.
type ZstdVFS struct{}

var _ sqlite3vfs.VFS = &ZstdVFS{}

// Access checks whether a file exists and can be accessed with the specified permissions.
func (z *ZstdVFS) Access(name string, flags sqlite3vfs.AccessFlag) (bool, error) {
	if strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-journal") {
		return false, nil
	}
	return true, nil
}

// Delete always returns an error indicating that the VFS is read-only.
func (z *ZstdVFS) Delete(name string, dirSync bool) error {
	return sqlite3vfs.ReadOnlyError
}

// FullPathname returns the full pathname of a file.
func (z *ZstdVFS) FullPathname(name string) string {
	return name
}

// Open opens a compressed database file for reading.
func (z *ZstdVFS) Open(name string, flags sqlite3vfs.OpenFlag) (sqlite3vfs.File, sqlite3vfs.OpenFlag, error) {
	var (
		err    error
		reader io.ReadSeeker
	)

	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		uri, err := url.Parse(name)
		if err != nil {
			return nil, 0, sqlite3vfs.CantOpenError
		}

		httpRanger := httpreadat.New(uri.String())
		size, err := httpRanger.Size()
		if err != nil {
			return nil, 0, sqlite3vfs.CantOpenError
		}

		reader = &sqlitezstd.ReadSeeker{
			ReaderAt: httpRanger,
			Size:     size,
		}
	} else {
		reader, err = os.Open(name)
		if err != nil {
			return nil, 0, sqlite3vfs.CantOpenError
		}
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, 0, sqlite3vfs.CantOpenError
	}

	seekable, err := seekable.NewReader(reader, decoder)
	if err != nil {
		return nil, 0, sqlite3vfs.CantOpenError
	}

	return &ZstdFile{
		decoder:  decoder,
		reader:   reader,
		seekable: seekable,
	}, flags | sqlite3vfs.OpenReadOnly, nil
}

// ZstdFile represents an open Zstandard compressed database file for mattn driver.
type ZstdFile struct {
	decoder  *zstd.Decoder
	reader   io.ReadSeeker
	seekable seekable.Reader
}

var _ sqlite3vfs.File = &ZstdFile{}

func (z *ZstdFile) CheckReservedLock() (bool, error) {
	return false, nil
}

func (z *ZstdFile) Close() error {
	_ = z.seekable.Close()
	if closer, ok := z.reader.(io.Closer); ok {
		_ = closer.Close()
	}
	return nil
}

func (z *ZstdFile) DeviceCharacteristics() sqlite3vfs.DeviceCharacteristic {
	return sqlite3vfs.IocapImmutable
}

func (z *ZstdFile) FileSize() (int64, error) {
	return z.seekable.Seek(0, io.SeekEnd)
}

func (z *ZstdFile) Lock(elock sqlite3vfs.LockType) error {
	return nil
}

func (z *ZstdFile) ReadAt(p []byte, off int64) (int, error) {
	return z.seekable.ReadAt(p, off)
}

func (z *ZstdFile) SectorSize() int64 {
	return 0
}

func (z *ZstdFile) Sync(flag sqlite3vfs.SyncType) error {
	return nil
}

func (z *ZstdFile) Truncate(size int64) error {
	return sqlite3vfs.ReadOnlyError
}

func (z *ZstdFile) Unlock(elock sqlite3vfs.LockType) error {
	return nil
}

func (z *ZstdFile) WriteAt(p []byte, off int64) (int, error) {
	return 0, sqlite3vfs.ReadOnlyError
}

var once = sync.OnceValue(func() error {
	err := sqlite3vfs.RegisterVFS("zstd", &ZstdVFS{})
	if err != nil {
		return fmt.Errorf("could not register vfs: %w", err)
	}
	return nil
})

func init() {
	err := once()
	if err != nil {
		panic(fmt.Sprintf("could not register vfs: %v", err))
	}
}
