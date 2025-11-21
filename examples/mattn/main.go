// Example usage with mattn/go-sqlite3 driver (CGO-based, traditional)
// Note: Requires CGO to be enabled
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/paulstuart/sqlitezstd/driver/mattn"
)

var (
	dbfile  = "testdata/sample.db.zst"
	options = "?vfs=zstd&mode=ro"
)

func main() {
	if len(os.Args) > 1 {
		dbfile = os.Args[1]
	}

	if stat, err := os.Stat(dbfile); err != nil || stat.IsDir() {
		log.Fatalf("Database file %s does not exist", dbfile)
	}
	dbfile += options
	log.Printf("Opening database file: %s", dbfile)

	// Open a compressed SQLite database
	// mattn works with or without "file:" prefix
	db, err := sql.Open("sqlite3", dbfile)
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
	err = db.QueryRow("SELECT COUNT(*) FROM samples").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Database has %d samples\n", count)

	if false {
		os.Exit(0)
	}
	// TODO: http support is not tested because we want a reliable way to serve the data via http,
	// this is readily done with a simple go http server but there are other things to fix/test first
	//
	// mattn also supports HTTP URLs with the VFS
	// httpDB, err := sql.Open("sqlite3", "https://example.com/database.sqlite.zst?vfs=zstd")
	// Note: file URI uses "file:" prefix; "file:///" is for absolute paths only
	// Also note: dbfile already has options appended from line 34
	uri := "file:" + dbfile
	log.Println("Opening file URI: ", uri)
	httpDB, err := sql.Open("sqlite3", uri)
	if err != nil {
		log.Fatal(err)
	}
	defer httpDB.Close()

	_, err = httpDB.Exec("PRAGMA temp_store = memory;")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully opened HTTP database")

	err = httpDB.QueryRow("SELECT COUNT(*) FROM samples").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Database has %d samples\n", count)

}
