# Makefile for sqlitezstd
# A pure Go implementation of zstd-seekable compressed SQLite databases

# Configuration
TESTDATA_DIR := testdata
SAMPLE_DB := $(TESTDATA_DIR)/sample.db
SAMPLE_DB_ZST := $(TESTDATA_DIR)/sample.db.zst
CSV_SRC := $(TESTDATA_DIR)/ev_data.csv.zst

# zstdseek settings
ZSTDSEEK_QUALITY := 1
ZSTDSEEK_CHUNKING := 128:1024:8192

# Build tags
BUILD_TAGS := -tags fts5

# Module directories
MODULES := internal/core driver/modernc driver/mattn driver/ncruces examples/modernc examples/mattn examples/ncruces

.PHONY: all build test bench lint format clean help compress examples tools tidy

# Default target
all: format lint test

# Install Go tools
tools:
	go install github.com/SaveTheRbtz/zstd-seekable-format-go/cmd/zstdseek@latest

# Build all packages (root module only, drivers have separate modules)
build:
	go build $(BUILD_TAGS) ./...
	cd driver/modernc && go build .
	cd driver/ncruces && go build .
	CGO_ENABLED=1 && cd driver/mattn && go build .

# Build driver modules
build-drivers:
	cd driver/modernc && go build .
	cd driver/ncruces && go build .
	CGO_ENABLED=1 cd driver/mattn && go build .

# Run tests (root module)
test:
	go test $(BUILD_TAGS) -v ./...

# Run tests with race detector
test-race:
	go test $(BUILD_TAGS) -race -v ./...

# Run benchmarks
bench:
	go test $(BUILD_TAGS) -bench=. -benchmem -run ^$$

# Run linter
lint:
	golangci-lint run --fix --timeout "10m"

# Format code
format:
	gofmt -w .

# Tidy all modules
tidy:
	go mod tidy
	@for dir in $(MODULES); do \
		echo "Tidying $$dir..."; \
		(cd $$dir && go mod tidy); \
	done

# Clean build artifacts
clean:
	go clean
	rm -f $(SAMPLE_DB_ZST)

# Compress a SQLite database using zstd seekable format
# Usage: make compress DB=path/to/database.db
compress:
ifndef DB
	$(error DB is required. Usage: make compress DB=path/to/database.db)
endif
	@echo "Compressing $(DB) to $(DB).zst..."
	zstdseek -f "$(DB)" -o "$(DB).zst" -q $(ZSTDSEEK_QUALITY) -c $(ZSTDSEEK_CHUNKING) -v
	@echo "Done. Output: $(DB).zst"

# Compress the sample database
compress-sample: $(SAMPLE_DB)
	zstdseek -f "$(SAMPLE_DB)" -o "$(SAMPLE_DB_ZST)" -q $(ZSTDSEEK_QUALITY) -c $(ZSTDSEEK_CHUNKING) -v -t

# Create sample database from CSV
$(SAMPLE_DB): $(CSV_SRC)
	@echo "Creating sample database from CSV..."
	DB_NAME=$(SAMPLE_DB) DB_TABLE=samples DB_SRC=$(CSV_SRC) ./scripts/load_csv.sh

# Build and run the modernc example
example-modernc: $(SAMPLE_DB_ZST)
	cd examples/modernc && go run $(BUILD_TAGS) .

# Build and run the mattn example (requires CGO)
example-mattn: $(SAMPLE_DB_ZST)
	cd examples/mattn && CGO_ENABLED=1 go run $(BUILD_TAGS) .

# Build and run the ncruces example
example-ncruces: $(SAMPLE_DB_ZST)
	cd examples/ncruces && go run $(BUILD_TAGS) .

# Run all examples
examples: example-modernc

# Query a compressed database (requires zstd compressed .db.zst file)
# Usage: make query DB=path/to/database.db.zst SQL="SELECT * FROM table LIMIT 10"
query:
ifndef DB
	$(error DB is required. Usage: make query DB=path/to/database.db.zst SQL="SELECT * FROM table")
endif
ifndef SQL
	$(error SQL is required. Usage: make query DB=path/to/database.db.zst SQL="SELECT * FROM table")
endif
	@cd examples/modernc && go run . -db="$(abspath $(DB))" -sql="$(SQL)"

# Show help
help:
	@echo "sqlitezstd Makefile targets:"
	@echo ""
	@echo "  all            - Format, lint, and test (default)"
	@echo "  build          - Build all packages"
	@echo "  build-drivers  - Build driver modules only"
	@echo "  test           - Run tests"
	@echo "  test-race      - Run tests with race detector"
	@echo "  bench          - Run benchmarks"
	@echo "  lint           - Run golangci-lint"
	@echo "  format         - Format code"
	@echo "  tidy           - Run go mod tidy on all modules"
	@echo "  clean          - Clean build artifacts"
	@echo "  tools          - Install required Go tools (zstdseek)"
	@echo ""
	@echo "  compress       - Compress a SQLite database (DB=path/to/file.db)"
	@echo "  compress-sample- Compress the sample database"
	@echo ""
	@echo "  example-modernc- Run modernc driver example (pure Go)"
	@echo "  example-mattn  - Run mattn driver example (CGO required)"
	@echo "  example-ncruces- Run ncruces driver example"
	@echo "  examples       - Run all compatible examples"
	@echo ""
	@echo "  query          - Query a compressed database"
	@echo "                   (DB=path/to/file.db.zst SQL=\"SELECT ...\")"
	@echo ""
	@echo "  help           - Show this help message"
