package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
    initdbfile string
    dbfile string
    port string
}

func main() {
    var err error
	sw, parms := parseArgs(os.Args)
	// [-i new_file]  Create and initialize db file
	if sw["i"] != "" {
		dbfile := sw["i"]
		if fileExists(dbfile) {
			s := fmt.Sprintf("File '%s' already exists.\n", dbfile)
			fmt.Printf(s)
			os.Exit(1)
		}
		err = createTables(dbfile)
        if err != nil {
            log.Fatal(err)
        }
		os.Exit(0)
	}

	if len(parms) == 0 {
		fmt.Printf(
`Usage:

Start webservice:
	%[1]s <dbfile> [port]

Initialize db file:
	%[1]s -i <dbfile>

`, os.Args[0])
		os.Exit(0)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/", indexHandler())

	port := "8000"

	fmt.Printf("Listening on %s...\n", port)
    err = http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	log.Fatal(err)
}

func indexHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<h1>index page</h1>")
	}
}

const (
    PA_NONE = iota
    PA_DBFILE
    PA_INITDBFILE
)

func parseArgs(args []string, conf *Config) {
    state := PA_NONE
    isDBFileSet := False
    isPortSet := False

    for i:=1; i < len(args); i++ {
        arg := args[i]
        if state == PA_NONE && arg == "-i" {
        }
	}
}

func listContains(ss []string, v string) bool {
	for _, s := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func createTables(dbfile string) error {
	if fileExists(dbfile) {
        return fmt.Errorf("File '%s' exists", dbfile)
	}

	db, err := sql.Open("sqlite3", dbfile)
	if err != nil {
        return err
	}

    s :=
    `CREATE TABLE page (
        page_id INTEGER PRIMARY KEY NOT NULL,
        title TEXT NOT NULL DEFAULT '',
        url TEXT NOT NULL DEFAULT '',
        content TEXT NOT NULL DEFAULT '',
        editcode TEXT NOT NULL DEFAULT '',
        createdt TEXT NOT NULL,
        lastreaddt TEXT NOT NULL
    );
    INSERT INTO page (page_id, title, url, content, editcode, createdt, lastreaddt)
    VALUES(1, "First Post!", "firstpost", "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.", "password", strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));`

    _, err = sqlexec(db, s)
    if err != nil {
        return err
    }
    return nil
}
