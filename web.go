package main

import (
	"database/sql"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
)

type PrintFunc func(format string, a ...interface{}) (n int, err error)

func makePrintFunc(w io.Writer) func(format string, a ...interface{}) (n int, err error) {
	// Return closure enclosing io.Writer.
	return func(format string, a ...interface{}) (n int, err error) {
		return fmt.Fprintf(w, format, a...)
	}
}

func qunescape(s string) string {
	us, err := url.QueryUnescape(s)
	if err != nil {
		us = s
	}
	return us
}
func qescape(s string) string {
	return url.QueryEscape(s)
}
func pathescape(s string) string {
	return url.PathEscape(s)
}
func pathunescape(s string) string {
	us, err := url.PathUnescape(s)
	if err != nil {
		us = s
	}
	return us
}
func escape(s string) string {
	return html.EscapeString(s)
}
func unescape(s string) string {
	return html.UnescapeString(s)
}

func handleErr(w http.ResponseWriter, err error, sfunc string) {
	log.Printf("%s: server error (%s)\n", sfunc, err)
	http.Error(w, fmt.Sprintf("%s", err), 500)
}
func handleDbErr(w http.ResponseWriter, err error, sfunc string) bool {
	if err == sql.ErrNoRows {
		http.Error(w, "Not found.", 404)
		return true
	}
	if err != nil {
		log.Printf("%s: database error (%s)\n", sfunc, err)
		http.Error(w, "Server database error.", 500)
		return true
	}
	return false
}
func handleTxErr(tx *sql.Tx, err error) bool {
	if err != nil {
		tx.Rollback()
		return true
	}
	return false
}
func logErr(sfunc string, err error) {
	log.Printf("%s error (%s)\n", sfunc, err)
}

//*** HTML template functions ***
func html_print_open(P PrintFunc, title string) {
	P("<!DOCTYPE html>\n")
	P("<html>\n")
	P("<head>\n")
	P("<meta charset=\"utf-8\">\n")
	P("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	P("<title>%s</title>\n", title)
	P("<link rel=\"stylesheet\" type=\"text/css\" href=\"/static/style.css\">\n")
	P("</head>\n")
	P("<body>\n")
}
func html_print_close(P PrintFunc) {
	P("</body>\n")
	P("</html>\n")
}

//*** Cookie functions ***
func setCookie(w http.ResponseWriter, name, val string) {
	c := http.Cookie{
		Name:  name,
		Value: val,
		Path:  "/",
		// Expires: time.Now().Add(24 * time.Hour),
	}
	http.SetCookie(w, &c)
}
func delCookie(w http.ResponseWriter, name string) {
	c := http.Cookie{
		Name:   name,
		Value:  "",
		Path:   "/",
		MaxAge: 0,
	}
	http.SetCookie(w, &c)
}
func readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
