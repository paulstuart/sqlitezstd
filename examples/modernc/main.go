// Example usage with modernc.org/sqlite driver (pure Go, transliterated C)
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/paulstuart/sqlitezstd/driver/modernc"
)

func main() {
	dbPath := flag.String("db", "../../testdata/sample.db.zst", "path to zstd-compressed SQLite database")
	sqlQuery := flag.String("sql", "", "SQL query to execute (if empty, runs default count query)")
	flag.Parse()

	// Open a compressed SQLite database
	// modernc uses "sqlite" (not "sqlite3") as the driver name
	// and requires the "file:" URI scheme
	// Note: modernc.org/sqlite/vfs generates a dynamic VFS name, so we must
	// retrieve it via VFSName() rather than using a hardcoded "zstd" name
	vfsName := modernc.VFSName()
	uri := fmt.Sprintf("file:%s?vfs=%s&mode=ro", *dbPath, vfsName)
	db, err := sql.Open("sqlite", uri)
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

	// If a custom SQL query was provided, execute it
	if *sqlQuery != "" {
		rows, err := db.Query(*sqlQuery)
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		// Get column names
		cols, err := rows.Columns()
		if err != nil {
			log.Fatal(err)
		}

		// Print header
		for i, col := range cols {
			if i > 0 {
				fmt.Print("\t")
			}
			fmt.Print(col)
		}
		fmt.Println()

		// Prepare value holders
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Print rows
		for rows.Next() {
			if err := rows.Scan(valuePtrs...); err != nil {
				log.Fatal(err)
			}
			for i, v := range values {
				if i > 0 {
					fmt.Print("\t")
				}
				fmt.Print(v)
			}
			fmt.Println()
		}
		if err := rows.Err(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	// Default behavior: count rows in samples table
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM samples").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Database has %d samples\n", count)
}
