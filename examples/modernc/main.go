// Example usage with modernc.org/sqlite driver (pure Go, transliterated C)
package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/paulstuart/sqlitezstd/driver/modernc"
)

func main() {
	// Open a compressed SQLite database
	// modernc uses "sqlite" (not "sqlite3") as the driver name
	// and requires the "file:" URI scheme
	// Note: modernc.org/sqlite/vfs generates a dynamic VFS name, so we must
	// retrieve it via VFSName() rather than using a hardcoded "zstd" name
	vfsName := modernc.VFSName()
	uri := fmt.Sprintf("file:testdata/sample.db.zst?vfs=%s&mode=ro", vfsName)
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Println("Opened database file: ", uri)

	// Set PRAGMA to use memory for temporary storage (required for read-only VFS)
	_, err = db.Exec("PRAGMA temp_store = memory;")
	if err != nil {
		log.Fatal(err)
	}

	// Query the database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM samples").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Database has %d samples\n", count)

	// HTTP URL example (commented out - would need a real zstd-compressed database URL)
	// httpURI := fmt.Sprintf("file:https://example.com/database.sqlite.zst?vfs=%s", vfsName)
	// httpDB, err := sql.Open("sqlite", httpURI)
	// ...
}
