package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
	initdbfile string
	dbfile     string
	port       string
}

type Server struct {
	db  *sql.DB
	cfg *Config
}

type JotiPage struct {
	jotipage_id int64
	title       string
	url         string
	content     string
	editcode    string
	createdt    string
	lastreaddt  string
}

func main() {
	var err error

	usage := `Usage:
Start webservice:
	%[1]s <dbfile> [port]
Initialize db file:
	%[1]s -i <dbfile>
`
	if len(os.Args) <= 1 {
		fmt.Printf(usage, os.Args[0])
		os.Exit(0)
	}

	var cfg Config
	parse_args(os.Args, &cfg)
	if cfg.initdbfile != "" {
		err = create_tables(cfg.initdbfile)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	if !file_exists(cfg.dbfile) {
		fmt.Printf("dbfile '%s' doesn't exist. Create one with: %s -i <dbfile>\n", cfg.dbfile, os.Args[0])
		os.Exit(1)
	}
	db, err := sql.Open("sqlite3", cfg.dbfile)
	if err != nil {
		fmt.Printf("Error opening '%s' (%s)\n", cfg.dbfile, err)
		os.Exit(1)
	}

	rand.Seed(time.Now().UnixNano())

	server := Server{db, &cfg}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/", server.index_handler)

	fmt.Printf("Listening on %s...\n", cfg.port)
	err = http.ListenAndServe(fmt.Sprintf(":%s", cfg.port), nil)
	log.Fatal(err)
}

func (server *Server) index_handler(w http.ResponseWriter, r *http.Request) {
	var joti_url string
	var action string
	ss := strings.Split(r.URL.Path, "/")
	if len(ss) >= 2 {
		joti_url = ss[1]
	}
	if len(ss) >= 3 {
		action = ss[2]
	}

	if action == "edit" {
		server.edit_handler(w, r, joti_url)
	} else if joti_url != "" {
		server.page_handler(w, r, joti_url)
	} else {
		server.new_handler(w, r)
	}
}

func (server *Server) new_handler(w http.ResponseWriter, r *http.Request) {
	var p JotiPage
	var errmsg string

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	if r.Method == "POST" {
		p.title = strings.TrimSpace(r.FormValue("title"))
		p.content = r.FormValue("content")
		p.url = strings.TrimSpace(r.FormValue("url"))
		p.editcode = strings.TrimSpace(r.FormValue("editcode"))

		for {
			if p.title == "" {
				errmsg = "Please enter a title"
				break
			}
			if p.content == "" {
				errmsg = "Please enter content"
				break
			}
			if p.url == "" {
				errmsg = "Please enter a url"
				break
			}
			newid, err := create_jotipage(server.db, &p)
			if err != nil {
				errmsg = "A server error occured."
				break
			}
			p.jotipage_id = newid
			print_create_page_success(P, &p, r)
			return
		}
	}

	print_joti_form(P, &p, errmsg)
}

func print_joti_form(P PrintFunc, p *JotiPage, errmsg string) {
	html_print_open(P, "Create a page")
	P("<h1><a href=\"/\">joti</a></h1>\n")
	P("<p>Simple text web pages</p>\n")
	P("<p>\n")
	P("    <a href=\"/\">What is joti?</a><br>\n")
	P("    <a href=\"/\">How to use joti?</a>\n")
	P("</p>\n")
	P("<h2>Create a joti webpage</h2>\n")
	P("<form class=\"jotiform\" method=\"post\" action=\"/\">\n")
	if errmsg != "" {
		P("    <div class=\"jotiform_error\">\n")
		P("        <p>%s</p>\n", errmsg)
		P("    </div>\n")
	}
	P("    <div>\n")
	P("        <label for=\"title\">Title</label>\n")
	P("        <input id=\"title\" name=\"title\" value=\"%s\">\n", escape(p.title))
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"content\">Content</label>\n")
	P("        <textarea id=\"content\" name=\"content\">%s</textarea>\n", escape(p.content))
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"url\">Custom URL (optional)</label>\n")
	P("        <input id=\"url\" name=\"url\" value=\"%s\">\n", escape(p.url))
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"editcode\">Custom edit code (optional)</label>\n")
	P("        <input id=\"editcode\" name=\"editcode\" value=\"%s\">\n", escape(p.editcode))
	P("    </div>\n")
	P("    <div class=\"jotiform_save\">\n")
	P("        <button type=\"submit\">Save</button>\n")
	P("    </div>\n")
	P("</form>\n")
	html_print_close(P)
}

func print_create_page_success(P PrintFunc, p *JotiPage, r *http.Request) {
	href_link := fmt.Sprintf("/%s", p.url)
	edit_href_link := fmt.Sprintf("/%s/edit", p.url)

	page_name := fmt.Sprintf("%s/%s", r.Host, p.url)
	edit_page_name := fmt.Sprintf("%s/%s/edit", r.Host, p.url)

	html_print_open(P, "Success")
	P("<h2>You made a page.</h2>\n")
	P("<p>The link to your page is here:</p>\n")
	P("<p><a href=\"%s\">%s</a></p>", href_link, page_name)
	P("<p>Edit your page here:</p>\n")
	P("<p><a href=\"%s\">%s</a></p>", edit_href_link, edit_page_name)
	P("<p>You will need this code to make changes to this page in the future:</p>\n")
	P("<p>Your edit code: <b>%s</b></p>\n", p.editcode)
	P("<p>You must keep this info safe (and bookmarking this page won't work). It cannot be accessed again!</p>\n")
	P("<p><a href=\"/\">joti home</a></p>\n")
	html_print_close(P)
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

func random_editcode() string {
	return edit_words[rand.Intn(len(edit_words))]
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

func (server *Server) edit_handler(w http.ResponseWriter, r *http.Request, joti_url string) {
	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	html_print_open(P, "Edit")
	P("<p>edit</p>\n")
	html_print_close(P)
}

func (server *Server) page_handler(w http.ResponseWriter, r *http.Request, joti_url string) {
	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	jp, err := find_jotipage_by_url(server.db, joti_url)
	if err != nil {
		html_print_open(P, "Error")
		P("<p>Error retrieving joti page:</p>\n")
		P("<p>%s</p>\n", err.Error())
		html_print_close(P)
		return
	}
	if jp == nil {
		html_print_open(P, "Not Found")
		P("<p>Page not found</p>\n")
		html_print_close(P)
		return
	}
	print_joti_page(P, jp)
}

func print_joti_page(P PrintFunc, jp *JotiPage) {
	html_print_open(P, "Joti page")
	html_str, err := md_to_html(nil, []byte(jp.content))
	if err != nil {
		P("<p>Error converting joti page:</p>\n")
		P("<p>%s</p>\n", err.Error())
		html_print_close(P)
		return
	}

	P("%s\n", html_str)
	html_print_close(P)
}

const (
	PA_NONE = iota
	PA_INITDBFILE
)

func parse_args(args []string, cfg *Config) {
	state := PA_NONE
	dbfile_set := false
	port_set := false

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if state == PA_NONE && arg == "-i" {
			state = PA_INITDBFILE
			continue
		}
		if state == PA_INITDBFILE {
			cfg.initdbfile = arg
			state = PA_NONE
			continue
		}
		if state == PA_NONE {
			if !dbfile_set {
				cfg.dbfile = arg
				dbfile_set = true
			} else if !port_set {
				cfg.port = arg
				port_set = true
			}
		}
	}

	if !port_set {
		cfg.port = "8000"
	}
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
		"CREATE TABLE jotipage (jotipage_id INTEGER PRIMARY KEY NOT NULL, title TEXT NOT NULL DEFAULT '', url TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '', editcode TEXT NOT NULL DEFAULT '', createdt TEXT NOT NULL, lastreaddt TEXT NOT NULL);",
		`INSERT INTO jotipage (jotipage_id, title, url, content, editcode, createdt, lastreaddt) VALUES(1, "First Post!", "firstpost", "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.", "password", strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));`,
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
