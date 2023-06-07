package main

import (
	"math/rand"
	"strconv"
	"time"
)

func isodate(t time.Time) string {
	return t.Format(time.RFC3339)
}
func parseisodate(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
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
