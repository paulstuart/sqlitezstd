# SQLiteZSTD: Pure Go Read-Only Access to Compressed SQLite Files

## Description

SQLiteZSTD provides a pure Go (no CGO) solution for accessing SQLite databases compressed with
[Zstandard seekable (zstd)](https://github.com/facebook/zstd/blob/216099a73f6ec19c246019df12a2877dada45cca/contrib/seekable_format/zstd_seekable_compression_format.md)
in a read-only manner. It leverages the [ncruces/go-sqlite3](https://github.com/ncruces/go-sqlite3)
WASM-based SQLite driver, eliminating the need for CGO while maintaining full SQLite compatibility.

Please note, SQLiteZSTD is specifically designed for reading data and **does not
support write operations**.

## Features

1. **Pure Go implementation** - No CGO dependencies, works on all platforms supported by Go
2. **Read-only access** to Zstandard-compressed SQLite databases
3. **Seekable compression** - Random access to database content without full decompression
4. **HTTP/HTTPS support** - Read compressed databases directly from web servers using Range requests
5. **Standard database/sql interface** - Works with existing Go database code
6. **Virtual File System (VFS)** - Custom VFS implementation for transparent decompression

## Usage

Your database needs to be compressed in the seekable Zstd format. I recommend
using this [CLI tool](github.com/SaveTheRbtz/zstd-seekable-format-go):

```bash
go get -a github.com/SaveTheRbtz/zstd-seekable-format-go/...
go run github.com/SaveTheRbtz/zstd-seekable-format-go/cmd/zstdseek \
    -f <dbPath> \
    -o <dbPath>.zst
```

The CLI provides different options for compression levels, but I do not have
specific recommendations for best usage patterns.

Below is an example of how to use SQLiteZSTD in a Go program:

```go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    _ "github.com/paulstuart/sqlitezstd"
)

func main() {
    // Open a compressed SQLite database
    // Note: Use the file: URI scheme and ?vfs=zstd parameter
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

### Reading from HTTP/HTTPS

You can also read compressed databases directly from HTTP servers:

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

- Always use the `file:` URI scheme in the connection string
- Add `?vfs=zstd` to specify the Zstandard VFS
- Set `PRAGMA temp_store = memory` to avoid temporary file creation (required for read-only VFS)
- The database file must be compressed using the Zstandard seekable format (see compression instructions below)

## Performance

Here's a simple benchmark comparing performance between reading from an
uncompressed vs. a compressed SQLite database, involving the insertion of 10k
records and retrieval of the `MAX` value (without an index) and FTS5.

```
BenchmarkReadUncompressedSQLite-4              	  159717	      7459 ns/op	     473 B/op	      15 allocs/op
BenchmarkReadUncompressedSQLiteFTS5Porter-4    	    2478	    471685 ns/op	     450 B/op	      15 allocs/op
BenchmarkReadUncompressedSQLiteFTS5Trigram-4   	     100	  10449792 ns/op	     542 B/op	      16 allocs/op
BenchmarkReadCompressedSQLite-4                	  266703	      3877 ns/op	    2635 B/op	      15 allocs/op
BenchmarkReadCompressedSQLiteFTS5Porter-4      	    2335	    487430 ns/op	   33992 B/op	      16 allocs/op
BenchmarkReadCompressedSQLiteFTS5Trigram-4     	      48	  21235303 ns/op	45970431 B/op	     148 allocs/op
BenchmarkReadCompressedHTTPSQLite-4            	  284820	      4341 ns/op	    3312 B/op	      15 allocs/op
```
