package sqlitezstd

import (
	"io"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/vfs"
)

// ZstdFile represents an open Zstandard compressed database file.
// It implements the vfs.File interface, providing read-only access to the decompressed
// database content while maintaining the immutability guarantees required by SQLite.
//
// The file wraps three components:
//   - decoder: Zstandard decoder for decompressing data
//   - reader: Underlying data source (file or HTTP reader)
//   - seekable: Seekable reader that enables random access to compressed data
//
// All write operations return errors, as compressed databases are immutable by design.
type ZstdFile struct {
	decoder  *zstd.Decoder
	reader   io.ReadSeeker
	seekable seekable.Reader
}

// Ensure ZstdFile implements the vfs.File interface at compile time.
var _ vfs.File = &ZstdFile{}

// CheckReservedLock checks if a reserved lock is held on the database file.
// Since this VFS is read-only and does not support concurrent write access,
// no locks are ever reserved, so this always returns false.
func (z *ZstdFile) CheckReservedLock() (bool, error) {
	return false, nil
}

// Close releases all resources associated with the file.
// This closes the seekable reader and the underlying data source if it implements io.Closer.
// Errors from closing are intentionally ignored to ensure cleanup always proceeds.
func (z *ZstdFile) Close() error {
	_ = z.seekable.Close()

	if closer, ok := z.reader.(io.Closer); ok {
		_ = closer.Close()
	}

	return nil
}

// DeviceCharacteristics returns flags indicating the behavior of this file type.
// IOCAP_IMMUTABLE indicates that the file will never change once created,
// allowing SQLite to optimize its caching and locking behavior accordingly.
func (z *ZstdFile) DeviceCharacteristics() vfs.DeviceCharacteristic {
	return vfs.IOCAP_IMMUTABLE
}

// Size returns the uncompressed size of the database file.
// The size is determined by seeking to the end of the decompressed stream.
func (z *ZstdFile) Size() (int64, error) {
	return z.seekable.Seek(0, io.SeekEnd)
}

// Lock attempts to acquire a lock on the database file.
// Since this VFS is read-only and multiple concurrent readers are safe,
// locks are always granted without any actual locking mechanism.
func (z *ZstdFile) Lock(elock vfs.LockLevel) error {
	return nil
}

// ReadAt reads len(p) bytes from the decompressed database at the specified offset.
// The seekable reader handles decompression transparently, allowing random access
// to any part of the database without decompressing the entire file.
func (z *ZstdFile) ReadAt(p []byte, off int64) (int, error) {
	return z.seekable.ReadAt(p, off)
}

// SectorSize returns the native sector size of the underlying storage device.
// Returning 0 allows SQLite to use its default sector size assumptions.
func (z *ZstdFile) SectorSize() int {
	return 0
}

// Sync ensures that any buffered data is written to stable storage.
// Since this VFS is read-only, there is no data to sync, so this is a no-op.
func (z *ZstdFile) Sync(flag vfs.SyncFlag) error {
	return nil
}

// Truncate changes the size of the database file.
// This operation is not supported for compressed databases, as they are immutable,
// so it always returns an I/O error.
func (z *ZstdFile) Truncate(size int64) error {
	return sqlite3.IOERR_TRUNCATE
}

// Unlock releases a lock on the database file.
// Since no actual locks are held (all operations are read-only), this is a no-op.
func (z *ZstdFile) Unlock(elock vfs.LockLevel) error {
	return nil
}

// WriteAt attempts to write data to the database file.
// This operation is not supported for compressed databases, as they are immutable,
// so it always returns an I/O error and writes 0 bytes.
func (z *ZstdFile) WriteAt(p []byte, off int64) (int, error) {
	return 0, sqlite3.IOERR_WRITE
}
