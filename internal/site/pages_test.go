package site

import (
	"encoding/xml"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/denyfirst/site/internal/promises"
)

// Each page states its own address, and its title and description are the
// same in the tags a link preview reads.
func TestEveryPageSaysWhichAddressItIs(t *testing.T) {
	for path, p := range pages {
		body := pageBody(t, path)
		for _, want := range []string{
			`<title>` + p.Title + `</title>`,
			`<link rel="canonical" href="` + SiteURL + path + `">`,
			`<meta property="og:url" content="` + SiteURL + path + `">`,
			`<meta property="og:title" content="` + p.Title + `">`,
			`<meta property="og:description" content="` + p.Description + `">`,
			`<meta property="og:site_name" content="denyfirst">`,
			`<meta property="og:image" content="` + SiteURL + `/og-denyfirst.png">`,
			`<meta name="description" content="` + p.Description + `">`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not carry %s", path, want)
			}
		}
		if !strings.Contains(p.Title, "denyfirst") {
			t.Errorf("%s is titled %q, which does not name the maker", path, p.Title)
		}
	}
}

// A crawler is told what is here: robots.txt allows it and names the
// sitemap, the sitemap is the page table, and nothing in it is a redirect.
func TestACrawlerIsToldWhatIsHere(t *testing.T) {
	robots := get(t, "/robots.txt").Body.String()
	if robots != "User-agent: *\nAllow: /\n\nSitemap: "+SiteURL+"/sitemap.xml\n" {
		t.Errorf("robots.txt is %q", robots)
	}

	var sitemap struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(get(t, "/sitemap.xml").Body.Bytes(), &sitemap); err != nil {
		t.Fatal(err)
	}
	var listed, want []string
	for _, u := range sitemap.URLs {
		listed = append(listed, u.Loc)
		w := get(t, strings.TrimPrefix(u.Loc, SiteURL))
		if w.Code != http.StatusOK {
			t.Errorf("the sitemap lists %s, which answers %d", u.Loc, w.Code)
		}
	}
	for path := range pages {
		want = append(want, SiteURL+path)
	}
	sort.Strings(want)
	if strings.Join(listed, " ") != strings.Join(want, " ") {
		t.Errorf("the sitemap lists %v, and the pages are %v", listed, want)
	}
}

// llms.txt says what the pages say and no more: every page and product, the
// contact, and sentences a test holds to.
func TestWhatLLMsTxtStatesIsSo(t *testing.T) {
	body := get(t, "/llms.txt").Body.String()
	for path, p := range pages {
		if line := "- [" + p.Title + "](" + SiteURL + path + "): " + p.Description; !strings.Contains(body, line) {
			t.Errorf("llms.txt does not carry %s", line)
		}
	}
	for _, p := range promises.Products {
		if line := "- [" + p.Name + "](" + p.Home + "): " + p.What + " ([llms.txt](" + p.Home + "llms.txt))"; !strings.Contains(body, line) {
			t.Errorf("llms.txt does not carry %s", line)
		}
	}
	if !strings.Contains(body, "- [Reporting a vulnerability]("+SiteURL+SecurityTxtPath+")") {
		t.Error("llms.txt does not say where a vulnerability is reported")
	}
	if !strings.Contains(body, "- [Source code](https://github.com/denyfirst)\n") {
		t.Error("llms.txt does not lead to the organisation's source, every repository of it")
	}
	coming, ok := between(body, "## Coming soon\n\nNot available yet, and no date is promised.\n", "\n## ")
	if !ok {
		t.Fatal("llms.txt does not say what is coming, and that none of it is available")
	}
	for _, p := range promises.Coming {
		if !strings.Contains(coming, "- "+p.Name+": "+p.What+"\n") {
			t.Errorf("llms.txt does not name %s as coming", p.Name)
		}
		if strings.Contains(coming, "](") {
			t.Error("llms.txt links a product that has no page yet")
		}
	}

	// "This site sets no cookie, loads nothing from another server, and
	// carries no tracker": held to every address here.
	if !strings.Contains(body, "This site sets no cookie, loads nothing from another server, and carries no tracker.") {
		t.Fatal("the sentence this test holds to is gone; hold its replacement")
	}
	for _, path := range []string{"/", "/privacy", "/llms.txt", "/robots.txt", "/sitemap.xml", "/style.css", SecurityTxtPath} {
		w := get(t, path)
		if w.Header().Get("Set-Cookie") != "" {
			t.Errorf("%s sets a cookie", path)
		}
		for _, directive := range strings.Split(w.Header().Get("Content-Security-Policy"), ";") {
			if strings.Contains(directive, "http") || strings.Contains(directive, "*") {
				t.Errorf("%s lets a browser load from elsewhere: %s", path, directive)
			}
		}
	}
}

// The privacy page carries every promise the organisation makes, each with an
// anchor that outlives its wording, and says how to check it.
func TestThePrivacyPageCarriesEveryPromise(t *testing.T) {
	body := pageBody(t, "/privacy")
	ids := map[string]bool{}
	for _, p := range promises.Organisation {
		if ids[p.ID] {
			t.Errorf("two promises are %s", p.ID)
		}
		ids[p.ID] = true
		if !strings.Contains(body, `<dt id="`+p.ID+`">`+escape(p.Says)+`</dt>`) {
			t.Errorf("the privacy page does not carry %s as it is written", p.ID)
		}
		if !strings.Contains(body, `<dd><strong>How you check it.</strong> `+escape(p.Checked)+`</dd>`) {
			t.Errorf("the privacy page does not say how to check %s", p.ID)
		}
	}
	if got := strings.Count(body, "<dt id="); got != len(promises.Organisation) {
		t.Errorf("the privacy page carries %d promises, and the organisation makes %d", got, len(promises.Organisation))
	}

	// Each product is named, with the page where it says what it adds.
	_, list, _ := strings.Cut(body, `<h2 id="products">Products</h2>`)
	for _, p := range promises.Products {
		if !strings.Contains(list, `<li><a href="`+p.Promises+`">`+p.Name+`</a> &mdash; `+escape(p.What)+`</li>`) {
			t.Errorf("the privacy page does not name %s with its promises", p.Name)
		}
	}
}

func escape(s string) string {
	return strings.NewReplacer("'", "&#39;", "&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "+", "&#43;").Replace(s)
}

// The address the front page gives is on the privacy page, with what happens
// to a letter sent to it.
func TestTheAddressTheFrontPageGivesIsOnThePrivacyPage(t *testing.T) {
	const address = "hello@denyfirst.dev"
	if !strings.Contains(pageBody(t, "/"), `href="mailto:`+address+`"`) {
		t.Fatalf("the front page does not give %s", address)
	}
	mail, ok := between(pageBody(t, "/privacy"), `<h2 id="mail">Writing to us</h2>`, "<h2")
	if !ok {
		t.Fatal("the privacy page does not say what happens to a letter")
	}
	said := strings.Join(strings.Fields(mail), " ")
	for _, want := range []string{
		`<a href="mailto:` + address + `">` + address + `</a>`,
		"with the address and headers it was sent with",
		"We use it to answer you and for nothing else",
		"it is not passed on",
		`<a href="/.well-known/security.txt">security.txt</a>`,
	} {
		if !strings.Contains(said, want) {
			t.Errorf("the privacy page's section on mail no longer says %q", want)
		}
	}
}

// The front page's index leads to each of its sections, in order, and each
// is numbered as the index numbers it.
func TestTheFrontPagesIndexHolds(t *testing.T) {
	body := pageBody(t, "/")
	index, ok := between(body, `<nav class="hero-index" aria-label="On this page">`, "</nav>")
	if !ok {
		t.Fatal("the opening has no index of the page")
	}
	at := 0
	for i, section := range []struct{ id, name string }{
		{"products", "Products"}, {"principles", "How we work"}, {"releases", "Releases"},
		{"about", "The team"}, {"this-visit", "This visit"},
	} {
		n := "0" + strconv.Itoa(i+1)
		if link := `<a href="#` + section.id + `"><span>` + n + `</span> ` + section.name + `</a>`; !strings.Contains(index, link) {
			t.Errorf("the index does not lead to %s", link)
		}
		where := strings.Index(body, `id="`+section.id+`"`)
		if where < at {
			t.Errorf("section %s is missing, or out of the index's order", section.id)
			continue
		}
		at = where
		if _, rest, _ := strings.Cut(body[where:], `<p class="eyebrow">`); !strings.HasPrefix(rest, n+" / "+section.name+"</p>") {
			t.Errorf("section %s is not numbered %s / %s, as the index says", section.id, n, section.name)
		}
	}
}

// The catalogue on the front page and the menu in the header name the same
// products, and say the same about each: Porch, available and linked, and
// each product still being built, named as coming soon and linked nowhere.
func TestTheCatalogueAndTheMenuNameTheSameProducts(t *testing.T) {
	front := pageBody(t, "/")
	if !strings.Contains(front, `<h3 class="product-feature-name">porch<span class="wordmark-porch-stop">.</span></h3>`) {
		t.Error("the front page's card does not name Porch by its mark")
	}
	if strings.Count(front, `class="badge badge-live"`) != 1 {
		t.Error("more or fewer than one product is marked available")
	}
	for _, p := range promises.Products {
		if !strings.Contains(front, `href="`+p.Home+`"`) {
			t.Errorf("the front page does not lead to %s", p.Name)
		}
	}

	if len(promises.Coming) < 3 {
		t.Fatalf("%d products are coming; this test would pass over fewer than it says", len(promises.Coming))
	}
	soon, ok := between(front, `<div class="product-soon">`, "</div>")
	if !ok {
		t.Fatal("the front page does not say what is coming")
	}
	if !strings.Contains(soon, `<p class="eyebrow">Coming soon</p>`) ||
		!strings.Contains(soon, `<p class="product-soon-note">None of these is available yet, and no date is promised.</p>`) {
		t.Error("the front page does not say its coming products are not available, and that no date is promised")
	}
	if n := strings.Count(soon, "<li>"); n != len(promises.Coming) {
		t.Errorf("the front page names %d coming products, and there are %d", n, len(promises.Coming))
	}
	if strings.Contains(soon, "<a ") {
		t.Error("the front page links a product that has no page yet")
	}
	at := 0
	for _, p := range promises.Coming {
		item := `<li><h3>` + p.Name + `</h3><p>` + escape(p.What) + `</p></li>`
		where := strings.Index(soon, item)
		if where < at {
			t.Errorf("the front page does not carry %s, or not in the list's order", item)
			continue
		}
		at = where
	}

	for path := range pages {
		body := pageBody(t, path)
		menu, ok := between(body, `<ul class="nav-menu-list">`, "</ul>")
		if !ok {
			t.Fatalf("%s has no products menu", path)
		}
		if !strings.Contains(menu, `<a href="`+PorchURL+`/"><span class="nav-menu-name">Porch</span>`) {
			t.Errorf("%s: the products menu does not lead to Porch", path)
		}
		if n := strings.Count(menu, "<li>"); n != len(promises.Products)+len(promises.Coming) {
			t.Errorf("%s: the products menu has %d entries, and there are %d products", path, n, len(promises.Products)+len(promises.Coming))
		}
		for _, p := range promises.Coming {
			if !strings.Contains(menu, `<li><span class="nav-menu-soon"><span class="nav-menu-name">`+p.Name+`</span><span class="nav-menu-note">Coming soon</span></span></li>`) {
				t.Errorf("%s: the products menu does not name %s as coming soon", path, p.Name)
			}
			if strings.Contains(body, p.Name+"</a>") || strings.Contains(strings.ToLower(body), strings.ToLower(p.Name)+".denyfirst.dev") {
				t.Errorf("%s links %s, which has no page yet", path, p.Name)
			}
		}
	}
}

// Each rule on the front page says how to check it.
func TestEachRuleOnTheFrontPageSaysHowToCheckIt(t *testing.T) {
	rules, ok := between(pageBody(t, "/"), `<ol class="rules">`, "</ol>")
	if !ok {
		t.Fatal("the front page has no list of rules")
	}
	if n := strings.Count(rules, "<li>"); n != 3 {
		t.Errorf("the front page lists %d rules, and says it has three", n)
	}
	if n := strings.Count(rules, `<span class="rule-proof-label">How you check it</span>`); n != 3 {
		t.Errorf("%d of the rules say how to check them, not all three", n)
	}
	if !strings.Contains(rules, `href="https://github.com/denyfirst/porch/blob/main/docs/checks.md"`) {
		t.Error("the rule about scope does not link the document that lists every connection")
	}
}

// The policy beside the opening is the one the page is served under.
func TestTheFrontPagesPolicyHolds(t *testing.T) {
	res := get(t, "/")
	body := res.Body.String()
	policy, ok := between(body, `<ol class="policy-rules" aria-label="The policy this page is served under">`, "</ol>")
	if !ok {
		t.Fatal("the front page has no policy beside its opening")
	}
	line := func(verb, what, why string) string {
		return `<li><span class="policy-` + verb + `">` + verb + `</span> <span class="policy-what">` + what +
			`</span> <span class="policy-why"><span aria-hidden="true">#</span> ` + why + `</span></li>`
	}
	want := []string{
		line("deny", "cookies", "no Set-Cookie, ever"),
		line("deny", "trackers", "none in the page"),
		line("deny", "other servers", "CSP default-src &#39;none&#39;"),
		line("deny", "fingerprinting", "no canvas, no font probing"),
		line("deny", "accounts, telemetry", "nothing to sign up for"),
		line("allow", "what you can check", "all our code is public"),
	}
	if got := strings.Count(policy, "<li>"); got != len(want) {
		t.Errorf("the policy has %d lines, want %d", got, len(want))
	}
	for _, l := range want {
		if !strings.Contains(policy, l) && !strings.Contains(policy, strings.ReplaceAll(l, "&#39;", "'")) {
			t.Errorf("the policy does not say %s", l)
		}
	}
	if strings.Index(policy, `policy-allow`) < strings.LastIndex(policy, `policy-deny`) {
		t.Error("the policy allows before it has finished denying")
	}
	if !strings.Contains(body, `<figcaption>Deny first, then allow only what is needed. <a href="#this-visit">What this visit took</a></figcaption>`) {
		t.Error("the policy does not lead to the receipt that shows what the visit took")
	}

	// Each line, against the response.
	if res.Header().Get("Set-Cookie") != "" {
		t.Error("the policy says no cookie is set, and the front page sets one")
	}
	if csp := res.Header().Get("Content-Security-Policy"); !strings.HasPrefix(csp, "default-src 'none';") || strings.Contains(csp, "http") {
		t.Errorf("the policy says other servers are denied, and the page's CSP is %q", csp)
	}
	if strings.Contains(body, "<form") || strings.Contains(body, `type="password"`) {
		t.Error("the policy says there is nothing to sign up for, and the page asks for something")
	}
	scripts := regexp.MustCompile(`<script src="(/[a-z]+\.js)"`).FindAllStringSubmatch(body, -1)
	if len(scripts) == 0 {
		t.Fatal("the front page loads no script, and this check would pass by reading nothing")
	}
	for _, m := range scripts {
		src := get(t, m[1]).Body.String()
		for _, probe := range []string{"canvas", "getContext", "toDataURL", "measureText", "document.fonts", "navigator.",
			"screen.", "AudioContext", "WebGL", "RTCPeerConnection", "getBattery", "deviceMemory", "hardwareConcurrency"} {
			if strings.Contains(src, probe) {
				t.Errorf("the policy says no fingerprinting, and %s uses %s", m[1], probe)
			}
		}
	}
}

// The receipt counts what the page took, and the count is right.
func TestTheReceiptIsTrue(t *testing.T) {
	w := get(t, "/")
	body := w.Body.String()

	slip, _ := between(body, `<article class="slip"`, "</article>")
	rows := map[string]string{}
	for _, m := range regexp.MustCompile(`<div><dt>([^<]+)</dt><dd>([^<]+)</dd></div>`).FindAllStringSubmatch(slip, -1) {
		rows[m[1]] = m[2]
	}
	want := map[string]string{
		"Cookies set": "0", "Trackers": "0", "Other servers asked": "0", "Files loaded": "6",
		"Kept in your cache": "0", "Told to the next site": "nothing",
		"Recorded about you": "nothing", "Total collected": "0",
	}
	if len(rows) != len(want) {
		t.Errorf("the receipt has %d lines, want %d: %v", len(rows), len(want), rows)
	}
	for line, want := range want {
		if rows[line] != want {
			t.Errorf("the receipt says %s %q, want %q", line, rows[line], want)
		}
	}

	// The page, what it links to load, and the one icon a browser asks for.
	var loads []string
	for _, pattern := range []string{`<link rel="stylesheet" href="([^"]+)"`, `<link rel="preload" href="([^"]+)"`, `<script src="([^"]+)"`} {
		for _, m := range regexp.MustCompile(pattern).FindAllStringSubmatch(body, -1) {
			loads = append(loads, m[1])
		}
	}
	icons := regexp.MustCompile(`<link rel="(?:icon|apple-touch-icon)" href="([^"]+)"`).FindAllStringSubmatch(body, -1)
	if len(icons) == 0 {
		t.Fatal("the front page links no icon")
	}
	if got := strconv.Itoa(1 + len(loads) + 1); got != rows["Files loaded"] {
		t.Errorf("the page loads %s files, and the receipt says %s: %v", got, rows["Files loaded"], loads)
	}
	for _, m := range icons {
		loads = append(loads, m[1])
	}
	for _, path := range append([]string{"/"}, loads...) {
		if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
			t.Errorf("the front page loads %s, which is not this site", path)
			continue
		}
		r := get(t, path)
		if r.Code != http.StatusOK {
			t.Errorf("%s answers %d", path, r.Code)
		}
		if c := r.Header().Get("Set-Cookie"); c != "" {
			t.Errorf("%s sets a cookie: %s", path, c)
		}
		if c := r.Header().Get("Cache-Control"); c != "no-store" {
			t.Errorf("%s may be kept: Cache-Control %q", path, c)
		}
	}

	// Kept in your cache: nothing, by the header; and nothing in storage but
	// the one word the switch remembers.
	for _, name := range []string{"assets/theme.js", "assets/hero.js"} {
		src := asset(t, name)
		for _, banned := range []string{"document.cookie", "sessionStorage", "indexedDB", "fetch(", "XMLHttpRequest", "sendBeacon", "WebSocket", "EventSource", "navigator.serviceWorker"} {
			if strings.Contains(src, banned) {
				t.Errorf("%s uses %s, which the receipt says nothing of", name, banned)
			}
		}
	}
	if strings.Contains(asset(t, "assets/hero.js"), "localStorage") {
		t.Error("hero.js stores something, and the receipt says the page keeps nothing")
	}

	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy is %q, so the next site is told where its visitor came from", got)
	}

	tail, _ := between(body, `<dl class="slip-rows slip-total">`, "</article>")
	if strings.Count(tail, "<p") != 1 || !strings.Contains(tail, `<p class="slip-foot">No copy of this receipt was kept.</p>`) {
		t.Errorf("the receipt carries more than its foot after the total:\n%s", tail)
	}
}

// The page ends on the sign-up it declines, and asks for nothing.
func TestTheFrontPageAsksForNothing(t *testing.T) {
	body := pageBody(t, "/")
	end, ok := between(body, `<section class="band band-tint" id="no-sign-up">`, "</section>")
	if !ok {
		t.Fatal("the front page does not end on the sign-up it declines")
	}
	if !strings.Contains(end, "We don&rsquo;t need your email, your name, or\n      anything else about you.") {
		t.Error("the ending no longer says what it does not need")
	}
	text, drawing, ok := strings.Cut(end, `<div class="sign-up" aria-hidden="true">`)
	if !ok {
		t.Fatal("the drawn sign-up is missing, or read aloud")
	}
	if n := strings.Count(end, "<a "); n != 1 {
		t.Errorf("the ending carries %d links; it should carry one, the mail link beside the drawing", n)
	}
	if !strings.Contains(text, `<a class="farewell-field" href="mailto:hello@denyfirst.dev">`) {
		t.Error("the text beside the drawing does not give our address")
	}
	if !strings.Contains(drawing, `<p class="sign-up-stamp">Denied</p>`) {
		t.Error("the drawn sign-up is not stamped")
	}
	for _, control := range []string{"<form", "<input", "<textarea", "<select", "contenteditable", "tabindex"} {
		if strings.Contains(body, control) {
			t.Errorf("the front page carries %s, and says it needs nothing about the reader", control)
		}
	}
	if n := strings.Count(body, `href="#products"`); n != 1 {
		t.Errorf("the front page leads to its products %d times; the index under the opening is enough", n)
	}
}

// The front page says who this is to a search engine with what it already
// shows, and claims nothing it does not.
func TestTheFrontPageSaysWhoItIs(t *testing.T) {
	home := pageBody(t, "/")
	prop := func(name string) []string {
		var out []string
		for _, m := range regexp.MustCompile(`itemprop="`+name+`"(?: (?:content|href)="([^"]*)")?`).FindAllStringSubmatch(home, -1) {
			out = append(out, m[1])
		}
		return out
	}
	if strings.Count(home, `itemtype="https://schema.org/Organization"`) != 1 {
		t.Error("the front page does not say once that it is an organisation's")
	}
	for name, want := range map[string]string{"name": "denyfirst", "url": SiteURL + "/", "sameAs": "https://github.com/denyfirst"} {
		if got := prop(name); len(got) != 1 || got[0] != want {
			t.Errorf("the organisation's %s is %q, want %q", name, got, want)
		}
	}
	if logo := prop("logo"); len(logo) != 1 || !strings.HasPrefix(logo[0], SiteURL+"/") {
		t.Errorf("the organisation's logo is %q", logo)
	} else if w := get(t, strings.TrimPrefix(logo[0], SiteURL)); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("the organisation's logo %s answers %d %s", logo[0], w.Code, w.Header().Get("Content-Type"))
	}
	if !strings.Contains(home, `<p class="band-prose" itemprop="description">denyfirst is an independent team`) {
		t.Error("the organisation is not described by the sentence the page says it in")
	}
	for path := range pages {
		body := pageBody(t, path)
		for _, never := range []string{"application/ld+json", "aggregateRating", `itemprop="review`, "ratingValue"} {
			if strings.Contains(body, never) {
				t.Errorf("%s carries %s", path, never)
			}
		}
	}
}

// The team's card and the release record state facts about Porch's
// repository: its key, its licence, its dependencies, and how a release is
// made. Those files are in Porch's repository, not this one, so they are read
// from a checkout of it named by PORCH_CHECKOUT; CI makes one and this test
// must then run, which the workflow checks.
func TestTheFrontPagesFactsAboutPorchHold(t *testing.T) {
	root := os.Getenv("PORCH_CHECKOUT")
	if root == "" {
		t.Skip("PORCH_CHECKOUT names no checkout of denyfirst/porch; CI sets it")
	}
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("the front page relies on %s in Porch's repository: %v", name, err)
		}
		return string(body)
	}
	body := pageBody(t, "/")

	const repo = "https://github.com/denyfirst/porch/blob/main/"
	const key = "SHA256:ut6bginhZ4lZINMSXNDv3vJ6fyvmDHhtnoBJH0/Nr9Y"
	for _, line := range []struct{ term, file, says string }{
		{"Known by", "docs/verify.md", "<code>" + key + "</code>"},
		{"Releases", "docs/verify.md", "Signed and reproducible"},
		{"Licence", "LICENSE", "AGPL-3.0"},
		{"Dependencies", "go.mod", "None"},
	} {
		if want := "<dt>" + line.term + `</dt><dd><a href="` + repo + line.file + `">` + line.says + "</a></dd>"; !strings.Contains(body, want) {
			t.Errorf("the team's card does not say %s", want)
		}
		read(line.file)
	}
	if !strings.Contains(body, "<dt>Investors</dt><dd>None</dd>") {
		t.Error("the team's card does not say it has no investors")
	}
	verify := read("docs/verify.md")
	if !strings.Contains(verify, "Good \"file\" signature for releases@denyfirst.dev with ED25519 key "+key) {
		t.Error("the key on the team's card is not the one Porch's verification guide expects")
	}
	if !strings.Contains(verify, "Builds are reproducible.") {
		t.Error("Porch's docs/verify.md no longer says how a release is rebuilt")
	}
	if licence := read("LICENSE"); !strings.Contains(licence, "GNU AFFERO GENERAL PUBLIC LICENSE") || !strings.Contains(licence, "Version 3") {
		t.Error("Porch's licence is not the AGPL-3.0 the card names")
	}
	if strings.Contains(read("go.mod"), "require") {
		t.Error("Porch's go.mod requires a module, and the card says there are no dependencies")
	}
	if !strings.Contains(strings.Join(strings.Fields(read("SECURITY.md")), " "), "This is an unfunded project") {
		t.Error("Porch's SECURITY.md no longer says the project is unfunded, and the card says it has no investors")
	}

	guide := strings.Join(strings.Fields(read("docs/releasing.md")), " ")
	record, ok := between(body, `<ol class="record-rows">`, "</ol>")
	if !ok {
		t.Fatal("the front page does not say how a release reaches a reader")
	}
	tick := `<span class="record-tick" aria-hidden="true"><svg viewBox="0 0 16 16" focusable="false"><path d="M3 8.5l3 3 7-7"/></svg></span>`
	for _, step := range []struct{ title, who, proof, file, guide string }{
		{"Tag signed", "Maintainer", "<code>git tag -s</code>", "", "| Signed tag | Which commit is being released, and by whom |"},
		{"Built in public", "A workflow", "<code>build-release.yml</code>, which cannot sign", ".github/workflows/build-release.yml", "in a public log, and cannot sign"},
		{"Rebuilt, every byte matched, signed", "Maintainer", "<code>release.ps1 -Compare</code>, <code>SHA256SUMS.sig</code>", "scripts/release.ps1", "Any difference refuses the signature"},
		{"Rebuilt again, after publishing", "Another workflow", "<code>reproduce.yml</code>", ".github/workflows/reproduce.yml", "Someone other than the maintainer can rebuild the same bytes"},
	} {
		row := "<li>" + tick + `<strong><span class="visually-hidden">Done: </span>` + step.title + `</strong><span class="record-proof"><span class="record-who">` +
			step.who + "</span> " + step.proof + "</span></li>"
		if !strings.Contains(record, row) {
			t.Errorf("the release record does not carry %s", row)
		}
		if !strings.Contains(guide, step.guide) {
			t.Errorf("Porch's docs/releasing.md no longer says %q, which the line %q stands on", step.guide, step.title)
		}
		if step.file != "" {
			read(step.file)
		}
	}
	if !strings.Contains(record, `<li class="record-you"><span class="record-tick" aria-hidden="true"></span><strong><span class="visually-hidden">Yours to do: </span>Checked by you</strong>`+
		`<span class="record-proof"><span class="record-who">You</span> <a href="`+repo+`docs/verify.md">Your turn: verify a release</a></span></li>`) {
		t.Error("the release record's last line is not left open for the reader, with the guide that fills it")
	}
	if strings.Count(record, "<li") != 5 {
		t.Error("the release record does not have the five lines it draws")
	}
	if !strings.Contains(guide, "Two parties make a release and neither can do it alone") {
		t.Error("Porch's docs/releasing.md no longer says two parties make a release, which the band repeats")
	}
	read("docs/checks.md")
}

// The lens over the opening is decoration: it reads where the pointer is,
// keeps nothing, sends nothing, and does nothing where it would be a nuisance.
func TestTheHeroLensIsDecorationAndNothingMore(t *testing.T) {
	js := asset(t, "assets/hero.js")
	for _, want := range []string{
		`"(hover: hover) and (pointer: fine)"`,
		`"(prefers-reduced-motion: reduce)"`,
		`lens.style.setProperty("--lens-x", x + "px");`,
		`hero.classList.remove("hero-lit")`,
		"window.requestAnimationFrame(",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("hero.js no longer contains %q", want)
		}
	}
	for _, never := range []string{"fetch(", "XMLHttpRequest", "sendBeacon", "localStorage", "sessionStorage", "document.cookie", "innerHTML", `setAttribute("style"`} {
		if strings.Contains(js, never) {
			t.Errorf("hero.js contains %q, and decoration has no business doing so", never)
		}
	}
	css := stylesheet(t)
	for _, want := range []string{"@property --lens-r {", "initial-value: 0px;", ".hero-lit .hero-lens { --lens-r: 15rem; }", "pointer-events: none;"} {
		if !strings.Contains(css, want) {
			t.Errorf("style.css no longer contains %q", want)
		}
	}
	for path := range pages {
		body := pageBody(t, path)
		if carries, front := strings.Contains(body, `<script src="/hero.js" defer></script>`), path == "/"; carries != front {
			t.Errorf("%s: loads hero.js %v, want %v", path, carries, front)
		}
	}
	lens, ok := between(pageBody(t, "/"), `<div class="hero-lens" aria-hidden="true">`, "</div>")
	if !ok || !strings.Contains(lens, "Look closer.") {
		t.Errorf("the lens is missing, read aloud, or without its quiet words: %s", lens)
	}
}

// The switch is a mark that says what pressing it does, and keeps the name a
// screen reader announces.
func TestTheColourSwitchIsAMarkAndKeepsItsName(t *testing.T) {
	src := asset(t, "assets/theme.js")
	for _, want := range []string{
		`const TO_DARK = "M12 4a8 8 0 0 1 0 16Z";`,
		`const TO_LIGHT = "M12 4a8 8 0 0 0 0 16Z";`,
		`half.setAttribute("d", toLight ? TO_LIGHT : TO_DARK);`,
		"\n    button.setAttribute(\"aria-label\",\n",
		`window.localStorage.setItem(KEY, next);`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("theme.js no longer carries %s", want)
		}
	}
	// It stores the one word, under the one key, and reads nothing else.
	if n := strings.Count(src, "localStorage."); n != 2 {
		t.Errorf("theme.js touches storage %d times; it reads and writes the one word", n)
	}
	for path := range pages {
		body := pageBody(t, path)
		if !strings.Contains(body, `<button class="theme-toggle" id="theme-toggle" type="button" hidden>Dark</button>`) {
			t.Errorf("%s has no switch, or shows one before the script that works it", path)
		}
		if !strings.Contains(body, "<head>") || !strings.Contains(strings.SplitN(body, "</head>", 2)[0], `<script src="/theme.js"></script>`) {
			t.Errorf("%s does not load theme.js in its head, before anything is drawn", path)
		}
	}
	sheet := stylesheet(t)
	if !strings.Contains(cssRule(t, sheet, ".theme-toggle .theme-half"), "fill: currentColor") {
		t.Error("the filled half does not take the button's colour")
	}
	if rule := cssRule(t, sheet, ".theme-toggle"); !strings.Contains(rule, "width: 34px") || !strings.Contains(rule, "height: 34px") {
		t.Errorf("the switch is not a square target: %q", rule)
	}
}

// The wordmark ends in the brand's red: both halves of the name are marked up
// wherever it is written, so one rule colours them.
func TestTheWordmarkEndsInTheBrandColour(t *testing.T) {
	if !strings.Contains(cssRule(t, stylesheet(t), ".wordmark-deny, .wordmark-stop"), "color: var(--brand)") {
		t.Error("the first half and the full stop are not both the brand's red")
	}
	for path := range pages {
		body := pageBody(t, path)
		if strings.Contains(body, `<span class="wordmark-deny">deny</span>first.`) {
			t.Errorf("%s: the full stop is plain text, so no rule can colour it", path)
		}
		if n, m := strings.Count(body, `<span class="wordmark-deny">deny</span>first`), strings.Count(body, `first<span class="wordmark-stop">.</span>`); n != m || n < 2 {
			t.Errorf("%s: the name is written %d times and its stop marked %d", path, n, m)
		}
	}
}

// Each footer leads to the organisation's own documents.
func TestTheFooterLeadsToThePrivacyPageAndTheContacts(t *testing.T) {
	for path := range pages {
		foot, ok := between(pageBody(t, path), `<footer class="colophon">`, "</footer>")
		if !ok {
			t.Fatalf("%s has no footer", path)
		}
		for _, want := range []string{`<a href="` + SiteURL + `/privacy">Privacy</a>`, `<a href="` + SiteURL + SecurityTxtPath + `">Security</a>`} {
			if !strings.Contains(foot, want) {
				t.Errorf("%s: the footer does not carry %s", path, want)
			}
		}
	}
}
