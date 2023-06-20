package main

import (
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"
)

const ISO8601Fmt = "2006-01-02T15:04:05Z"
const RFC3339 = time.RFC3339

func isodate(t time.Time) string {
	return t.Format(ISO8601Fmt)
}
func parseisodate(s string) time.Time {
	t, _ := time.Parse(ISO8601Fmt, s)
	return t
}
func formatisodate(s string) string {
	t := parseisodate(s)
	return t.Format("2 Jan 2006")
}
func formatdate(s string) string {
	t := parseisodate(s)
	return t.Format("2 Jan 2006")
}
func randdate(startyear, endyear int) time.Time {
	minSecs := time.Date(startyear, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	maxSecs := time.Date(endyear, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	secs := minSecs + rand.Int63n(maxSecs-minSecs)
	return time.Unix(secs, 0)
}
func nowdate() string {
	return time.Now().Format(ISO8601Fmt)
}
func days_to_duration(ndays int) time.Duration {
	return time.Duration(ndays) * time.Hour * 24
}

func atoi(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
func idtoi(sid string) int64 {
	return int64(atoi(sid))
}
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
func atof(s string) float64 {
	if s == "" {
		return 0.0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return f
}

func ss_contains(ss []string, v string) bool {
	for _, s := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func file_exists(file string) bool {
	_, err := os.Stat(file)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func create_logger_from_file(logfile string) (*log.Logger, error) {
	f, err := os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	logger := log.New(f, "", 0)
	return logger, nil
}

type LogPrintfFunc func(fmt string, a ...interface{})
type LogErrFunc func(prefix string, err error)

func make_log_print_func(logger *log.Logger) LogPrintfFunc {
	return func(fmt string, a ...interface{}) {
		logger.Printf(fmt, a...)
	}
}
func make_log_err_func(logger *log.Logger) LogErrFunc {
	return func(prefix string, err error) {
		logger.Printf("%s: %s\n", prefix, err.Error())
	}
}
