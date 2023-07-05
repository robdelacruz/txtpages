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

type TxtPage struct {
	txtpage_id int64
	title      string
	url        string
	content    string
	desc       string
	author     string
	passcode   string
	createdt   string
	lastreaddt string
}

type TxtPages []*TxtPage

type Z int

const (
	Z_OK Z = iota
	Z_DBERR
	Z_URL_EXISTS
	Z_NOT_FOUND
	Z_WRONG_PASSCODE
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
	} else if z == Z_WRONG_PASSCODE {
		return "Incorrect passcode"
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
		`CREATE TABLE txtpage (
	txtpage_id INTEGER PRIMARY KEY NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	url TEXT UNIQUE NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	desc TEXT NOT NULL DEFAULT '',
	author TEXT NOT NULL DEFAULT '',
	passcode TEXT NOT NULL DEFAULT '',
	createdt TEXT NOT NULL,
	lastreaddt TEXT NOT NULL
);`,
		`INSERT INTO txtpage (
	txtpage_id,
	title,
	url,
	content,
	passcode,
	desc,
	author,
	createdt,
	lastreaddt)
VALUES(
	1,
	"First Post!",
	"firstpost",
	"This is the first post.",
	"",
	"",
	"",
	strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
	strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
);`,
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

func find_txtpage_by_id(db *sql.DB, id int64, tp *TxtPage) Z {
	s := "SELECT txtpage_id, title, url, content, desc, author, passcode, createdt, lastreaddt FROM txtpage WHERE txtpage_id = ?"
	row := db.QueryRow(s, id)
	err := row.Scan(&tp.txtpage_id, &tp.title, &tp.url, &tp.content, &tp.desc, &tp.author, &tp.passcode, &tp.createdt, &tp.lastreaddt)
	if err == sql.ErrNoRows {
		return Z_NOT_FOUND
	}
	if err != nil {
		logerr("find_txtpage_by_id", err)
		return Z_DBERR
	}
	return Z_OK
}
func find_txtpage_by_url(db *sql.DB, url string, tp *TxtPage) Z {
	s := "SELECT txtpage_id, title, url, content, desc, author, passcode, createdt, lastreaddt FROM txtpage WHERE url = ?"
	row := db.QueryRow(s, url)
	err := row.Scan(&tp.txtpage_id, &tp.title, &tp.url, &tp.content, &tp.desc, &tp.author, &tp.passcode, &tp.createdt, &tp.lastreaddt)
	if err == sql.ErrNoRows {
		return Z_NOT_FOUND
	}
	if err != nil {
		logerr("find_txtpage_by_url", err)
		return Z_DBERR
	}
	return Z_OK
}
func find_all_txtpage_orderby_createdt(db *sql.DB) (TxtPages, Z) {
	s := "SELECT txtpage_id, title, url, content, desc, author, passcode, createdt, lastreaddt FROM txtpage ORDER BY createdt DESC"
	rows, err := db.Query(s)
	if err != nil {
		logerr("find_all_txtpage_orderby_createdt", err)
		return nil, Z_DBERR
	}
	defer rows.Close()

	tt := TxtPages{}
	for rows.Next() {
		var tp TxtPage
		err := rows.Scan(&tp.txtpage_id, &tp.title, &tp.url, &tp.content, &tp.desc, &tp.author, &tp.passcode, &tp.createdt, &tp.lastreaddt)
		if err != nil {
			logerr("find_all_txtpage_orderby_createdt", err)
			return nil, Z_DBERR
		}

		tt = append(tt, &tp)
	}
	return tt, Z_OK
}

func random_passcode() string {
	return edit_words[rand.Intn(len(edit_words))]
}

func content_to_desc(content string) string {
	// Use first 200 chars for desc
	desc_len := 200
	content_len := len(content)
	if content_len < desc_len {
		desc_len = content_len
	}
	desc := content[:desc_len]

	// Remove markdown heading ###... chars from desc
	re := regexp.MustCompile("(?m)^#+")
	desc = re.ReplaceAllString(desc, "")

	return desc
}

func create_txtpage(db *sql.DB, tp *TxtPage) Z {
	if tp.url != "" {
		tp.url = sanitize_txtpage_url(tp.url)
	}
	if tp.url != "" && (!is_url_allowed(tp.url) || txtpage_url_exists(db, tp.url, 0)) {
		return Z_URL_EXISTS
	}
	if match_stock_page(tp.url, stock_pages) != nil {
		return Z_URL_EXISTS
	}
	if tp.createdt == "" {
		tp.createdt = nowdate()
	}
	if tp.lastreaddt == "" {
		tp.lastreaddt = tp.createdt
	}
	if tp.passcode == "" {
		tp.passcode = random_passcode()
	}
	tp.content = process_content(tp.content)

	var s string
	var result sql.Result
	var err error

	if tp.url == "" {
		// Generate unique url if no url specified.
		s = "INSERT INTO txtpage (title, content, desc, author, passcode, createdt, lastreaddt, url) VALUES (?, ?, ?, ?, ?, ?, ?, ? || (SELECT IFNULL(MAX(txtpage_id), 0)+1 FROM txtpage))"
		result, err = sqlexec(db, s, tp.title, tp.content, tp.desc, tp.author, tp.passcode, tp.createdt, tp.lastreaddt, sanitize_txtpage_url(tp.title))
	} else {
		s = "INSERT INTO txtpage (title, content, desc, author, passcode, createdt, lastreaddt, url) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
		result, err = sqlexec(db, s, tp.title, tp.content, tp.desc, tp.author, tp.passcode, tp.createdt, tp.lastreaddt, tp.url)
	}
	if err != nil {
		logerr("create_txtpage", err)
		return Z_DBERR
	}
	id, err := result.LastInsertId()
	if err != nil {
		logerr("create_txtpage", err)
		return Z_DBERR
	}
	tp.txtpage_id = id

	// If url was autogen, load the page we just created to retrieve the url.
	if tp.url == "" {
		z := find_txtpage_by_id(db, id, tp)
		if z != Z_OK {
			return z
		}
	}
	return Z_OK
}

func edit_txtpage(db *sql.DB, tp *TxtPage, passcode string) Z {
	if passcode != tp.passcode {
		return Z_WRONG_PASSCODE
	}
	if tp.url != "" {
		tp.url = sanitize_txtpage_url(tp.url)
	}
	if tp.url != "" && (!is_url_allowed(tp.url) || txtpage_url_exists(db, tp.url, tp.txtpage_id)) {
		return Z_URL_EXISTS
	}
	if match_stock_page(tp.url, stock_pages) != nil {
		return Z_URL_EXISTS
	}
	if tp.createdt == "" {
		tp.createdt = nowdate()
	}
	tp.lastreaddt = nowdate()
	if tp.passcode == "" {
		tp.passcode = random_passcode()
	}
	if tp.url == "" {
		tp.url = generate_url(tp)
	}
	tp.content = process_content(tp.content)

	s := "UPDATE txtpage SET title = ?, content = ?, desc = ?, author = ?, passcode = ?, lastreaddt = ?, url = ? WHERE txtpage_id = ?"
	_, err := sqlexec(db, s, tp.title, tp.content, tp.desc, tp.author, tp.passcode, tp.lastreaddt, tp.url, tp.txtpage_id)
	if err != nil {
		logerr("edit_txtpage", err)
		return Z_DBERR
	}
	return Z_OK
}

func touch_txtpage_by_url(db *sql.DB, url string) Z {
	s := "UPDATE txtpage SET lastreaddt = ? WHERE url = ?"
	_, err := sqlexec(db, s, nowdate(), url)
	if err != nil {
		logerr("touch_txtpage_by_url", err)
		return Z_DBERR
	}
	return Z_OK
}

func sanitize_txtpage_url(url string) string {
	url = strings.TrimSpace(strings.ToLower(url))

	// Replace whitespace with "_"
	re := regexp.MustCompile(`\s+`)
	url = re.ReplaceAllString(url, "_")

	// Remove all chars not matching alphanumeric, '_', '-' chars
	re = regexp.MustCompile(`[^\w\-]`)
	url = re.ReplaceAllString(url, "")

	return url
}

// Generate txtpage url: title + txtpage_id
func generate_url(tp *TxtPage) string {
	return fmt.Sprintf("%s%d", sanitize_txtpage_url(tp.title), tp.txtpage_id)
}

func process_content(content string) string {
	// Replace line break with two spaces + line break
	// Markdown will process end of line two spaces as <br>.
	re := regexp.MustCompile("(\\S)\r?\n(\\S)")
	content = re.ReplaceAllString(content, "$1  \n$2")
	return content
}

// Return true if url exists in a previous txtpage row.
// Exclude row containing exclude_txtpage_id in the check.
func txtpage_url_exists(db *sql.DB, url string, exclude_txtpage_id int64) bool {
	s := "SELECT txtpage_id FROM txtpage WHERE url = ? AND txtpage_id <> ?"
	row := db.QueryRow(s, url, exclude_txtpage_id)
	var tmpid int64
	err := row.Scan(&tmpid)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		logerr("txtpage_url_exists", err)
	}
	return true
}

// Delete txtpages with lastreaddt before specified duration
// Ex.
// Delete with lastreaddt older than 60 seconds
// delete_txtpages_before_duration(60 * time.Second)
//
// Delete with lastreaddt older than 60 days
// delete_txtpages_before_duration(60 * time.Hour * 24)
func delete_txtpages_before_duration(db *sql.DB, d time.Duration) Z {
	var err error
	cutoffdt := isodate(time.Now().Add(-d))
	logprint("Deleting txtpages older than %s\n", cutoffdt)

	s1 := "SELECT txtpage_id, title, lastreaddt FROM txtpage WHERE lastreaddt < ?"
	rows, err := db.Query(s1, cutoffdt)
	if err != nil {
		logerr("delete_txtpages_before_duration", err)
		return Z_DBERR
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var title, lastreaddt string
		rows.Scan(&id, &title, &lastreaddt)
		logprint("***  %s %d %s\n", lastreaddt, id, title)
	}

	s := "DELETE FROM txtpage WHERE lastreaddt < ?"
	_, err = sqlexec(db, s, cutoffdt)
	if err != nil {
		logerr("delete_txtpages_before_duration", err)
		return Z_DBERR
	}
	return Z_OK
}
