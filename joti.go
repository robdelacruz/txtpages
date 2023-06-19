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
	var z Z
	var p JotiPage
	var fvalidate bool

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	if r.Method == "POST" {
		p.title = strings.TrimSpace(r.FormValue("title"))
		p.content = strings.TrimSpace(r.FormValue("content"))
		p.url = strings.TrimSpace(r.FormValue("url"))
		p.editcode = strings.TrimSpace(r.FormValue("editcode"))

		for {
			if p.title == "" || p.content == "" {
				fvalidate = true
				break
			}
			z = create_jotipage(server.db, &p)
			if z != Z_OK {
				fvalidate = true
				break
			}
			print_save_page_success(P, &p, r)
			return
		}
	}

	print_create_page_form(r, P, &p, fvalidate, z)
}

func print_create_page_form(r *http.Request, P PrintFunc, p *JotiPage, fvalidate bool, zresult Z) {
	var errmsg string

	if fvalidate {
		if zresult != Z_OK {
			errmsg = zresult.Error()
		}
	}

	html_print_open(P, "Create a page")
	P("<h1><a href=\"/\">joti</a></h1>\n")
	P("<p>Simple text web pages</p>\n")
	P("<p>\n")
	P("    <a href=\"/\">What is joti?</a><br>\n")
	P("    <a href=\"/\">How to use joti?</a>\n")
	P("</p>\n")
	P("<h2>Create a joti webpage</h2>\n")
	P("<form class=\"jotiform\" method=\"post\" action=\"%s\">\n", r.URL.Path)
	if errmsg != "" {
		P("    <div class=\"jotiform_error\">\n")
		P("        <p>%s</p>\n", errmsg)
		P("    </div>\n")
	}
	P("    <div>\n")
	if fvalidate && p.title == "" {
		P("        <label for=\"title\">Please enter a Title</label>\n")
		P("        <input id=\"title\" class=\"highlight\" name=\"title\" value=\"%s\">\n", escape(p.title))
	} else {
		P("        <label for=\"title\">Title</label>\n")
		P("        <input id=\"title\" name=\"title\" value=\"%s\">\n", escape(p.title))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && p.content == "" {
		P("        <label for=\"content\">Please enter Content</label>\n")
		P("        <textarea id=\"content\" class=\"highlight\" name=\"content\">%s</textarea>\n", escape(p.content))
	} else {
		P("        <label for=\"content\">Content</label>\n")
		P("        <textarea id=\"content\" name=\"content\">%s</textarea>\n", escape(p.content))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && zresult == Z_URL_EXISTS {
		P("        <label for=\"url\">URL already exists, enter another one</label>\n")
		P("        <input id=\"url\" class=\"highlight\" name=\"url\" value=\"%s\">\n", escape(p.url))
	} else {
		P("        <label for=\"url\">Custom URL (optional)</label>\n")
		P("        <input id=\"url\" name=\"url\" value=\"%s\">\n", escape(p.url))
	}
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"editcode\">Custom edit code (optional)</label>\n")
	P("        <input id=\"editcode\" name=\"editcode\" value=\"%s\">\n", escape(p.editcode))
	P("    </div>\n")
	P("    <div class=\"jotiform_save\">\n")
	P("        <button type=\"submit\">Create Page</button>\n")
	P("    </div>\n")
	P("</form>\n")
	html_print_close(P)
}

func print_save_page_success(P PrintFunc, p *JotiPage, r *http.Request) {
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

func (server *Server) edit_handler(w http.ResponseWriter, r *http.Request, joti_url string) {
	var z Z
	var jp JotiPage
	var editcode string
	var fvalidate bool

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	z = find_jotipage_by_url(server.db, joti_url, &jp)
	if z == Z_NOT_FOUND {
		html_print_open(P, "Not Found")
		P("<p>Page not found</p>\n")
		html_print_close(P)
		return
	}
	if z != Z_OK {
		html_print_open(P, "Error")
		P("<p>Error retrieving joti page:</p>\n")
		P("<p>%s</p>\n", z.Error())
		html_print_close(P)
		return
	}

	if r.Method == "POST" {
		jp.title = strings.TrimSpace(r.FormValue("title"))
		jp.content = strings.TrimSpace(r.FormValue("content"))
		jp.url = strings.TrimSpace(r.FormValue("url"))
		editcode = strings.TrimSpace(r.FormValue("editcode"))

		for {
			if jp.title == "" || jp.content == "" || editcode != jp.editcode {
				fvalidate = true
				break
			}
			z = edit_jotipage(server.db, &jp, editcode)
			if z != Z_OK {
				fvalidate = true
				break
			}
			print_save_page_success(P, &jp, r)
			return
		}
	}

	print_edit_page_form(r, P, &jp, editcode, fvalidate, Z_OK)
}

func print_edit_page_form(r *http.Request, P PrintFunc, p *JotiPage, editcode string, fvalidate bool, zresult Z) {
	var errmsg string

	if fvalidate {
		if zresult != Z_OK {
			errmsg = zresult.Error()
		}
	}

	html_print_open(P, "Edit page")
	P("<h2>Edit joti webpage</h2>\n")
	P("<form class=\"jotiform\" method=\"post\" action=\"%s\">\n", r.URL.Path)
	if errmsg != "" {
		P("    <div class=\"jotiform_error\">\n")
		P("        <p>%s</p>\n", errmsg)
		P("    </div>\n")
	}
	P("    <div>\n")
	if fvalidate && p.title == "" {
		P("        <label for=\"title\">Please enter a Title</label>\n")
		P("        <input id=\"title\" class=\"highlight\" name=\"title\" value=\"%s\">\n", escape(p.title))
	} else {
		P("        <label for=\"title\">Title</label>\n")
		P("        <input id=\"title\" name=\"title\" value=\"%s\">\n", escape(p.title))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && p.content == "" {
		P("        <label for=\"content\">Please enter Content</label>\n")
		P("        <textarea id=\"content\" class=\"highlight\" name=\"content\">%s</textarea>\n", escape(p.content))
	} else {
		P("        <label for=\"content\">Content</label>\n")
		P("        <textarea id=\"content\" name=\"content\">%s</textarea>\n", escape(p.content))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && zresult == Z_URL_EXISTS {
		P("        <label for=\"url\">URL already exists, enter another one</label>\n")
		P("        <input id=\"url\" class=\"highlight\" name=\"url\" value=\"%s\">\n", escape(p.url))
	} else {
		P("        <label for=\"url\">Custom URL</label>\n")
		P("        <input id=\"url\" name=\"url\" value=\"%s\">\n", escape(p.url))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && editcode != p.editcode {
		P("        <label for=\"editcode\">Incorrect edit code, please re-enter</label>\n")
		P("        <input id=\"editcode\" class=\"highlight\" name=\"editcode\" value=\"%s\">\n", escape(editcode))
	} else {
		P("        <label for=\"editcode\">Enter edit code</label>\n")
		P("        <input id=\"editcode\" name=\"editcode\" value=\"%s\">\n", escape(editcode))
	}
	P("    </div>\n")
	P("    <div class=\"jotiform_save\">\n")
	P("        <button type=\"submit\">Save Page</button>\n")
	P("    </div>\n")
	P("</form>\n")
	html_print_close(P)
}

func (server *Server) page_handler(w http.ResponseWriter, r *http.Request, joti_url string) {
	var z Z
	var jp JotiPage

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	z = find_jotipage_by_url(server.db, joti_url, &jp)
	if z == Z_NOT_FOUND {
		html_print_open(P, "Not Found")
		P("<p>Page not found</p>\n")
		html_print_close(P)
		return
	}
	if z != Z_OK {
		html_print_open(P, "Error")
		P("<p>Error retrieving joti page:</p>\n")
		P("<p>%s</p>\n", z.Error())
		html_print_close(P)
		return
	}
	print_joti_page(P, &jp)
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

func parse_args(args []string, cfg *Config) {
	const (
		PA_NONE = iota
		PA_INITDBFILE
	)

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
