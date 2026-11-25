package site

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// readableContrast is WCAG 2's minimum for body text, 4.5:1. Every colour
// text is set in is held to it against every surface it can sit on, in both
// schemes, so a token in the stylesheet is a promise rather than a taste.
const readableContrast = 4.5

var (
	tokenPattern      = regexp.MustCompile(`--([a-z-]+):\s*(#[0-9a-fA-F]{6})\s*;`)
	textColourPattern = regexp.MustCompile(`(?m)(?:^|[;{\s])color:\s*var\(--([a-z-]+)\)`)
)

func relativeLuminance(hex string) float64 {
	channel := func(offset int) float64 {
		v, err := strconv.ParseInt(hex[offset:offset+2], 16, 32)
		if err != nil {
			return 0
		}
		c := float64(v) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	hex = strings.TrimPrefix(hex, "#")
	return 0.2126*channel(0) + 0.7152*channel(2) + 0.0722*channel(4)
}

func contrast(a, b string) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	high, low := math.Max(la, lb), math.Min(la, lb)
	return (high + 0.05) / (low + 0.05)
}

// schemes reads the light tokens and the dark ones laid over them.
func schemes(t *testing.T, sheet string) map[string]map[string]string {
	t.Helper()
	block := func(from string) string {
		start := strings.Index(sheet, from)
		if start < 0 {
			t.Fatalf("the stylesheet has no %q block", from)
		}
		rest := sheet[start:]
		end := strings.Index(rest, "\n}")
		if end < 0 {
			t.Fatalf("the %q block is not closed", from)
		}
		return rest[:end]
	}
	parse := func(text string) map[string]string {
		found := map[string]string{}
		for _, m := range tokenPattern.FindAllStringSubmatch(text, -1) {
			found[m[1]] = strings.ToLower(m[2])
		}
		return found
	}
	light := parse(block(":root {"))
	if len(light) < 8 {
		t.Fatalf("only %d colour tokens were found, which is too few to be right", len(light))
	}
	dark := map[string]string{}
	for k, v := range light {
		dark[k] = v
	}
	for k, v := range parse(block("@media screen and (prefers-color-scheme: dark)")) {
		dark[k] = v
	}
	return map[string]map[string]string{"light": light, "dark": dark}
}

func TestEveryColourTextIsSetInIsLegible(t *testing.T) {
	sheet := stylesheet(t)
	tables := schemes(t, sheet)

	used := map[string]bool{}
	for _, m := range textColourPattern.FindAllStringSubmatch(sheet, -1) {
		used[m[1]] = true
	}
	if len(used) < 5 {
		t.Fatalf("only %d text colours were found in the stylesheet, which is too few to be right", len(used))
	}

	// The organisation's pages override four tokens with the brand's own
	// (body[data-owner="organisation"]); text is set in those here.
	overrides := map[string]string{"link": "brand-link", "tool": "brand", "accent": "brand-accent"}

	for _, scheme := range []string{"light", "dark"} {
		tokens := tables[scheme]
		for token := range used {
			if over, ok := overrides[token]; ok {
				token = over
			}
			value, ok := tokens[token]
			if !ok {
				t.Errorf("%s: text is set in --%s, which is not defined", scheme, token)
				continue
			}
			if token == "paper" {
				if got := contrast(value, tokens["ink"]); got < readableContrast {
					t.Errorf("%s: --paper on --ink is %.2f:1", scheme, got)
				}
				continue
			}
			on := []string{"paper", "paper-sunk"}
			switch {
			case strings.HasPrefix(token, "term-"):
				on = []string{"term-bg", "term-bar"}
			case strings.HasPrefix(token, "slip-"):
				on = []string{"slip-paper"}
			}
			for _, surface := range on {
				if got := contrast(value, tokens[surface]); got < readableContrast {
					t.Errorf("%s: --%s (%s) on --%s (%s) is %.2f:1, below %.1f:1",
						scheme, token, value, surface, tokens[surface], got, readableContrast)
				}
			}
		}
	}
}

func TestTheContrastArithmeticIsRight(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want float64
	}{
		{"#000000", "#ffffff", 21},
		{"#ffffff", "#ffffff", 1},
		{"#777777", "#ffffff", 4.48},
		{"#767676", "#ffffff", 4.54},
	} {
		if got := contrast(c.a, c.b); math.Abs(got-c.want) > 0.01 {
			t.Errorf("contrast(%s, %s) = %.2f, want %.2f", c.a, c.b, got, c.want)
		}
	}
}

// A dark system and the switch set to dark give the same page: the two
// blocks carry the same tokens with the same values.
func TestBothWaysToTheDarkSchemeAreTheSame(t *testing.T) {
	sheet := stylesheet(t)
	block := func(from string) map[string]string {
		start := strings.Index(sheet, from)
		if start < 0 {
			t.Fatalf("the stylesheet has no %q block", from)
		}
		rest := sheet[start:]
		rest = rest[:strings.Index(rest, "}")]
		found := map[string]string{}
		for _, m := range regexp.MustCompile(`--([a-z-]+):\s*([^;]+);`).FindAllStringSubmatch(rest, -1) {
			found[m[1]] = strings.TrimSpace(m[2])
		}
		return found
	}
	system := block(`:root:not([data-theme="light"]) {`)
	chosen := block(`:root[data-theme="dark"] {`)
	if len(system) < 8 {
		t.Fatalf("only %d tokens in the dark scheme", len(system))
	}
	if len(system) != len(chosen) {
		t.Errorf("the dark scheme has %d tokens one way and %d the other", len(system), len(chosen))
	}
	for k, v := range system {
		if chosen[k] != v {
			t.Errorf("--%s is %s on a dark system and %s when dark is chosen", k, v, chosen[k])
		}
	}
}

// Nothing at rest is faded out: a state is said in words and colour.
func TestNoRestingStateIsFadedOut(t *testing.T) {
	stripped := regexp.MustCompile(`(?s)@keyframes[^{]*\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}`).ReplaceAllString(stylesheet(t), "")
	for _, m := range regexp.MustCompile(`opacity:\s*(0(?:\.\d+)?)\s*[;}]`).FindAllStringSubmatch(stripped, -1) {
		t.Errorf("a resting state is faded to opacity %s", m[1])
	}
}

// fontSHA256 is the typeface that was chosen. It changes when somebody means
// it to.
const fontSHA256 = "4c8b93f431d462c696e12b9d6a033feb3394d36e66e781357c496b95d8a75e05"

func TestTheTypefaceIsServedFromHereAndIsTheOneChosen(t *testing.T) {
	body := asset(t, "assets/schibsted-grotesk.woff2")
	if sum := sha256.Sum256([]byte(body)); hex.EncodeToString(sum[:]) != fontSHA256 {
		t.Errorf("the typeface is not the file that was chosen: sha256 %x", sum)
	}
	licence := asset(t, "assets/schibsted-grotesk-OFL.txt")
	for _, want := range []string{"SIL OPEN FONT LICENSE Version 1.1", "Copyright 2023 The Schibsted-Grotesk Project Authors"} {
		if !strings.Contains(licence, want) {
			t.Errorf("the typeface's licence does not say %q", want)
		}
	}
	if res := get(t, FontPath); res.Body.String() != body {
		t.Errorf("%s is not the font", FontPath)
	}
	sheet := stylesheet(t)
	if n := strings.Count(sheet, "@font-face"); n != 1 {
		t.Errorf("the stylesheet declares %d faces, not one", n)
	}
	urls := regexp.MustCompile(`url\(([^)]*)\)`).FindAllStringSubmatch(sheet, -1)
	if len(urls) != 1 || urls[0][1] != `"`+FontPath+`"` {
		t.Errorf("the stylesheet loads %v, not the typeface from this server alone", urls)
	}
}

// The arrows are drawn, not set as characters a reader's fonts draw
// differently, and they keep still for a reader who asked for less motion.
func TestArrowsAreDrawnAndKeepStill(t *testing.T) {
	sheet := stylesheet(t)
	if strings.ContainsAny(sheet, "↗↘→↓▾") {
		t.Error("the stylesheet sets an arrow as a character")
	}
	for path := range pages {
		if strings.ContainsAny(pageBody(t, path), "↗↘→") {
			t.Errorf("%s writes an arrow into its text", path)
		}
	}
	shape := cssRule(t, sheet, ".arrow-ne::after")
	for _, want := range []string{`content: "";`, "border-top:", "border-right:"} {
		if !strings.Contains(shape, want) {
			t.Errorf("the arrow is not drawn: its rule lacks %q", want)
		}
	}
	still := ""
	for _, b := range regexp.MustCompile(`(?s)@media \(prefers-reduced-motion: reduce\) \{(.*?)\n\}`).FindAllStringSubmatch(sheet, -1) {
		still += b[1]
	}
	for _, want := range []string{".arrow-ne::after { transition: none; }"} {
		if !strings.Contains(still, want) {
			t.Errorf("the reduced-motion rules do not include %q", want)
		}
	}
}

// Products is a menu that opens on hover or focus, drawn by the stylesheet
// alone, and is closed until somebody asks.
func TestProductsAreAMenuNotABar(t *testing.T) {
	sheet := stylesheet(t)
	for _, want := range []string{".nav-menu:hover .nav-menu-list", ".nav-menu:focus-within .nav-menu-list", ".nav-menu-list::before"} {
		if !strings.Contains(sheet, want) {
			t.Errorf("the menu does not open with %s", want)
		}
	}
	if list := cssRule(t, sheet, ".nav-menu-list"); !strings.Contains(list, "display: none") || !strings.Contains(list, "right: -1rem;") {
		t.Errorf("the menu is open before anybody asks for it, or not anchored by its right edge:\n%s", list)
	}
}

// Every class and id the stylesheet names is on a page or put there by a
// script. The sheet began as Porch's, which drew much more than these two
// pages; a rule for something that is not here is weight nobody reviews.
func TestTheStylesheetDrawsOnlyWhatIsHere(t *testing.T) {
	sheet := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(stylesheet(t), "")
	// Selectors only: what comes before each brace, outside declarations.
	var selectors strings.Builder
	for _, m := range regexp.MustCompile(`(?m)([^{};]+)\{`).FindAllStringSubmatch(sheet, -1) {
		if !strings.HasPrefix(strings.TrimSpace(m[1]), "@") {
			selectors.WriteString(m[1] + "\n")
		}
	}
	var here strings.Builder
	for path := range pages {
		here.WriteString(pageBody(t, path))
	}
	here.WriteString(asset(t, "assets/theme.js"))
	here.WriteString(asset(t, "assets/hero.js"))
	text := here.String()

	var missing []string
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`([.#])([a-zA-Z][a-zA-Z0-9_-]*)`).FindAllStringSubmatch(selectors.String(), -1) {
		name := m[2]
		if seen[m[0]] {
			continue
		}
		seen[m[0]] = true
		found := false
		if m[1] == "#" {
			found = strings.Contains(text, `id="`+name+`"`)
		} else {
			found = regexp.MustCompile(`class="[^"]*\b`+regexp.QuoteMeta(name)+`\b[^"]*"`).MatchString(text) ||
				strings.Contains(text, `"`+name+`"`)
		}
		if !found {
			missing = append(missing, m[0])
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("the stylesheet draws %d things that are on no page: %s", len(missing), strings.Join(missing, " "))
	}
}

// The organisation's pages keep the brand's red for their links and buttons;
// Porch's violet is only Porch's card.
func TestTheOrganisationsPagesKeepTheBrandColour(t *testing.T) {
	rule := cssRule(t, stylesheet(t), `body[data-owner="organisation"]`)
	for _, want := range []string{"--tool:         var(--brand);", "--link:         var(--brand-link);", "--accent:       var(--brand-accent);"} {
		if !strings.Contains(rule, want) {
			t.Errorf("the organisation's override lacks %s", want)
		}
	}
	for path := range pages {
		if !strings.Contains(pageBody(t, path), `<body data-owner="organisation">`) {
			t.Errorf("%s is not marked as the organisation's", path)
		}
	}
}
