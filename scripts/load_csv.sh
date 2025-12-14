#!/usr/bin/env bash

#
# Load a zstd compressed csv file into an sqlite database
#

set -eu -o pipefail

DB_NAME=${DB_NAME:-testdata/sample.db}
DB_TABLE=${DB_TABLE:-samples}
DB_SRC=${DB_SRC:-testdata/ev_data.csv.zst}

sqlite3 "$DB_NAME" <<EOF
.mode csv
.import '|zstdcat $DB_SRC' $DB_TABLE
select * from $DB_TABLE;
EOF

