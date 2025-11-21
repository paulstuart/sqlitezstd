// Package sqlitezstd provides a pure Go SQLite Virtual File System (VFS) implementation
// that enables read-only access to Zstandard seekable compressed SQLite databases.
//
// The package registers a VFS named "zstd" that can be used with the ncruces/go-sqlite3
// database driver, eliminating the need for CGO while maintaining compatibility with
// compressed database files.
//
// Key features:
//   - Pure Go implementation with no CGO dependencies
//   - Support for local file system and HTTP-based database access
//   - Seamless integration with database/sql using ncruces/go-sqlite3 driver
//   - Optimized for read-only operations on immutable databases
//
// The VFS automatically handles decompression transparently, allowing standard SQL
// operations on compressed databases without modification to application code beyond
// specifying the VFS in the connection string.
package sqlitezstd

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
)

// ZstdVFS implements the VFS interface for Zstandard compressed databases.
// This VFS provides read-only access to seekable Zstandard compressed SQLite databases,
// supporting both local file system paths and HTTP/HTTPS URLs.
//
// The implementation is designed for immutable databases and rejects write operations,
// journal files, and WAL mode to maintain data integrity and simplify the architecture.
type ZstdVFS struct{}

// Ensure ZstdVFS implements the vfs.VFS interface at compile time.
var _ vfs.VFS = &ZstdVFS{}

// Access checks whether a file exists and can be accessed with the specified permissions.
// For journal and WAL files, it always returns false to prevent SQLite from attempting
// to create or modify these files, as the VFS is read-only.
//
// For the main database file, it returns true, allowing SQLite to proceed with opening
// the compressed database for reading.
func (z *ZstdVFS) Access(name string, flags vfs.AccessFlag) (bool, error) {
	if strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-journal") {
		return false, nil
	}

	return true, nil
}

// Delete always returns an error indicating that the VFS is read-only.
// This method is part of the VFS interface but is not supported for compressed databases,
// as they are immutable by design.
func (z *ZstdVFS) Delete(name string, dirSync bool) error {
	return sqlite3.IOERR_DELETE
}

// FullPathname returns the full pathname of a file.
// For this implementation, the name is returned as-is since it already represents
// the complete path (either a file system path or URL).
func (z *ZstdVFS) FullPathname(name string) (string, error) {
	return name, nil
}

// Open opens a compressed database file for reading.
// The method supports both local file system paths and HTTP/HTTPS URLs, automatically
// detecting the appropriate access method based on the name prefix.
//
// For HTTP(S) URLs, it uses HTTP Range requests to enable efficient seeking without
// downloading the entire database. For local files, it uses standard file system operations.
//
// The opened file is wrapped in a Zstandard seekable reader that handles decompression
// transparently, allowing SQLite to read the database as if it were uncompressed.
//
// Returns an error if the file cannot be opened, parsed as a URL, or initialized as a
// seekable Zstandard reader.
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

		reader = &ReadSeeker{
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

// once ensures VFS registration happens exactly once during package initialization.
// Using sync.OnceValue provides thread-safe lazy initialization while caching any
// registration error for subsequent callers.
var once = sync.OnceValue(func() error {
	vfs.Register("zstd", &ZstdVFS{})
	return nil
})

// Init initializes the VFS registration.
// This function is a no-op maintained for backward compatibility.
// The VFS is automatically registered via the init function.
func Init() error {
	return nil
}

// init registers the Zstandard VFS with SQLite during package initialization.
// The VFS is registered under the name "zstd" and can be used by specifying
// "?vfs=zstd" in the database connection string.
//
// Panics if VFS registration fails, as this indicates a fundamental initialization
// error that would prevent the package from functioning correctly.
func init() {
	err := once()
	if err != nil {
		panic(fmt.Sprintf("could not register vfs: %v", err))
	}
}
