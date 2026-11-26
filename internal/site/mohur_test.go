package site

import (
	"html"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/denyfirst/site/internal/promises"
)

// mohurPages are Mohur's two policy pages and the files in Mohur's repository
// they are copies of.
var mohurPages = map[string]string{
	"/mohur/privacy": "docs/policies/privacy.md",
	"/mohur/terms":   "docs/policies/terms.md",
}

// The reader refuses what Mohur's app refuses and reads the rest as the app
// does. These are the app's own cases (client/test/policy.test.mjs): if a
// file could show here something the app would not show, the page the Store
// links and the page in the app would be two pages.
func TestThePolicyReaderShowsWhatTheAppShows(t *testing.T) {
	const ok = "# Title\n\nLast changed 1 January 2027.\n\n"
	for _, bad := range []string{
		"<script>alert(1)</script>", "<img src=x onerror=alert(1)>", "| a | b |", "```\ncode\n```", "1. first", "* star", "> quote",
		"    indented", "[link](http://plain.example/)", "[mail](mailto:a@b.c)", "![image](https://x.example/a.png)", "**open bold", "a `tick",
		"an *emphasis*", "an _emphasis_", "an &amp; entity", "an \\* escape", "some <b>html</b> inside", "- a list <i>item</i>",
	} {
		if _, err := parsePolicy(ok + bad); err == nil {
			t.Errorf("%q is read; the app refuses it", bad)
		}
	}
	for _, bad := range []string{"No title\n", "", "# T\n" + strings.Repeat("x", 70000)} {
		if _, err := parsePolicy(bad); err == nil {
			t.Errorf("a policy of %d bytes beginning %.12q is read; the app refuses it", len(bad), bad)
		}
	}

	blocks, err := parsePolicy(ok + "Some **bold**, `code` and a [link](https://example.com/a).\n\n- one\n  more\n- two\n\n## Next")
	if err != nil {
		t.Fatal(err)
	}
	want := []block{
		{T: "p", Inline: []piece{{T: "text", V: "Some "}, {T: "b", V: "bold"}, {T: "text", V: ", "}, {T: "code", V: "code"}, {T: "text", V: " and a "}, {T: "link", V: "link", Href: "https://example.com/a"}, {T: "text", V: "."}}},
		{T: "ul", Items: [][]piece{{{T: "text", V: "one more"}}, {{T: "text", V: "two"}}}},
		{T: "h2", Inline: []piece{{T: "text", V: "Next"}}, ID: "next"},
	}
	if !reflect.DeepEqual(blocks[2:], want) {
		t.Errorf("read as\n%#v\nand the app reads\n%#v", blocks[2:], want)
	}
}

// Each page is the file's words and nothing else: every word in the file is
// on the page in its order, and the page's article has no word that is not
// in the file. The layout around it is the site's, as for every page.
func TestMohursPagesAreItsWordsAndNothingElse(t *testing.T) {
	// Bold, code and links sit inside a line, so their tags join what is
	// either side; every other tag ends a block.
	inlineTag := regexp.MustCompile(`</?(?:strong|code|a)\b[^>]*>`)
	tag := regexp.MustCompile(`<[^>]*>`)
	for path, file := range mohurPages {
		source := asset(t, "assets/mohur/"+filepath.Base(file))
		blocks, err := parsePolicy(source)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		var words []string
		for _, b := range blocks {
			if b.T == "ul" {
				for _, item := range b.Items {
					words = append(words, strings.Fields(text(item))...)
				}
				continue
			}
			words = append(words, strings.Fields(text(b.Inline))...)
		}

		article, ok := between(pageBody(t, path), "<article", "</article>")
		if !ok {
			t.Fatalf("%s has no article", path)
		}
		_, article, _ = strings.Cut(article, ">")
		got := strings.Fields(html.UnescapeString(tag.ReplaceAllString(inlineTag.ReplaceAllString(article, ""), " ")))
		if strings.Join(got, " ") != strings.Join(words, " ") {
			t.Errorf("%s is not %s's words alone:\n got %.200q\nwant %.200q", path, file, strings.Join(got, " "), strings.Join(words, " "))
		}

		if title := text(blocks[0].Inline); pages[path].Title != title+" — denyfirst" {
			t.Errorf("%s is titled %q, and the file %q", path, pages[path].Title, title)
		}
		if !regexp.MustCompile(`^Last changed \d{1,2} [A-Z][a-z]+ \d{4}\.$`).MatchString(text(blocks[1].Inline)) {
			t.Errorf("%s does not say when it last changed", file)
		}
	}
}

// Each of Mohur's promises carries its ID as an anchor, so a link to one
// keeps working when its wording improves, and the promises narrow the
// organisation's without repeating them.
func TestEachOfMohursPromisesHasItsAnchor(t *testing.T) {
	body := pageBody(t, "/mohur/privacy")
	section, ok := between(body, `<h2 id="what-mohur-promises">What Mohur promises</h2>`, "<h2 ")
	if !ok {
		t.Fatal("the privacy page has no anchored section of Mohur's promises")
	}
	ids := regexp.MustCompile(`<h3 id="([^"]+)">`).FindAllStringSubmatch(section, -1)
	if len(ids) != 6 {
		t.Fatalf("%d promises are anchored, and Mohur makes six", len(ids))
	}
	organisation := map[string]bool{}
	for _, p := range promises.Organisation {
		organisation[p.ID] = true
	}
	for _, m := range ids {
		id := m[1]
		if !strings.HasPrefix(id, "mohur-") {
			t.Errorf("%s is not named for Mohur", id)
		}
		if organisation[id] {
			t.Errorf("%s is the organisation's ID too", id)
		}
		if !strings.Contains(section, "ID: <code>"+id+"</code>.") {
			t.Errorf("the anchor %s is not the ID its promise states", id)
		}
	}
	// The anchor Mohur's notes for this site give as the example.
	if !strings.Contains(section, `<h3 id="mohur-servers-carry-only-ciphertext">`) {
		t.Error("the example anchor in Mohur's notes for this site does not exist")
	}
	for _, p := range promises.Organisation {
		if strings.Contains(body, escape(p.Says)) {
			t.Errorf("Mohur's page repeats the organisation's %s, which it may only narrow", p.ID)
		}
	}
}

// Mohur's privacy page sends a reader to this site's for the organisation's
// promises and for what happens to mail to hello@denyfirst.dev, so that page
// has to keep saying both.
func TestThePrivacyPageKeepsWhatMohurSendsReadersFor(t *testing.T) {
	privacy := asset(t, "assets/mohur/privacy.md")
	if strings.Count(privacy, "(https://denyfirst.dev/privacy)") < 2 {
		t.Fatal("Mohur's privacy page no longer sends readers to the organisation's; this test holds the page to less than it says")
	}
	body := pageBody(t, "/privacy")
	mail, ok := between(body, `<h2 id="mail">`, "<h2 ")
	if !ok || !strings.Contains(mail, "hello@denyfirst.dev") {
		t.Error("the organisation's privacy page no longer says what happens to mail to hello@denyfirst.dev")
	}
	if !strings.Contains(body, `<dt id="`+promises.Organisation[0].ID+`">`) {
		t.Error("the organisation's privacy page no longer carries its promises")
	}
}

// Mohur is not released: it is still listed as coming, so the front page says
// it is not available, while these two pages exist for the Store to link
// before it lists the app. And the move to released is not made here by
// accident: Mohur's pages are not a product's home.
func TestMohurIsComingAndItsPagesAreThePolicyOnly(t *testing.T) {
	coming := false
	for _, p := range promises.Coming {
		if p.Name == "Mohur" {
			coming = true
		}
	}
	if !coming {
		t.Error("Mohur is no longer listed as coming; its own page, with where to get it, comes with that change")
	}
	for path := range pages {
		if strings.HasPrefix(path, mohurPrefix) {
			if _, ok := mohurPages[path]; !ok {
				t.Errorf("%s is a page of Mohur's and not one of its policies", path)
			}
		}
	}
}

// Every address under /mohur is this site's. One that is not a page is
// answered here as missing, never sent to Porch, which never had one.
func TestAnAddressUnderMohurIsAnsweredHere(t *testing.T) {
	for _, path := range []string{"/mohur", "/mohur/", "/mohur/privacy/", "/mohur/Privacy", "/mohur/../privacy", "/mohur/llms.txt"} {
		w := get(t, path)
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d with %q, want 404 here", path, w.Code, w.Header().Get("Location"))
		}
		if w.Header().Get("Content-Security-Policy") != contentSecurityPolicy {
			t.Errorf("GET %s is answered without the headers", path)
		}
	}
	for path := range mohurPages {
		if w := on(t, http.MethodPost, siteName, path); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want 405", path, w.Code)
		}
	}
}

// The two pages are Mohur's files, byte for byte, as they are on Mohur's main
// branch, and Mohur's notes for this site name the addresses they are at.
// Those files are in Mohur's repository, not this one, so they are read from
// a checkout of it named by MOHUR_CHECKOUT; CI makes one and this test must
// then run, which the workflow checks. When Mohur's main changes either file,
// this fails until the copy here is the new one: the page the Store links
// does not fall behind the page the app shows.
func TestMohursPoliciesAreMohursOwnWords(t *testing.T) {
	root := os.Getenv("MOHUR_CHECKOUT")
	if root == "" {
		t.Skip("MOHUR_CHECKOUT names no checkout of denyfirst/mohur; CI sets it")
	}
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("Mohur's repository has no %s: %v", name, err)
		}
		return string(body)
	}
	// And the line this site lists Mohur with is the one those notes give.
	for _, p := range promises.Coming {
		if p.Name == "Mohur" && !strings.Contains(read("docs/policies/README.md"), "What: "+p.What+"\n") {
			t.Errorf("Mohur is listed here as %q, which is not the line Mohur's notes for this site give", p.What)
		}
	}
	for path, file := range mohurPages {
		if asset(t, "assets/mohur/"+filepath.Base(file)) != read(file) {
			t.Errorf("%s is not Mohur's %s as it is on Mohur's main branch; copy it again", path, file)
		}
		if !strings.Contains(read("docs/policies/README.md"), "`"+SiteURL+path+"`") {
			t.Errorf("Mohur's notes for this site do not give %s as the address", SiteURL+path)
		}
	}
}
