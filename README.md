# SQLiteZSTD: Multi-Driver Pure Go Read-Only Access to Compressed SQLite Files

## Description

SQLiteZSTD provides Go adapters for accessing SQLite databases compressed with
[Zstandard seekable (zstd)](https://github.com/facebook/zstd/blob/216099a73f6ec19c246019df12a2877dada45cca/contrib/seekable_format/zstd_seekable_compression_format.md)
in a read-only manner.

**Multi-Driver Support**: Works with any Go SQLite driver:

- **ncruces/go-sqlite3** - Pure Go WASM-based (default, no CGO)
- **mattn/go-sqlite3** - CGO-based (traditional)
- **modernc.org/sqlite** - Pure Go transliterated C

Please note, SQLiteZSTD is specifically designed for reading data and **does not
support write operations**.

## Features

1. **Multiple SQLite driver support** - Works with ncruces, mattn, and modernc drivers
2. **Pure Go options available** - No CGO dependencies required (ncruces or modernc)
3. **Read-only access** to Zstandard-compressed SQLite databases
4. **Seekable compression** - Random access to database content without full decompression
5. **HTTP/HTTPS support** - Read compressed databases directly from web servers using Range requests
6. **Standard database/sql interface** - Works with existing Go database code
7. **Virtual File System (VFS)** - Custom VFS implementation for transparent decompression

## Installation

```bash
# Default (ncruces/go-sqlite3 - pure Go, no CGO)
go get github.com/paulstuart/sqlitezstd

# Or get specific driver adapters
go get github.com/paulstuart/sqlitezstd/driver/ncruces  # Pure Go WASM
go get github.com/paulstuart/sqlitezstd/driver/mattn    # CGO-based
go get github.com/paulstuart/sqlitezstd/driver/modernc  # Pure Go
```

## Usage

### Option 1: Default Driver (ncruces - Pure Go, no CGO)

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    _ "github.com/paulstuart/sqlitezstd"  // Uses ncruces by default
)

func main() {
    // Open a compressed SQLite database
    db, err := sql.Open("sqlite3", "file:path/to/database.sqlite.zst?vfs=zstd")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Set PRAGMA to use memory for temporary storage (required for read-only VFS)
    _, err = db.Exec("PRAGMA temp_store = memory;")
    if err != nil {
        log.Fatal(err)
    }

    // Query the database
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM your_table").Scan(&count)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Table has %d rows\n", count)
}
```

### Option 2: mattn/go-sqlite3 (CGO-based)

```go
package main

import (
    "database/sql"
    _ "github.com/paulstuart/sqlitezstd/driver/mattn"
)

func main() {
    // Note: mattn doesn't require "file:" prefix
    db, err := sql.Open("sqlite3", "database.sqlite.zst?vfs=zstd")
    // ...
}
```

### Option 3: modernc.org/sqlite (Pure Go)

```go
package main

import (
    "database/sql"
    _ "github.com/paulstuart/sqlitezstd/driver/modernc"
)

func main() {
    // modernc uses "sqlite" as the driver name
    db, err := sql.Open("sqlite", "file:database.sqlite.zst?vfs=zstd")
    // ...
}
```

### Reading from HTTP/HTTPS

All drivers support reading from HTTP servers:

```go
// Open a compressed database from a web server
db, err := sql.Open("sqlite3", "file:https://example.com/database.sqlite.zst?vfs=zstd")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// The VFS will use HTTP Range requests to fetch only the needed data
```

### Important Notes

- **ncruces driver**: Use `file:` URI scheme in connection string
- **mattn driver**: Works with or without `file:` prefix
- **modernc driver**: Use driver name `"sqlite"` (not `"sqlite3"`)
- **All drivers**: Add `?vfs=zstd` to specify the Zstandard VFS
- **All drivers**: Set `PRAGMA temp_store = memory` to avoid temporary file creation
- The database file must be compressed using the Zstandard seekable format (see below)

## Compressing Your Database

Your database needs to be compressed in the seekable Zstandard format:

```bash
go install github.com/SaveTheRbtz/zstd-seekable-format-go/cmd/zstdseek@latest

zstdseek -f your_database.sqlite -o your_database.sqlite.zst
```

The CLI provides different options for compression levels and chunk sizes.

## Driver Comparison

| Feature | ncruces | mattn | modernc |
|---------|---------|-------|---------|
| CGO Required | ❌ No | ✅ Yes | ❌ No |
| Cross-compilation | ✅ Easy | ❌ Hard | ✅ Easy |
| Performance | Good | Excellent | Good |
| Maturity | Newer | Very Mature | Mature |
| Binary Size | Medium | Small | Larger |
| Platform Support | All Go platforms | CGO platforms | All Go platforms |

## Performance

Compressed databases offer significant storage savings while maintaining good read performance.
The seekable format allows random access without decompressing the entire file.

Example compression ratios:
- Text-heavy databases: 60-80% reduction
- Mixed content databases: 40-60% reduction
- Already-compressed data: Minimal reduction

HTTP Range request support means only the needed portions of the compressed database are
downloaded, making it efficient for remote database access.

## License

See LICENSE file for details.

## Credits

Originally forked from [jtarchie/sqlitezstd](https://github.com/jtarchie/sqlitezstd),
adapted to support multiple SQLite drivers.
