package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

func main() {
	s := "string"
	s2 := s[:4]

	desc_len := 200
	content_len := len(s)
	if content_len < desc_len {
		desc_len = content_len
	}
	s3 := s[:desc_len]
	fmt.Printf("s: '%s' s2: '%s' s3: '%s'\n", s, s2, s3)
}

func test_time() {
	t := time.Now()
	fmt.Printf("t: '%s'\n", isodate(t))

	t = t.Add(60 * time.Hour * 24)
	fmt.Printf("t + 60 days: '%s'\n", isodate(t))

	t = t.Add(-60 * time.Hour * 24)
	fmt.Printf("t - 60 days: '%s'\n", isodate(t))

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			newtime := <-ticker.C
			fmt.Printf("ticker: newtime: '%s'\n", isodate(newtime))
			t := newtime.Add(days_to_duration(-22))
			fmt.Printf("ticker: t-22 days: '%s'\n", isodate(t))
		}
	}()

	for {
	}
}

func test_md() {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)

	bs, err := os.ReadFile("test.md")

	var buf bytes.Buffer
	err = md.Convert(bs, &buf)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", buf.String())
}
