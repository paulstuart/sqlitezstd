module github.com/paulstuart/sqlitezstd/driver/modernc

go 1.25.4

replace github.com/paulstuart/sqlitezstd/internal/core => ../../internal/core

require (
	github.com/SaveTheRbtz/zstd-seekable-format-go/pkg v0.8.0
	github.com/klauspost/compress v1.18.1
	github.com/paulstuart/sqlitezstd/internal/core v0.0.0-00010101000000-000000000000
	github.com/psanford/httpreadat v0.1.0
	modernc.org/sqlite v1.40.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	modernc.org/libc v1.66.10 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
