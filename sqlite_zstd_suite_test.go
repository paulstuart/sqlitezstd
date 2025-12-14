package sqlitezstd_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/georgysavva/scany/v2/sqlscan"
	_ "github.com/jtarchie/sqlitezstd"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestSqliteZstd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SqliteZstd Suite")
}

const maxSize = 1_000_000

// trackingResponseWriter wraps http.ResponseWriter to track bytes written
type trackingResponseWriter struct {
	http.ResponseWriter
	bytesWritten int64
}

func (tw *trackingResponseWriter) Write(p []byte) (int, error) {
	n, err := tw.ResponseWriter.Write(p)
	tw.bytesWritten += int64(n)
	return n, err
}

func createDatabase() string {
	buildPath, err := os.MkdirTemp("", "")
	Expect(err).ToNot(HaveOccurred())

	dbPath := filepath.Join(buildPath, "test.sqlite")

	client, err := sql.Open("sqlite3", dbPath)
	Expect(err).ToNot(HaveOccurred())

	_, err = client.Exec(`
		CREATE TABLE entries (
			id INTEGER PRIMARY KEY
		);
	`)
	Expect(err).ToNot(HaveOccurred())

	tx, err := client.Begin()
	Expect(err).ToNot(HaveOccurred())
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare("INSERT INTO entries (id) VALUES (?)")
	Expect(err).ToNot(HaveOccurred())
	defer stmt.Close() //nolint: errcheck

	for id := 1; id <= maxSize; id++ {
		_, err = stmt.Exec(id)
		Expect(err).ToNot(HaveOccurred())
	}

	err = tx.Commit()
	Expect(err).ToNot(HaveOccurred())

	zstPath := dbPath + ".zst"

	command := exec.Command(
		"go", "run", "github.com/SaveTheRbtz/zstd-seekable-format-go/cmd/zstdseek",
		"-f", dbPath,
		"-o", zstPath,
	)

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0))

	return zstPath
}

func createComplexDatabase() (string, string) {
	buildPath, err := os.MkdirTemp("", "")
	Expect(err).ToNot(HaveOccurred())

	dbPath := filepath.Join(buildPath, "complex.sqlite")

	client, err := sql.Open("sqlite3", dbPath)
	Expect(err).ToNot(HaveOccurred())
	defer client.Close() //nolint: errcheck

	_, err = client.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT,
			age INTEGER
		);
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY,
			user_id INTEGER,
			product TEXT,
			quantity INTEGER,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`)
	Expect(err).ToNot(HaveOccurred())

	tx, err := client.Begin()
	Expect(err).ToNot(HaveOccurred())
	defer func() { _ = tx.Rollback() }()

	userStmt, err := tx.Prepare("INSERT INTO users (name, age) VALUES (?, ?)")
	Expect(err).ToNot(HaveOccurred())
	defer userStmt.Close() //nolint: errcheck

	orderStmt, err := tx.Prepare("INSERT INTO orders (user_id, product, quantity) VALUES (?, ?, ?)")
	Expect(err).ToNot(HaveOccurred())
	defer orderStmt.Close() //nolint: errcheck

	for i := 1; i <= maxSize; i++ {
		_, err = userStmt.Exec(fmt.Sprintf("User%d", i), 20+(i%60))
		Expect(err).ToNot(HaveOccurred())

		_, err = orderStmt.Exec(i, fmt.Sprintf("Product%d", i%100), i%10+1)
		Expect(err).ToNot(HaveOccurred())
	}

	err = tx.Commit()
	Expect(err).ToNot(HaveOccurred())

	err = client.Close()
	Expect(err).ToNot(HaveOccurred())

	zstPath := dbPath + ".zst"

	command := exec.Command(
		"go", "run", "github.com/SaveTheRbtz/zstd-seekable-format-go/cmd/zstdseek",
		"-f", dbPath,
		"-o", zstPath,
		"-t",
		"-c", "16:32:64",
	)

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0))

	return dbPath, zstPath
}

var _ = Describe("SqliteZSTD", func() {
	It("can read from a compressed sqlite db", func() {
		zstPath := createDatabase()

		client, err := sql.Open("sqlite3", fmt.Sprintf("%s?vfs=zstd", zstPath))
		Expect(err).ToNot(HaveOccurred())
		defer client.Close() //nolint: errcheck

		row := client.QueryRow("SELECT COUNT(*) FROM entries;")
		Expect(row.Err()).ToNot(HaveOccurred())

		var count int64
		err = row.Scan(&count)
		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(BeEquivalentTo(maxSize))
	})

	It("can handle multiple readers", func() {
		zstPath := createDatabase()

		waiter := &sync.WaitGroup{}

		for range 5 {
			waiter.Add(1)

			go func() {
				defer waiter.Done()
				defer GinkgoRecover()

				client, err := sql.Open("sqlite3", fmt.Sprintf("%s?vfs=zstd", zstPath))
				Expect(err).ToNot(HaveOccurred())
				defer client.Close() //nolint: errcheck

				for range 1_000 {
					row := client.QueryRow("SELECT * FROM entries ORDER BY RANDOM() LIMIT 1;")
					Expect(row.Err()).ToNot(HaveOccurred())
				}
			}()
		}

		waiter.Wait()
	})

	When("file does not exist", func() {
		It("returns an error", func() {
			client, err := sql.Open("sqlite3", "file:some.db?vfs=zstd")
			Expect(err).ToNot(HaveOccurred())
			defer client.Close() //nolint: errcheck

			row := client.QueryRow("SELECT * FROM entries ORDER BY RANDOM() LIMIT 1;")
			Expect(row.Err()).To(HaveOccurred())
		})
	})

	It("allows reading from HTTP server", func() {
		zstPath := createDatabase()
		zstDir := filepath.Dir(zstPath)
		server := httptest.NewServer(http.FileServer(http.Dir(zstDir)))
		defer server.Close()

		client, err := sql.Open("sqlite3", fmt.Sprintf("%s/%s?vfs=zstd", server.URL, filepath.Base(zstPath)))
		Expect(err).ToNot(HaveOccurred())
		defer client.Close() //nolint: errcheck

		row := client.QueryRow("SELECT COUNT(*) FROM entries;")
		Expect(row.Err()).ToNot(HaveOccurred())

		var count int64
		err = row.Scan(&count)
		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(BeEquivalentTo(maxSize))
	})

	It("ensures data integrity between compressed and uncompressed databases", func() {
		uncompressedPath, compressedPath := createComplexDatabase()

		uncompressedDB, err := sql.Open("sqlite3", uncompressedPath)
		Expect(err).ToNot(HaveOccurred())
		defer uncompressedDB.Close() //nolint: errcheck

		compressedDB, err := sql.Open("sqlite3", fmt.Sprintf("%s?vfs=zstd", compressedPath))
		Expect(err).ToNot(HaveOccurred())
		defer compressedDB.Close() //nolint: errcheck

		row := compressedDB.QueryRow(`SELECT COUNT(*) FROM users;`)
		Expect(row.Err()).ToNot(HaveOccurred())

		var count int64
		Expect(row.Scan(&count)).ToNot(HaveOccurred())
		Expect(count).To(BeEquivalentTo(maxSize))

		query := `
		  -- since VFS is read-only, it can not be used for files
			-- please use this
			PRAGMA temp_store = memory;
			SELECT u.age, COUNT(*) as order_count, SUM(o.quantity) as total_quantity
			FROM users u
			JOIN orders o ON u.id = o.user_id
			GROUP BY u.age
			ORDER BY u.age
		`

		type Result struct {
			Age           int
			OrderCount    int64
			TotalQuantity int64
		}

		var uncompressedResults, compressedResults []Result

		err = sqlscan.Select(context.Background(), uncompressedDB, &uncompressedResults, query)
		Expect(err).ToNot(HaveOccurred())

		err = sqlscan.Select(context.Background(), compressedDB, &compressedResults, query)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(compressedResults)).To(BeNumerically(">", 0))
		Expect(len(compressedResults)).To(Equal(len(uncompressedResults)), "Compressed and uncompressed databases have different number of rows")

		for i := range uncompressedResults {
			Expect(compressedResults[i]).To(Equal(uncompressedResults[i]), "Row %d does not match between compressed and uncompressed databases", i)
		}
	})

	It("uses HTTP Range headers and only downloads needed bytes", func() {
		zstPath := createDatabase()
		zstDir := filepath.Dir(zstPath)

		// Track HTTP requests
		var totalBytesServed int64
		var rangeRequestCount int64
		var mu sync.Mutex

		// Get the actual file size
		fileInfo, err := os.Stat(zstPath)
		Expect(err).ToNot(HaveOccurred())
		fileSize := fileInfo.Size()

		// Create a custom handler that tracks requests
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if Range header is present
			rangeHeader := r.Header.Get("Range")
			if rangeHeader != "" {
				mu.Lock()
				rangeRequestCount++
				mu.Unlock()
			}

			// Open the file
			file, err := os.Open(filepath.Join(zstDir, filepath.Base(zstPath)))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer file.Close() //nolint: errcheck

			// Wrap the response writer to track bytes
			tw := &trackingResponseWriter{
				ResponseWriter: w,
			}

			// Use http.ServeContent which properly handles Range requests
			http.ServeContent(tw, r, filepath.Base(zstPath), fileInfo.ModTime(), file)

			// Track total bytes served
			mu.Lock()
			totalBytesServed += tw.bytesWritten
			mu.Unlock()
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		// Open database and perform a simple query
		client, err := sql.Open("sqlite3", fmt.Sprintf("%s/%s?vfs=zstd", server.URL, filepath.Base(zstPath)))
		Expect(err).ToNot(HaveOccurred())
		defer client.Close() //nolint: errcheck

		// Perform a simple query that should only require reading a small portion
		// of the database (reading a single row by primary key)
		row := client.QueryRow("SELECT id FROM entries WHERE id = 1;")
		Expect(row.Err()).ToNot(HaveOccurred())

		var id int64
		err = row.Scan(&id)
		Expect(err).ToNot(HaveOccurred())
		Expect(id).To(BeEquivalentTo(1))

		mu.Lock()
		finalBytesServed := totalBytesServed
		finalRangeCount := rangeRequestCount
		mu.Unlock()

		// Verify Range headers were used
		Expect(finalRangeCount).To(BeNumerically(">", 0), "Expected Range requests to be made")

		// The key assertion: we should NOT download the entire file for a simple single-row query
		// Target: download less than 50% of the file
		percentDownloaded := float64(finalBytesServed) / float64(fileSize) * 100
		Expect(percentDownloaded).To(BeNumerically("<", 50.0),
			"Should download less than 50%% of file for single-row query, but downloaded %.2f%%", percentDownloaded)
	})
})
