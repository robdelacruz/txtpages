package main

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

func main() {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)

	bs, err := ioutil.ReadFile("test.md")

	var buf bytes.Buffer
	err = md.Convert(bs, &buf)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", buf.String())
}
