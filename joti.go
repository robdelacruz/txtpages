package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

type StockPage struct {
	url   string
	title string
	html  string
	desc  string
}

const STOCK_PAGES_DIR = "stock"

var stock_pages []StockPage

var logprint LogPrintfFunc
var logerr LogErrFunc

const JOTI_SLOGAN = "Joti - Fast text web pages"
const JOTI_AUTHOR = "joti"

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
			logerr("create_tables", err)
			os.Exit(1)
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

	l, err := create_logger_from_file("log.txt")
	if err != nil {
		logerr("create_logger_from_file", err)
		panic(err)
	}
	logprint = make_log_print_func(l)
	logerr = make_log_err_func(l)

	stock_pages = load_stock_pages()

	// Check and delete old pages every 24 hours
	const TICKER_DURATION = 24 * time.Hour

	// Delete pages with lastreaddt older than 6 months
	CLEAR_OLD_PAGES_DURATION := days_to_duration(30) * 6

	ticker := time.NewTicker(TICKER_DURATION)
	defer ticker.Stop()
	go func() {
		for {
			<-ticker.C
			delete_jotipages_before_duration(db, CLEAR_OLD_PAGES_DURATION)
		}
	}()

	rand.Seed(time.Now().UnixNano())
	server := Server{db, &cfg}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/", server.index_handler)

	fmt.Printf("Listening on %s...\n", cfg.port)
	err = http.ListenAndServe(fmt.Sprintf(":%s", cfg.port), nil)
	log.Fatal(err)
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

func load_stock_pages() []StockPage {
	pp := []StockPage{}

	ee, err := os.ReadDir(STOCK_PAGES_DIR)
	if err != nil {
		panic(err)
	}
	for _, e := range ee {
		if e.IsDir() {
			continue
		}

		filename := e.Name()

		bs, err := os.ReadFile(fmt.Sprintf("%s/%s", STOCK_PAGES_DIR, filename))
		if err != nil {
			panic(err)
		}

		// url is filename minux the extension
		url := strings.TrimSuffix(filename, filepath.Ext(filename))

		// title is first line matching the following:
		// # Title is followed by one or more '#'
		re := regexp.MustCompile("(?m)^#+\\s+(.*)$")
		ss := re.FindStringSubmatch(string(bs))
		title := url
		if len(ss) > 1 {
			title = ss[1]
		}

		desc := content_to_desc(string(bs))

		// convert stock page markdown to html
		html, err := md_to_html(nil, bs)
		if err != nil {
			panic(err)
		}

		sp := StockPage{}
		sp.url = url
		sp.title = title
		sp.html = html
		sp.desc = desc

		pp = append(pp, sp)
	}

	return pp
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

func (server *Server) page_handler(w http.ResponseWriter, r *http.Request, joti_url string) {
	var z Z
	var jp JotiPage

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	// Show stock page if exists.
	sp := match_stock_page(joti_url, stock_pages)
	if sp != nil {
		print_stock_page(P, sp)
		return
	}

	z = find_jotipage_by_url(server.db, joti_url, &jp)
	if z == Z_NOT_FOUND {
		html_print_open(P, "Not Found", "", "")
		P("<p>Page not found</p>\n")
		html_print_close(P)
		return
	}
	if z != Z_OK {
		html_print_open(P, "Error", "", "")
		P("<p>Error retrieving joti page:</p>\n")
		P("<p>%s</p>\n", z.Error())
		html_print_close(P)
		return
	}
	touch_jotipage_by_url(server.db, jp.url)
	print_joti_page(P, &jp)
}

func match_stock_page(url string, ss []StockPage) *StockPage {
	for _, sp := range ss {
		if url == sp.url {
			return &sp
		}
	}
	return nil
}

func (server *Server) new_handler(w http.ResponseWriter, r *http.Request) {
	var z Z
	var jp JotiPage
	var fvalidate bool

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	if r.Method == "POST" {
		jp.title = strings.TrimSpace(r.FormValue("title"))
		jp.content = strings.TrimSpace(r.FormValue("content"))
		jp.url = sanitize_jotipage_url(strings.TrimSpace(r.FormValue("url")))
		jp.editcode = strings.TrimSpace(r.FormValue("editcode"))

		for {
			if jp.title == "" || jp.content == "" {
				fvalidate = true
				break
			}
			z = create_jotipage(server.db, &jp)
			if z != Z_OK {
				fvalidate = true
				break
			}
			print_save_page_success(P, &jp, r)
			return
		}
	}

	print_create_page_form(P, &jp, r.URL.Path, fvalidate, z)
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
		html_print_open(P, "Not Found", "", "")
		P("<p>Page not found</p>\n")
		html_print_close(P)
		return
	}
	if z != Z_OK {
		html_print_open(P, "Error", "", "")
		P("<p>Error retrieving joti page:</p>\n")
		P("<p>%s</p>\n", z.Error())
		html_print_close(P)
		return
	}

	if r.Method == "POST" {
		jp.title = strings.TrimSpace(r.FormValue("title"))
		jp.content = strings.TrimSpace(r.FormValue("content"))
		jp.url = sanitize_jotipage_url(strings.TrimSpace(r.FormValue("url")))
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

	print_edit_page_form(P, &jp, r.URL.Path, fvalidate, z, editcode)
}

// print_titlebar(P, "header", "/", "home", "/", "about")
func print_titlebar(P PrintFunc, classname string, ll ...string) {
	// Must pass an even number of ll args (link/target pairs)
	if len(ll)%2 > 0 {
		return
	}
	P("<div class=\"titlebar %s\">\n", classname)
	for i := 0; i < len(ll); i += 2 {
		P("    <p><a href=\"%s\">%s</a>", ll[i], ll[i+1])
	}
	P("</div>\n")
}
func print_joti_header(P PrintFunc) {
	P("<div class=\"titlebar header\">\n")
	P("    <p><a href=\"/\">Joti</a> - Fast text web pages</p>\n")
	P("    <p><a href=\"/about\">About</a></p>\n")
	P("    <p><a href=\"/howto\">How to use</a></p>\n")
	P("</div>\n")
}
func print_joti_footer(P PrintFunc) {
	P("<div class=\"titlebar footer\">\n")
	P("    <p><a href=\"/\">Joti</a> - Fast text web pages</p>\n")
	P("    <p><a href=\"/about\">About</a></p>\n")
	P("    <p><a href=\"/howto\">How to use</a></p>\n")
	P("</div>\n")
}
func print_jotipage_header(P PrintFunc, title string, url string) {
	P("<div class=\"titlebar header\">\n")
	P("    <h1>%s</h1>\n", title)
	P("    <p><a href=\"/%s/edit\">Edit</a></p>\n", url)
	P("</div>\n")
}

func print_stock_page(P PrintFunc, sp *StockPage) {
	html_print_open(P, sp.title, sp.desc, JOTI_AUTHOR)
	P("%s\n", sp.html)
	print_joti_footer(P)
	html_print_close(P)
}

func print_joti_page(P PrintFunc, jp *JotiPage) {
	desc := jp.desc
	if desc == "" {
		desc = content_to_desc(jp.content)
	}
	html_print_open(P, jp.title, desc, jp.author)
	html_str, err := md_to_html(nil, []byte(jp.content))
	if err != nil {
		P("<p>Error converting joti page:</p>\n")
		P("<p>%s</p>\n", err.Error())
		html_print_close(P)
		return
	}
	print_jotipage_header(P, jp.title, jp.url)
	P("%s\n", html_str)
	print_joti_footer(P)
	html_print_close(P)
}

func print_create_page_form(P PrintFunc, jp *JotiPage, actionpath string, fvalidate bool, zresult Z) {
	var errmsg string

	if fvalidate {
		if zresult != Z_OK {
			errmsg = zresult.Error()
		}
	}

	html_print_open(P, "Create joti page", JOTI_SLOGAN, JOTI_AUTHOR)
	print_joti_header(P)
	P("<h2>Create a joti webpage</h2>\n")
	P("<form class=\"jotiform\" method=\"post\" action=\"%s\">\n", actionpath)
	if errmsg != "" {
		P("    <div class=\"jotiform_error\">\n")
		P("        <p>%s</p>\n", errmsg)
		P("    </div>\n")
	}
	P("    <div>\n")
	if fvalidate && jp.title == "" {
		P("        <label for=\"title\">Please enter a Title</label>\n")
		P("        <input id=\"title\" class=\"highlight\" autofocus name=\"title\" value=\"%s\">\n", escape(jp.title))
	} else {
		P("        <label for=\"title\">Title</label>\n")
		P("        <input id=\"title\" name=\"title\" value=\"%s\">\n", escape(jp.title))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && jp.content == "" {
		P("        <label for=\"content\">Please enter Content</label>\n")
		P("        <textarea id=\"content\" class=\"highlight\" autofocus name=\"content\">%s</textarea>\n", escape(jp.content))
	} else {
		P("        <label for=\"content\">Content</label>\n")
		P("        <textarea id=\"content\" name=\"content\">%s</textarea>\n", escape(jp.content))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && zresult == Z_URL_EXISTS {
		P("        <label for=\"url\">URL already exists, enter another one</label>\n")
		P("        <input id=\"url\" class=\"highlight\" name=\"url\" autofocus value=\"%s\">\n", escape(jp.url))
	} else {
		P("        <label for=\"url\">Custom URL (optional)</label>\n")
		P("        <input id=\"url\" name=\"url\" value=\"%s\">\n", escape(jp.url))
	}
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"editcode\">Custom edit code (optional)</label>\n")
	P("        <input id=\"editcode\" name=\"editcode\" value=\"%s\">\n", escape(jp.editcode))
	P("    </div>\n")
	P("    <div class=\"jotiform_save\">\n")
	P("        <button type=\"submit\">Create Page</button>\n")
	P("    </div>\n")
	P("</form>\n")
	html_print_close(P)
}

func print_edit_page_form(P PrintFunc, jp *JotiPage, actionpath string, fvalidate bool, zresult Z, editcode string) {
	var errmsg string

	if fvalidate {
		if zresult != Z_OK {
			errmsg = zresult.Error()
		}
	}

	html_print_open(P, "Edit page", JOTI_SLOGAN, JOTI_AUTHOR)
	print_joti_header(P)
	P("<h2>Edit joti webpage</h2>\n")
	P("<form class=\"jotiform\" method=\"post\" action=\"%s\">\n", actionpath)
	if errmsg != "" {
		P("    <div class=\"jotiform_error\">\n")
		P("        <p>%s</p>\n", errmsg)
		P("    </div>\n")
	}
	P("    <div>\n")
	if fvalidate && jp.title == "" {
		P("        <label for=\"title\">Please enter a Title</label>\n")
		P("        <input id=\"title\" class=\"highlight\" autofocus name=\"title\" value=\"%s\">\n", escape(jp.title))
	} else {
		P("        <label for=\"title\">Title</label>\n")
		P("        <input id=\"title\" name=\"title\" value=\"%s\">\n", escape(jp.title))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && jp.content == "" {
		P("        <label for=\"content\">Please enter Content</label>\n")
		P("        <textarea id=\"content\" class=\"highlight\" autofocus name=\"content\">%s</textarea>\n", escape(jp.content))
	} else {
		P("        <label for=\"content\">Content</label>\n")
		P("        <textarea id=\"content\" name=\"content\">%s</textarea>\n", escape(jp.content))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && zresult == Z_URL_EXISTS {
		P("        <label for=\"url\">URL already exists, enter another one</label>\n")
		P("        <input id=\"url\" class=\"highlight\" name=\"url\" autofocus value=\"%s\">\n", escape(jp.url))
	} else {
		P("        <label for=\"url\">Custom URL</label>\n")
		P("        <input id=\"url\" name=\"url\" value=\"%s\">\n", escape(jp.url))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && editcode != jp.editcode {
		P("        <label for=\"editcode\">Incorrect edit code, please re-enter</label>\n")
		P("        <input id=\"editcode\" class=\"highlight\" autofocus name=\"editcode\" value=\"%s\">\n", escape(editcode))
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

func print_save_page_success(P PrintFunc, jp *JotiPage, r *http.Request) {
	href_link := fmt.Sprintf("/%s", jp.url)
	edit_href_link := fmt.Sprintf("/%s/edit", jp.url)

	page_name := fmt.Sprintf("%s/%s", r.Host, jp.url)
	edit_page_name := fmt.Sprintf("%s/%s/edit", r.Host, jp.url)

	html_print_open(P, "Success", "", "")
	P("<h2>You made a page.</h2>\n")
	P("<p>The link to your page is here:</p>\n")
	P("<p><a href=\"%s\">%s</a></p>", href_link, page_name)
	P("<p>Edit your page here:</p>\n")
	P("<p><a href=\"%s\">%s</a></p>", edit_href_link, edit_page_name)
	P("<p>You will need this code to make changes to this page in the future:</p>\n")
	P("<p>Your edit code: <b>%s</b></p>\n", jp.editcode)
	P("<p>You must keep this info safe (and bookmarking this page won't work). It cannot be accessed again!</p>\n")
	print_joti_footer(P)
	html_print_close(P)
}
