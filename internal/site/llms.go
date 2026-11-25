package site

import (
	"bytes"
	"sort"
	"strings"
	"text/template"

	"github.com/denyfirst/site/internal/promises"
)

// llms.txt is a page's worth of plain text that says what a site is, for a
// program that reads it rather than a person: the proposal at llmstxt.org.
// Whether a given assistant reads it is that assistant's business; offering
// it costs a few hundred bytes.
//
// It is written from the same table as the pages and the sitemap, so a page
// added to one is in the other, and its few sentences of fact are ones the
// pages already say. A file for machines that said more than the pages would
// be the one place this site could overstate itself unread, and the thing a
// model repeats is the file.

// llmsPage is one line under Pages.
type llmsPage struct {
	Title, URL, Description string
}

// llmsFile is what assets/llms.txt reads.
type llmsFile struct {
	Pages    []llmsPage
	Products []promises.Product
	SiteURL  string
}

// buildLLMs renders /llms.txt. Called at startup, after the pages.
func buildLLMs() []byte {
	var list []llmsPage
	for path := range rendered {
		p := pages[path]
		list = append(list, llmsPage{Title: p.Title, URL: canonicalURL(path), Description: p.Description})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].URL < list[j].URL })

	source, err := assets.ReadFile("assets/llms.txt")
	if err != nil {
		panic("site: " + err.Error())
	}
	t, err := template.New("llms.txt").Option("missingkey=error").Parse(string(source))
	if err != nil {
		panic("site: llms.txt: " + err.Error())
	}
	var out bytes.Buffer
	if err := t.Execute(&out, llmsFile{Pages: list, Products: promises.Products, SiteURL: SiteURL}); err != nil {
		panic("site: llms.txt: " + err.Error())
	}
	return []byte(strings.TrimLeft(out.String(), "\n"))
}
