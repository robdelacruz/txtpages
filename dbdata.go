package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type JotiPage struct {
	jotipage_id int64
	title       string
	url         string
	content     string
	editcode    string
	createdt    string
	lastreaddt  string
}

type Z int

const (
	Z_OK Z = iota
	Z_DBERR
	Z_URL_EXISTS
	Z_NOT_FOUND
	Z_NO_URL
)

func (z Z) Error() string {
	if z == Z_OK {
		return "OK"
	} else if z == Z_DBERR {
		return "Internal Database error"
	} else if z == Z_URL_EXISTS {
		return "URL exists"
	} else if z == Z_NOT_FOUND {
		return "Not found"
	} else if z == Z_NO_URL {
		return "No URL specified"
	}
	return "Unknown error"
}

func create_tables(dbfile string) error {
	if file_exists(dbfile) {
		return fmt.Errorf("File '%s' exists", dbfile)
	}

	db, err := sql.Open("sqlite3", dbfile)
	if err != nil {
		return err
	}

	ss := []string{
		"CREATE TABLE jotipage (jotipage_id INTEGER PRIMARY KEY NOT NULL, title TEXT NOT NULL DEFAULT '', url TEXT UNIQUE NOT NULL, content TEXT NOT NULL DEFAULT '', editcode TEXT NOT NULL DEFAULT '', createdt TEXT NOT NULL, lastreaddt TEXT NOT NULL);",
		`INSERT INTO jotipage (jotipage_id, title, url, content, editcode, createdt, lastreaddt) VALUES(1, "First Post!", "firstpost", "This is the first post.", "password", strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));`,
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, s := range ss {
		_, err := txexec(tx, s)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func random_editcode() string {
	return edit_words[rand.Intn(len(edit_words))]
}

func create_jotipage(db *sql.DB, p *JotiPage) Z {
	if p.url == "" {
		return Z_NO_URL
	}
	if jotipage_url_exists(db, p.url) {
		return Z_URL_EXISTS
	}

	if p.createdt == "" {
		p.createdt = time.Now().Format(time.RFC3339)
	}
	if p.lastreaddt == "" {
		p.lastreaddt = p.createdt
	}
	if p.editcode == "" {
		p.editcode = random_editcode()
	}
	s := "INSERT INTO jotipage (title, url, content, editcode, createdt, lastreaddt) VALUES (?, ?, ?, ?, ?, ?)"
	result, err := sqlexec(db, s, p.title, p.url, p.content, p.editcode, p.createdt, p.lastreaddt)
	if err != nil {
		return Z_DBERR
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Z_DBERR
	}
	p.jotipage_id = id
	return Z_OK
}

func jotipage_url_exists(db *sql.DB, url string) bool {
	var jp JotiPage

	z := find_jotipage_by_url(db, url, &jp)
	if z == Z_OK {
		return true
	}
	return false
}

func find_jotipage_by_url(db *sql.DB, url string, jp *JotiPage) Z {
	s := "SELECT jotipage_id, title, url, content, editcode, createdt, lastreaddt FROM jotipage WHERE url = ?"
	row := db.QueryRow(s, url)
	err := row.Scan(&jp.jotipage_id, &jp.title, &jp.url, &jp.content, &jp.editcode, &jp.createdt, &jp.lastreaddt)
	if err == sql.ErrNoRows {
		return Z_NOT_FOUND
	}
	if err != nil {
		return Z_DBERR
	}
	return Z_OK
}
