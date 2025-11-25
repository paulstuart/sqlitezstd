# Zstd Dictionary Support for SQLite Compression

## Goal

Enhance the sqlitezstd package to support zstd dictionaries, enabling significantly better compression ratios for SQLite databases—especially smaller ones or collections of databases with similar schemas.

## Background

### Seekable vs Dictionary Compression

**Seekable zstd** (currently implemented) adds seek tables to compressed files, enabling random access required for SQLite's page-based reads. This is orthogonal to dictionary support.

**Zstd dictionaries** are pre-trained compression models that capture common patterns. When compressing data similar to the training set, dictionaries can dramatically improve compression ratios. This is especially effective for:

- Small files (where the compressor doesn't have enough data to build good statistics)
- Collections of similar data (multiple databases with the same schema)
- SQLite specifically (predictable page structure, B-tree headers, etc.)

These two features can be combined: seekable zstd files compressed with a dictionary.

## Implementation Plan

### Phase 1: Dictionary Infrastructure

1. **Dictionary storage/distribution mechanism**
   - Option A: Embed dictionary in a custom header within the .zst file
   - Option B: External dictionary file referenced via connection string parameter
   - Option C: Well-known dictionary location (e.g., alongside the database)
   - Recommendation: Start with Option B for simplicity, consider Option A for self-contained files

2. **Dictionary training utility**
   - Create a tool to train dictionaries from sample SQLite databases
   - Extract pages from multiple databases to build training corpus
   - Output dictionary file for use during compression/decompression

### Phase 2: VFS Modifications

1. **Modify decoder initialization** to accept dictionary options
   - Update the modernc VFS Open() method to accept dictionary parameters

2. **Connection string parameter** for dictionary path
   - Example: `database.db.zst?vfs=zstd&mode=ro&dict=/path/to/dict`
   - Parse and validate dictionary parameter in Open()

3. **Dictionary loading and caching**
   - Load dictionary once per unique path
   - Cache decoded dictionaries to avoid repeated parsing

### Phase 3: Compression Tooling

1. **Enhance zstdseek wrapper** (or create new tool) to support:
   - `--dict` flag for compression with dictionary
   - Dictionary training mode from sample files

2. **Compression utility for SQLite databases**
   - Input: uncompressed .db file + optional dictionary
   - Output: seekable zstd compressed file

### Phase 4: Testing and Validation

1. **Unit tests** for dictionary loading and decoding
2. **Integration tests** comparing compression ratios with/without dictionaries
3. **Benchmark tests** for decompression performance impact
4. **Dictionary mismatch handling** - verify graceful error when wrong dictionary is provided

## API Design (Draft)

### Connection String

```
database.db.zst?vfs=zstd&mode=ro&zstd_dict=/path/to/dictionary
```

### Dictionary Training CLI

```bash
# Train dictionary from sample databases
sqlitezstd-train --output sqlite.dict db1.db db2.db db3.db

# Compress with dictionary
zstdseek --dict sqlite.dict -f database.db -o database.db.zst
```

## Dependencies

Current libraries already support dictionaries:

- `github.com/klauspost/compress/zstd`: `WithDecoderDicts()`, `WithEncoderDict()`
- `github.com/SaveTheRbtz/zstd-seekable-format-go/pkg`: Accepts zstd encoder/decoder interfaces

## Blockers

Before implementing dictionary support, validate that the modernc driver works correctly with the current seekable zstd approach:

- [ ] Verify basic read operations work with seekable zstd files
- [ ] Confirm page-aligned reads function correctly
- [ ] Test with various database sizes and page sizes

Note: This project focuses on the pure-Go modernc.org/sqlite implementation. CGO-based drivers (mattn/go-sqlite3) and WASM-based drivers (ncruces/go-sqlite3) are out of scope.

## Error Handling

### Dictionary Mismatch

When a file was compressed with dictionary A but the user provides dictionary B (or no dictionary):

1. **Detection**: zstd decompression will fail with a dictionary ID mismatch error
2. **Error message**: Provide a clear error indicating dictionary mismatch, including the expected dictionary ID if available
3. **Graceful degradation**: Not possible—dictionaries must match exactly

### Missing Dictionary

If a file requires a dictionary but none is provided:

1. The zstd decoder will fail with a "dictionary required" error
2. Wrap this error with guidance on providing the `zstd_dict` parameter

## References

- [Zstd Dictionary Compression](https://facebook.github.io/zstd/#small-data)
- [klauspost/compress dictionary API](https://pkg.go.dev/github.com/klauspost/compress/zstd#WithDecoderDicts)
- [Zstd Seekable Format](https://github.com/facebook/zstd/blob/dev/contrib/seekable_format/zstd_seekable_compression_format.md)
