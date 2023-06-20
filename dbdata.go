package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
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
	Z_WRONG_EDITCODE
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
	} else if z == Z_WRONG_EDITCODE {
		return "Incorrect edit code"
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

func find_jotipage_by_id(db *sql.DB, id int64, jp *JotiPage) Z {
	s := "SELECT jotipage_id, title, url, content, editcode, createdt, lastreaddt FROM jotipage WHERE jotipage_id = ?"
	row := db.QueryRow(s, id)
	err := row.Scan(&jp.jotipage_id, &jp.title, &jp.url, &jp.content, &jp.editcode, &jp.createdt, &jp.lastreaddt)
	if err == sql.ErrNoRows {
		return Z_NOT_FOUND
	}
	if err != nil {
		panic(err)
		return Z_DBERR
	}
	return Z_OK
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

func random_editcode() string {
	return edit_words[rand.Intn(len(edit_words))]
}

func create_jotipage(db *sql.DB, p *JotiPage) Z {
	if p.url != "" && jotipage_url_exists(db, p.url, 0) {
		return Z_URL_EXISTS
	}
	if p.createdt == "" {
		p.createdt = nowdate()
	}
	if p.lastreaddt == "" {
		p.lastreaddt = p.createdt
	}
	if p.editcode == "" {
		p.editcode = random_editcode()
	}

	var s string
	var result sql.Result
	var err error

	if p.url == "" {
		// Generate unique url if no url specified.
		s = "INSERT INTO jotipage (title, content, editcode, createdt, lastreaddt, url) VALUES (?, ?, ?, ?, ?, ? || (SELECT IFNULL(MAX(jotipage_id), 0)+1 FROM jotipage))"
		result, err = sqlexec(db, s, p.title, p.content, p.editcode, p.createdt, p.lastreaddt, base_url_from_title(p.title))
	} else {
		s = "INSERT INTO jotipage (title, content, editcode, createdt, lastreaddt, url) VALUES (?, ?, ?, ?, ?, ?)"
		result, err = sqlexec(db, s, p.title, p.content, p.editcode, p.createdt, p.lastreaddt, p.url)
	}
	if err != nil {
		panic(err)
		return Z_DBERR
	}
	id, err := result.LastInsertId()
	if err != nil {
		panic(err)
		return Z_DBERR
	}
	p.jotipage_id = id

	// If url was autogen, load the page we just created to retrieve the url.
	if p.url == "" {
		z := find_jotipage_by_id(db, id, p)
		if z != Z_OK {
			return z
		}
	}
	return Z_OK
}

func edit_jotipage(db *sql.DB, p *JotiPage, editcode string) Z {
	if editcode != p.editcode {
		return Z_WRONG_EDITCODE
	}
	if p.url != "" && jotipage_url_exists(db, p.url, p.jotipage_id) {
		return Z_URL_EXISTS
	}
	if p.createdt == "" {
		p.createdt = nowdate()
	}
	p.lastreaddt = nowdate()
	if p.editcode == "" {
		p.editcode = random_editcode()
	}
	if p.url == "" {
		// Generate unique url if no url specified.
		p.url = fmt.Sprintf("%s%d", base_url_from_title(p.title), p.jotipage_id)
	}

	s := "UPDATE jotipage SET title = ?, content = ?, editcode = ?, lastreaddt = ?, url = ? WHERE jotipage_id = ?"
	_, err := sqlexec(db, s, p.title, p.content, p.editcode, p.lastreaddt, p.url, p.jotipage_id)
	if err != nil {
		return Z_DBERR
	}
	return Z_OK
}

func touch_jotipage_by_url(db *sql.DB, url string) Z {
	s := "UPDATE jotipage SET lastreaddt = ? WHERE url = ?"
	_, err := sqlexec(db, s, nowdate(), url)
	if err != nil {
		return Z_DBERR
	}
	return Z_OK
}

func base_url_from_title(title string) string {
	url := strings.TrimSpace(strings.ToLower(title))

	// Replace whitespace with underscore
	re := regexp.MustCompile(`\s`)
	url = re.ReplaceAllString(url, "_")

	// Remove all chars not matching A-Za-z0-9_
	re = regexp.MustCompile(`[^\w]`)
	url = re.ReplaceAllString(url, "")

	return url
}

// Return true if url exists in a previous jotipage row.
// Exclude row containing exclude_jotipage_id in the check.
func jotipage_url_exists(db *sql.DB, url string, exclude_jotipage_id int64) bool {
	s := "SELECT jotipage_id FROM jotipage WHERE url = ? AND jotipage_id <> ?"
	row := db.QueryRow(s, url, exclude_jotipage_id)
	var tmpid int64
	err := row.Scan(&tmpid)
	if err == sql.ErrNoRows {
		return false
	}
	return true
}

// Delete jotipages with lastreaddt before specified duration
// Ex.
// Delete with lastreaddt older than 60 seconds
// delete_jotipages_before_duration(60 * time.Second)
//
// Delete with lastreaddt older than 60 days
// delete_jotipages_before_duration(60 * time.Hour * 24)
func delete_jotipages_before_duration(db *sql.DB, d time.Duration) Z {
	var err error
	cutoffdt := isodate(time.Now().Add(-d))
	fmt.Printf("Deleting jotipages older than %s\n", cutoffdt)

	s1 := "SELECT jotipage_id, title, lastreaddt FROM jotipage WHERE lastreaddt < ?"
	rows, err := db.Query(s1, cutoffdt)
	if err != nil {
		panic(err)
		return Z_DBERR
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var title, lastreaddt string
		rows.Scan(&id, &title, &lastreaddt)
		fmt.Printf("***  %s %d %s\n", lastreaddt, id, title)
	}

	s := "DELETE FROM jotipage WHERE lastreaddt < ?"
	_, err = sqlexec(db, s, cutoffdt)
	if err != nil {
		return Z_DBERR
	}
	return Z_OK
}
