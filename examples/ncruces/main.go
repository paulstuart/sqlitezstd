// Example usage with ncruces/go-sqlite3 driver (pure Go, WASM-based, no CGO)
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/paulstuart/sqlitezstd/driver/ncruces"
)

func main() {
	// Open a compressed SQLite database
	// ncruces requires the "file:" URI scheme
	uri := "file:testdata/sample.db.zst?vfs=zstd&mode=ro"
	db, err := sql.Open("sqlite3", uri)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Println("Opened database file:", uri)

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
	// httpDB, err := sql.Open("sqlite3", "file:https://example.com/database.sqlite.zst?vfs=zstd")
	// ...
}
