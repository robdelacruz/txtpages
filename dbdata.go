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

func create_jotipage(db *sql.DB, p *JotiPage) (int64, error) {
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
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func find_jotipage_by_url(db *sql.DB, url string) (*JotiPage, error) {
	s := "SELECT jotipage_id, title, url, content, editcode, createdt, lastreaddt FROM jotipage WHERE url = ?"
	row := db.QueryRow(s, url)
	var jp JotiPage
	err := row.Scan(&jp.jotipage_id, &jp.title, &jp.url, &jp.content, &jp.editcode, &jp.createdt, &jp.lastreaddt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &jp, nil
}
