// Package db встраивает SQL-схему в бинарник.
package db

import _ "embed"

// Schema — содержимое schema.sql, применяется store.Migrate.
//
//go:embed schema.sql
var Schema string
