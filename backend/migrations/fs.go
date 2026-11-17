package migrations

import "embed"

// FS holds golang-migrate versioned SQL files. File names must match
// {version}_{title}.up.sql and {version}_{title}.down.sql.
//
//go:embed *.sql
var FS embed.FS
