package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed schema.sql
var schema string

// Open: buat database kalau belum ada, lalu jalankan schema (CREATE TABLE IF NOT EXISTS, idempotent).
func Open(host, port, user, pass, name string, fresh bool) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, pass, host, port)
	root, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if fresh { // = migrate:fresh (dev only)
		root.Exec("DROP DATABASE IF EXISTS `" + name + "`")
	}
	if _, err = root.Exec("CREATE DATABASE IF NOT EXISTS `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return nil, err
	}
	root.Close()

	d, err := sql.Open("mysql", dsn+name+"?parseTime=false&charset=utf8mb4&multiStatements=true")
	if err != nil {
		return nil, err
	}
	for _, stmt := range strings.Split(schema, ";") {
		if s := strings.TrimSpace(stmt); s != "" {
			if _, err := d.Exec(s); err != nil {
				return nil, fmt.Errorf("schema: %w\n%s", err, s)
			}
		}
	}
	return d, nil
}
