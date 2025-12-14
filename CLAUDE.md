# Leveraging ZStd compression for a pure Go sqlite implementation

This package was forked from https://github.com/jtarchie/sqlitezstd, with the goal of applying it to work with non-CGO driver, so that one can use compressed sqlite databases without using CGO.

## Goals

- Adapt the codebase so that it can work with Modern C's implementation of SQLite
- Make it so this package can be used by other projects without customization (other than importing the package and referencing it as needed)
- Identify opportunities to leverage ZStd libraries across databases for improvements in compressing a collection of databases (but only after making the fundamentals work)

## Non-Goals

- Supporting anything other than read-only databases

## Engineering guidelines

- Use idiomatic Go
- Leverage the language's capabilities as of the current version of Go (1.25), including the new testing/synctest package
- Avoid using non stdlib packages unless truly called for
- Avoid gratuitous emjoies
- Provide comprehensive unit and integration tests
- Ensure "completed" work passes all existing tests
- Ensure code has comprehensive codoc commenting that explains reasoning and driving factors, avoid commenting on what is actually happening
- Leverage log/slog logging for easier debugging
- Create documents as needed in the doc directory and link to them from the README

## Course corrections

If suggested guidelines are found to be problematic, create/update the file doc/CORRECTIONS.md for review and use as seen fit.
