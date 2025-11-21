// Package ncruces provides a Zstandard VFS adapter for ncruces/go-sqlite3.
// This adapter enables read-only access to Zstandard-compressed SQLite databases
// using the pure Go WASM-based ncruces/go-sqlite3 driver.
//
// Usage:
//
//	import _ "github.com/paulstuart/sqlitezstd/driver/ncruces"
//
//	db, err := sql.Open("sqlite3", "file:database.sqlite.zst?vfs=zstd")
package ncruces

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/vfs"
	"github.com/psanford/httpreadat"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/paulstuart/sqlitezstd/internal/core"
)

// ZstdVFS implements the VFS interface for Zstandard compressed databases.
type ZstdVFS struct{}

var _ vfs.VFS = &ZstdVFS{}

// Access checks whether a file exists and can be accessed with the specified permissions.
func (z *ZstdVFS) Access(name string, flags vfs.AccessFlag) (bool, error) {
	if strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-journal") {
		return false, nil
	}
	return true, nil
}

// Delete always returns an error indicating that the VFS is read-only.
func (z *ZstdVFS) Delete(name string, dirSync bool) error {
	return sqlite3.IOERR_DELETE
}

// FullPathname returns the full pathname of a file.
func (z *ZstdVFS) FullPathname(name string) (string, error) {
	return name, nil
}

// Open opens a compressed database file for reading.
func (z *ZstdVFS) Open(name string, flags vfs.OpenFlag) (vfs.File, vfs.OpenFlag, error) {
	var (
		err    error
		reader io.ReadSeeker
	)

	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		uri, err := url.Parse(name)
		if err != nil {
			return nil, 0, sqlite3.CANTOPEN
		}

		httpRanger := httpreadat.New(uri.String())
		size, err := httpRanger.Size()
		if err != nil {
			return nil, 0, sqlite3.CANTOPEN
		}

		reader = &core.ReadSeeker{
			ReaderAt: httpRanger,
			Size:     size,
		}
	} else {
		reader, err = os.Open(name)
		if err != nil {
			return nil, 0, sqlite3.CANTOPEN
		}
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, 0, sqlite3.CANTOPEN
	}

	seekable, err := seekable.NewReader(reader, decoder)
	if err != nil {
		return nil, 0, sqlite3.CANTOPEN
	}

	return &ZstdFile{
		decoder:  decoder,
		reader:   reader,
		seekable: seekable,
	}, flags | vfs.OPEN_READONLY, nil
}

// ZstdFile represents an open Zstandard compressed database file for ncruces driver.
type ZstdFile struct {
	decoder  *zstd.Decoder
	reader   io.ReadSeeker
	seekable seekable.Reader
}

var _ vfs.File = &ZstdFile{}

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

func (z *ZstdFile) DeviceCharacteristics() vfs.DeviceCharacteristic {
	return vfs.IOCAP_IMMUTABLE
}

func (z *ZstdFile) Size() (int64, error) {
	return z.seekable.Seek(0, io.SeekEnd)
}

func (z *ZstdFile) Lock(elock vfs.LockLevel) error {
	return nil
}

func (z *ZstdFile) ReadAt(p []byte, off int64) (int, error) {
	return z.seekable.ReadAt(p, off)
}

func (z *ZstdFile) SectorSize() int {
	return 0
}

func (z *ZstdFile) Sync(flag vfs.SyncFlag) error {
	return nil
}

func (z *ZstdFile) Truncate(size int64) error {
	return sqlite3.IOERR_TRUNCATE
}

func (z *ZstdFile) Unlock(elock vfs.LockLevel) error {
	return nil
}

func (z *ZstdFile) WriteAt(p []byte, off int64) (int, error) {
	return 0, sqlite3.IOERR_WRITE
}

var once = sync.OnceValue(func() error {
	vfs.Register("zstd", &ZstdVFS{})
	return nil
})

func init() {
	err := once()
	if err != nil {
		panic(fmt.Sprintf("could not register vfs: %v", err))
	}
}
