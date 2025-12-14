package sqlitezstd_test

import (
	"bytes"
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
	"time"

	"github.com/georgysavva/scany/v2/sqlscan"
	_ "github.com/paulstuart/sqlitezstd/driver/ncruces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func createDatabase(t *testing.T) string {
	t.Helper()

	buildPath, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	dbPath := filepath.Join(buildPath, "test.sqlite")

	client, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)

	_, err = client.Exec(`
		CREATE TABLE entries (
			id INTEGER PRIMARY KEY
		);
	`)
	require.NoError(t, err)

	tx, err := client.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare("INSERT INTO entries (id) VALUES (?)")
	require.NoError(t, err)
	defer stmt.Close() //nolint: errcheck

	for id := 1; id <= maxSize; id++ {
		_, err = stmt.Exec(id)
		require.NoError(t, err)
	}

	err = tx.Commit()
	require.NoError(t, err)

	zstPath := dbPath + ".zst"

	cmd := exec.Command(
		"go", "run", "github.com/SaveTheRbtz/zstd-seekable-format-go/cmd/zstdseek",
		"-f", dbPath,
		"-o", zstPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Start()
	require.NoError(t, err)

	// Wait with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("Command failed with stdout:\n%s", stdout.String())
			t.Logf("Command failed with stderr:\n%s", stderr.String())
		}
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("zstdseek command timed out after 30 seconds")
	}

	return zstPath
}

func createComplexDatabase(t *testing.T) (string, string) {
	t.Helper()

	buildPath, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	dbPath := filepath.Join(buildPath, "complex.sqlite")

	client, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
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
	require.NoError(t, err)

	tx, err := client.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	userStmt, err := tx.Prepare("INSERT INTO users (name, age) VALUES (?, ?)")
	require.NoError(t, err)
	defer userStmt.Close() //nolint: errcheck

	orderStmt, err := tx.Prepare("INSERT INTO orders (user_id, product, quantity) VALUES (?, ?, ?)")
	require.NoError(t, err)
	defer orderStmt.Close() //nolint: errcheck

	for i := 1; i <= maxSize; i++ {
		_, err = userStmt.Exec(fmt.Sprintf("User%d", i), 20+(i%60))
		require.NoError(t, err)

		_, err = orderStmt.Exec(i, fmt.Sprintf("Product%d", i%100), i%10+1)
		require.NoError(t, err)
	}

	err = tx.Commit()
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	zstPath := dbPath + ".zst"

	cmd := exec.Command(
		"go", "run", "github.com/SaveTheRbtz/zstd-seekable-format-go/cmd/zstdseek",
		"-f", dbPath,
		"-o", zstPath,
		"-t",
		"-c", "16:32:64",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Start()
	require.NoError(t, err)

	// Wait with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("Command failed with stdout:\n%s", stdout.String())
			t.Logf("Command failed with stderr:\n%s", stderr.String())
		}
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("zstdseek command timed out after 30 seconds")
	}

	return dbPath, zstPath
}

func TestCanReadFromCompressedDB(t *testing.T) {
	zstPath := createDatabase(t)

	client, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?vfs=zstd", zstPath))
	require.NoError(t, err)
	defer client.Close() //nolint: errcheck

	row := client.QueryRow("SELECT COUNT(*) FROM entries;")
	require.NoError(t, row.Err())

	var count int64
	err = row.Scan(&count)
	require.NoError(t, err)
	assert.EqualValues(t, maxSize, count)
}

func TestCanHandleMultipleReaders(t *testing.T) {
	zstPath := createDatabase(t)

	var wg sync.WaitGroup
	errChan := make(chan error, 5)

	for range 5 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			client, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?vfs=zstd", zstPath))
			if err != nil {
				errChan <- err
				return
			}
			defer client.Close() //nolint: errcheck

			for range 1_000 {
				row := client.QueryRow("SELECT * FROM entries ORDER BY RANDOM() LIMIT 1;")
				if row.Err() != nil {
					errChan <- row.Err()
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		require.NoError(t, err)
	}
}

func TestFileDoesNotExist(t *testing.T) {
	client, err := sql.Open("sqlite3", "file:some.db?vfs=zstd")
	require.NoError(t, err)
	defer client.Close() //nolint: errcheck

	row := client.QueryRow("SELECT * FROM entries ORDER BY RANDOM() LIMIT 1;")
	assert.Error(t, row.Err())
}

func TestReadingFromHTTPServer(t *testing.T) {
	zstPath := createDatabase(t)
	zstDir := filepath.Dir(zstPath)
	server := httptest.NewServer(http.FileServer(http.Dir(zstDir)))
	defer server.Close()

	client, err := sql.Open("sqlite3", fmt.Sprintf("file:%s/%s?vfs=zstd", server.URL, filepath.Base(zstPath)))
	require.NoError(t, err)
	defer client.Close() //nolint: errcheck

	row := client.QueryRow("SELECT COUNT(*) FROM entries;")
	require.NoError(t, row.Err())

	var count int64
	err = row.Scan(&count)
	require.NoError(t, err)
	assert.EqualValues(t, maxSize, count)
}

func TestDataIntegrityBetweenCompressedAndUncompressed(t *testing.T) {
	uncompressedPath, compressedPath := createComplexDatabase(t)

	uncompressedDB, err := sql.Open("sqlite3", "file:"+uncompressedPath)
	require.NoError(t, err)
	defer uncompressedDB.Close() //nolint: errcheck

	compressedDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?vfs=zstd", compressedPath))
	require.NoError(t, err)
	defer compressedDB.Close() //nolint: errcheck

	row := compressedDB.QueryRow(`SELECT COUNT(*) FROM users;`)
	require.NoError(t, row.Err())

	var count int64
	require.NoError(t, row.Scan(&count))
	assert.EqualValues(t, maxSize, count)

	// Execute PRAGMA separately for each database
	_, err = uncompressedDB.Exec(`PRAGMA temp_store = memory;`)
	require.NoError(t, err)

	_, err = compressedDB.Exec(`PRAGMA temp_store = memory;`)
	require.NoError(t, err)

	query := `
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
	require.NoError(t, err)

	err = sqlscan.Select(context.Background(), compressedDB, &compressedResults, query)
	require.NoError(t, err)

	assert.Greater(t, len(compressedResults), 0)
	assert.Equal(t, len(uncompressedResults), len(compressedResults), "Compressed and uncompressed databases have different number of rows")

	for i := range uncompressedResults {
		assert.Equal(t, uncompressedResults[i], compressedResults[i], "Row %d does not match between compressed and uncompressed databases", i)
	}
}

func TestHTTPRangeHeadersOnlyDownloadNeededBytes(t *testing.T) {
	zstPath := createDatabase(t)
	zstDir := filepath.Dir(zstPath)

	// Track HTTP requests
	var totalBytesServed int64
	var rangeRequestCount int64
	var mu sync.Mutex

	// Get the actual file size
	fileInfo, err := os.Stat(zstPath)
	require.NoError(t, err)
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
	client, err := sql.Open("sqlite3", fmt.Sprintf("file:%s/%s?vfs=zstd", server.URL, filepath.Base(zstPath)))
	require.NoError(t, err)
	defer client.Close() //nolint: errcheck

	// Perform a simple query that should only require reading a small portion
	// of the database (reading a single row by primary key)
	row := client.QueryRow("SELECT id FROM entries WHERE id = 1;")
	require.NoError(t, row.Err())

	var id int64
	err = row.Scan(&id)
	require.NoError(t, err)
	assert.EqualValues(t, 1, id)

	mu.Lock()
	finalBytesServed := totalBytesServed
	finalRangeCount := rangeRequestCount
	mu.Unlock()

	// Verify Range headers were used
	assert.Greater(t, finalRangeCount, int64(0), "Expected Range requests to be made")

	// The key assertion: we should NOT download the entire file for a simple single-row query
	// Target: download less than 50% of the file
	percentDownloaded := float64(finalBytesServed) / float64(fileSize) * 100
	assert.Less(t, percentDownloaded, 50.0,
		"Should download less than 50%% of file for single-row query, but downloaded %.2f%%", percentDownloaded)
}
