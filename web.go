package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

type HtmlMeta struct {
	desc     string
	keywords string
	author   string
}

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

// *** HTML template functions ***
func html_print_open(P PrintFunc, title, desc, author string) {
	P("<!DOCTYPE html>\n")
	P("<html>\n")
	P("<head>\n")
	P("<meta charset=\"utf-8\">\n")
	if desc != "" {
		P("<meta name=\"description\" content=\"%s\">\n", escape(desc))
	}
	P("<meta name=\"keywords\" content=\"%s\">\n", "")
	if author != "" {
		P("<meta name=\"author\" content=\"%s\">\n", escape(author))
	}
	P("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	P("<title>%s</title>\n", title)
	P("<link rel=\"icon\" href=\"/static/news-paper.svg\">\n")
	P("<link rel=\"stylesheet\" type=\"text/css\" href=\"/static/style.css\">\n")
	P("</head>\n")
	P("<body>\n")
}
func html_print_close(P PrintFunc) {
	P("</body>\n")
	P("</html>\n")
}

// *** Cookie functions ***
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

func create_goldmark_interface() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(),
	)
}

func md_to_html(gmd goldmark.Markdown, markdown_bytes []byte) (string, error) {
	if gmd == nil {
		gmd = create_goldmark_interface()
	}
	var buf bytes.Buffer
	err := gmd.Convert(markdown_bytes, &buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
