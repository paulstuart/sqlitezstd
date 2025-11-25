module github.com/paulstuart/sqlitezstd/examples/mattn

go 1.25.4

replace github.com/paulstuart/sqlitezstd/driver/mattn => ../../driver/mattn

replace github.com/paulstuart/sqlitezstd/internal/core => ../../internal/core

require github.com/paulstuart/sqlitezstd/driver/mattn v0.0.0-00010101000000-000000000000

require (
	github.com/SaveTheRbtz/zstd-seekable-format-go/pkg v0.8.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/mattn/go-sqlite3 v1.14.32 // indirect
	github.com/paulstuart/sqlitezstd/internal/core v0.0.0-00010101000000-000000000000 // indirect
	github.com/psanford/httpreadat v0.1.0 // indirect
	github.com/psanford/sqlite3vfs v0.0.0-20240315230605-24e1d98cf361 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
)
