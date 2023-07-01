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

const TXTPAGES_TITLE = "TxtPages - Create fast text webpages"
const TXTPAGES_NAME = "TxtPages"
const TXTPAGES_SLOGAN = "Create fast text webpages"
const TXTPAGES_AUTHOR = "txtpages"

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
			delete_txtpages_before_duration(db, CLEAR_OLD_PAGES_DURATION)
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
	var url string
	var action string
	ss := strings.Split(r.URL.Path, "/")
	if len(ss) >= 2 {
		url = ss[1]
	}
	if len(ss) >= 3 {
		action = ss[2]
	}

	if action == "edit" {
		server.edit_handler(w, r, url)
	} else if url != "" {
		server.page_handler(w, r, url)
	} else {
		server.new_handler(w, r)
	}
}

func (server *Server) page_handler(w http.ResponseWriter, r *http.Request, url string) {
	var z Z
	var tp TxtPage

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	// Show stock page if exists.
	sp := match_stock_page(url, stock_pages)
	if sp != nil {
		print_stock_page(P, sp)
		return
	}

	z = find_txtpage_by_url(server.db, url, &tp)
	if z == Z_NOT_FOUND {
		html_print_open(P, "Not Found", "", "")
		print_header(P)
		P("<p>Page not found: %s</p>\n", url)
		html_print_close(P)
		return
	}
	if z != Z_OK {
		html_print_open(P, "Error", "", "")
		print_header(P)
		P("<p>Error retrieving txtpage: %s</p>\n", z.Error())
		html_print_close(P)
		return
	}
	touch_txtpage_by_url(server.db, tp.url)
	print_txtpage(P, &tp)
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
	var tp TxtPage
	var fvalidate bool

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	if r.Method == "POST" {
		tp.title = strings.TrimSpace(r.FormValue("title"))
		tp.content = strings.TrimSpace(r.FormValue("content"))
		tp.desc = strings.TrimSpace(r.FormValue("desc"))
		tp.author = strings.TrimSpace(r.FormValue("author"))
		tp.url = strings.TrimSpace(r.FormValue("url"))
		tp.passcode = strings.TrimSpace(r.FormValue("passcode"))

		for {
			if tp.title == "" || tp.content == "" {
				fvalidate = true
				break
			}
			z = create_txtpage(server.db, &tp)
			if z != Z_OK {
				fvalidate = true
				break
			}
			print_save_page_success(P, &tp, r)
			return
		}
	}

	print_create_page_form(P, &tp, r.URL.Path, fvalidate, z)
}

func (server *Server) edit_handler(w http.ResponseWriter, r *http.Request, url string) {
	var z Z
	var tp TxtPage
	var passcode string
	var fvalidate bool

	w.Header().Set("Content-Type", "text/html")
	P := makePrintFunc(w)

	z = find_txtpage_by_url(server.db, url, &tp)
	if z == Z_NOT_FOUND {
		html_print_open(P, "Not Found", "", "")
		print_header(P)
		P("<p>Page not found</p>\n")
		html_print_close(P)
		return
	}
	if z != Z_OK {
		html_print_open(P, "Error", "", "")
		print_header(P)
		P("<p>Error retrieving txtpage: %s</p>\n", z.Error())
		html_print_close(P)
		return
	}

	if r.Method == "POST" {
		tp.title = strings.TrimSpace(r.FormValue("title"))
		tp.content = strings.TrimSpace(r.FormValue("content"))
		tp.desc = strings.TrimSpace(r.FormValue("desc"))
		tp.author = strings.TrimSpace(r.FormValue("author"))
		tp.url = strings.TrimSpace(r.FormValue("url"))
		passcode = strings.TrimSpace(r.FormValue("passcode"))

		for {
			if tp.title == "" || tp.content == "" || passcode != tp.passcode {
				fvalidate = true
				break
			}
			z = edit_txtpage(server.db, &tp, passcode)
			if z != Z_OK {
				fvalidate = true
				break
			}
			print_save_page_success(P, &tp, r)
			return
		}
	}

	print_edit_page_form(P, &tp, r.URL.Path, fvalidate, z, passcode)
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
func print_header(P PrintFunc) {
	P("<div class=\"titlebar header\">\n")
	P("    <p><a href=\"/\">%s</a> - %s</p>\n", TXTPAGES_NAME, TXTPAGES_SLOGAN)
	P("    <p><a href=\"/about\">About</a></p>\n")
	P("    <p><a href=\"/howto\">How to use</a></p>\n")
	P("</div>\n")
}
func print_footer(P PrintFunc) {
	P("<div class=\"titlebar footer\">\n")
	P("    <p><a href=\"/\">%s</a> - %s</p>\n", TXTPAGES_NAME, TXTPAGES_SLOGAN)
	P("    <p><a href=\"/about\">About</a></p>\n")
	P("    <p><a href=\"/howto\">How to use</a></p>\n")
	P("</div>\n")
}
func print_page_header(P PrintFunc, title string, url string) {
	P("<div class=\"titlebar header\">\n")
	P("    <h1>%s</h1>\n", title)
	P("    <p><a href=\"/%s/edit\">Edit</a></p>\n", url)
	P("</div>\n")
}

func print_stock_page(P PrintFunc, sp *StockPage) {
	html_print_open(P, sp.title, sp.desc, TXTPAGES_AUTHOR)
	P("%s\n", sp.html)
	print_footer(P)
	html_print_close(P)
}

func print_txtpage(P PrintFunc, tp *TxtPage) {
	desc := tp.desc
	if desc == "" {
		desc = content_to_desc(tp.content)
	}
	html_print_open(P, tp.title, desc, tp.author)
	html_str, err := md_to_html(nil, []byte(tp.content))
	if err != nil {
		print_header(P)
		P("<p>Error converting txtpage: %s</p>\n", err.Error())
		html_print_close(P)
		return
	}
	print_page_header(P, tp.title, tp.url)
	P("<article class=\"txtpage_content\">\n")
	P("%s\n", html_str)
	P("</article>\n")
	print_footer(P)
	html_print_close(P)
}

func print_create_page_form(P PrintFunc, tp *TxtPage, actionpath string, fvalidate bool, zresult Z) {
	var errmsg string

	if fvalidate {
		if zresult != Z_OK {
			errmsg = zresult.Error()
		}
	}

	html_print_open(P, TXTPAGES_TITLE, TXTPAGES_TITLE, TXTPAGES_AUTHOR)
	print_header(P)
	P("<h2>Create a txtpage</h2>\n")
	P("<form class=\"txtpage_form\" method=\"post\" action=\"%s\">\n", actionpath)
	if errmsg != "" {
		P("    <div class=\"txtpage_form_error\">\n")
		P("        <p>%s</p>\n", errmsg)
		P("    </div>\n")
	}
	P("    <div>\n")
	if fvalidate && tp.title == "" {
		P("        <label for=\"title\">Please enter a Title</label>\n")
		P("        <input id=\"title\" class=\"highlight\" autofocus name=\"title\" value=\"%s\">\n", escape(tp.title))
	} else {
		P("        <label for=\"title\">Title</label>\n")
		P("        <input id=\"title\" name=\"title\" value=\"%s\">\n", escape(tp.title))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && tp.content == "" {
		P("        <label for=\"content\">Please enter Content</label>\n")
		P("        <textarea id=\"content\" name=\"content\" rows=\"20\" class=\"highlight\" autofocus>%s</textarea>\n", escape(tp.content))
	} else {
		P("        <label for=\"content\">Content</label>\n")
		P("        <textarea id=\"content\" name=\"content\" rows=\"20\">%s</textarea>\n", escape(tp.content))
	}
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"desc\">Description <i>(optional)</i></label>\n")
	P("        <textarea id=\"desc\" name=\"desc\" rows=\"3\">%s</textarea>\n", escape(tp.desc))
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"author\">Author <i>(optional)</i></label>\n")
	P("        <input id=\"author\" name=\"author\" value=\"%s\">\n", escape(tp.author))
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && zresult == Z_URL_EXISTS {
		P("        <label for=\"url\">URL already exists, enter another one</label>\n")
		P("        <input id=\"url\" class=\"highlight\" name=\"url\" autofocus value=\"%s\">\n", escape(tp.url))
	} else {
		P("        <label for=\"url\">Set URL <i>(optional)</i></label>\n")
		P("        <input id=\"url\" name=\"url\" value=\"%s\">\n", escape(tp.url))
	}
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"passcode\">Set passcode <i>(optional)</i></label>\n")
	P("        <input id=\"passcode\" name=\"passcode\" value=\"%s\">\n", escape(tp.passcode))
	P("    </div>\n")
	P("    <div class=\"txtpage_form_save\">\n")
	P("        <button type=\"submit\">Create Page</button>\n")
	P("    </div>\n")
	P("</form>\n")
	html_print_close(P)
}

func print_edit_page_form(P PrintFunc, tp *TxtPage, actionpath string, fvalidate bool, zresult Z, passcode string) {
	var errmsg string

	if fvalidate {
		if zresult != Z_OK {
			errmsg = zresult.Error()
		}
	}

	html_print_open(P, "Edit txtpage", TXTPAGES_TITLE, TXTPAGES_AUTHOR)
	print_header(P)
	P("<h2>Edit txtpage</h2>\n")
	P("<form class=\"txtpage_form\" method=\"post\" action=\"%s\">\n", actionpath)
	if errmsg != "" {
		P("    <div class=\"txtpage_form_error\">\n")
		P("        <p>%s</p>\n", errmsg)
		P("    </div>\n")
	}
	P("    <div>\n")
	if fvalidate && tp.title == "" {
		P("        <label for=\"title\">Please enter a Title</label>\n")
		P("        <input id=\"title\" class=\"highlight\" autofocus name=\"title\" value=\"%s\">\n", escape(tp.title))
	} else {
		P("        <label for=\"title\">Title</label>\n")
		P("        <input id=\"title\" name=\"title\" value=\"%s\">\n", escape(tp.title))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && tp.content == "" {
		P("        <label for=\"content\">Please enter Content</label>\n")
		P("        <textarea id=\"content\" name=\"content\" rows=\"20\" class=\"highlight\" autofocus>%s</textarea>\n", escape(tp.content))
	} else {
		P("        <label for=\"content\">Content</label>\n")
		P("        <textarea id=\"content\" name=\"content\" rows=\"20\">%s</textarea>\n", escape(tp.content))
	}
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"desc\">Description <i>(optional)</i></label>\n")
	P("        <textarea id=\"desc\" name=\"desc\" rows=\"3\">%s</textarea>\n", escape(tp.desc))
	P("    </div>\n")
	P("    <div>\n")
	P("        <label for=\"author\">Author <i>(optional)</i></label>\n")
	P("        <input id=\"author\" name=\"author\" value=\"%s\">\n", escape(tp.author))
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && zresult == Z_URL_EXISTS {
		P("        <label for=\"url\">URL already exists, enter another one</label>\n")
		P("        <input id=\"url\" class=\"highlight\" name=\"url\" autofocus value=\"%s\">\n", escape(tp.url))
	} else {
		P("        <label for=\"url\">Set URL</label>\n")
		P("        <input id=\"url\" name=\"url\" value=\"%s\">\n", escape(tp.url))
	}
	P("    </div>\n")
	P("    <div>\n")
	if fvalidate && passcode != tp.passcode {
		P("        <label for=\"passcode\">Incorrect passcode, please re-enter</label>\n")
		P("        <input id=\"passcode\" class=\"highlight\" autofocus name=\"passcode\" value=\"%s\">\n", escape(passcode))
	} else {
		P("        <label for=\"passcode\">Enter passcode</label>\n")
		P("        <input id=\"passcode\" name=\"passcode\" value=\"%s\">\n", escape(passcode))
	}
	P("    </div>\n")
	P("    <div class=\"txtpage_form_save\">\n")
	P("        <button type=\"submit\">Save Page</button>\n")
	P("    </div>\n")
	P("</form>\n")
	html_print_close(P)
}

func print_save_page_success(P PrintFunc, tp *TxtPage, r *http.Request) {
	href_link := fmt.Sprintf("/%s", tp.url)
	edit_href_link := fmt.Sprintf("/%s/edit", tp.url)

	page_name := fmt.Sprintf("%s/%s", r.Host, tp.url)
	edit_page_name := fmt.Sprintf("%s/%s/edit", r.Host, tp.url)

	html_print_open(P, "TxtPage Success", "", "")
	P("<h2>TxtPage created!</h2>\n")
	P("<p>Link to your txtpage:<br>\n")
	P("<a href=\"%s\">%s</a></p>", href_link, page_name)
	P("<p>Edit your txtpage:<br>\n")
	P("<a href=\"%s\">%s</a></p>", edit_href_link, edit_page_name)
	P("<p>Passcode: <strong><i>%s</i></strong></p>\n", tp.passcode)
	P("<p>Memorize or write down your passcode and keep it somewhere safe.<br>You will need this when making changes to your txtpage.</p>\n")
	print_footer(P)
	html_print_close(P)
}
