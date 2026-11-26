package site

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Mohur's privacy page and its terms of use are Mohur's own files,
// docs/policies/privacy.md and terms.md in its repository, copied here byte
// for byte. The app ships the same two files and shows them itself, offline,
// so a reader of the page the Microsoft Store links and a person reading
// them in the app read the same words: the layout is this site's, and nothing
// else is. A test compares the copies with Mohur's (MohursOwnWords).
//
// The files use a small part of Markdown on purpose: headings, paragraphs,
// "- " lists, **bold**, `code` and [links](https://…). This reads them the way
// the app's reader does (client/src/core/policy.mjs in Mohur's repository),
// and refuses what it refuses, so a page cannot show here what the app would
// not show, or the other way round. The page is built from the pieces by
// html/template, which escapes every one of them: no text in a file becomes
// markup.

// maxPolicy is the longest file the app reads.
const maxPolicy = 64 << 10

// piece is a run of text inside a block: plain, bold, code or a link.
type piece struct {
	T    string // "text", "b", "code" or "link"
	V    string
	Href string
}

// block is a heading, a paragraph or a list.
type block struct {
	T      string // "h1", "h2", "h3", "p" or "ul"
	Inline []piece
	Items  [][]piece

	// ID is the anchor a heading carries: a promise's own ID, or the words
	// of any other heading below the title.
	ID string
}

var (
	inlineMarkup = regexp.MustCompile("\\*\\*([^*]+)\\*\\*|`([^`]+)`|\\[([^\\]]+)\\]\\((https://[^\\s)]+)\\)")
	unsupported  = regexp.MustCompile("(?i)[*`\\\\]|\\]\\(|<[a-z!/]|&#?\\w+;|(^|[^\\w])_\\S")
	heading      = regexp.MustCompile(`^(#{1,3}) (.+)$`)
	otherBlock   = regexp.MustCompile("^(\\s|#|>|\\||```|\\d+\\.|\\* |<)")
	promiseID    = regexp.MustCompile(`ID: (mohur-[a-z0-9-]+)\.$`)
)

// inline splits a line into text, bold, code and link pieces.
func inline(text, where string) ([]piece, error) {
	if strings.Contains(text, "![") {
		return nil, fmt.Errorf("%s: images are not shown", where)
	}
	var out []piece
	at := 0
	for _, m := range inlineMarkup.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > at {
			out = append(out, piece{T: "text", V: text[at:m[0]]})
		}
		switch {
		case m[2] >= 0:
			out = append(out, piece{T: "b", V: text[m[2]:m[3]]})
		case m[4] >= 0:
			out = append(out, piece{T: "code", V: text[m[4]:m[5]]})
		default:
			out = append(out, piece{T: "link", V: text[m[6]:m[7]], Href: text[m[8]:m[9]]})
		}
		at = m[1]
	}
	if at < len(text) {
		out = append(out, piece{T: "text", V: text[at:]})
	}
	for _, p := range out {
		if p.T == "text" && unsupported.MatchString(p.V) {
			return nil, fmt.Errorf("%s: unsupported markup in %q", where, p.V)
		}
	}
	return out, nil
}

// parsePolicy turns a policy file into blocks, as the app does.
func parsePolicy(source string) ([]block, error) {
	if source == "" || len(source) > maxPolicy {
		return nil, errors.New("a policy file is missing or too long")
	}
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var (
		blocks []block
		para   []string
		list   [][]string
	)
	flush := func() error {
		if para != nil {
			in, err := inline(strings.Join(para, " "), "paragraph")
			if err != nil {
				return err
			}
			blocks = append(blocks, block{T: "p", Inline: in})
		}
		if list != nil {
			var items [][]piece
			for _, item := range list {
				in, err := inline(strings.Join(item, " "), "list")
				if err != nil {
					return err
				}
				items = append(items, in)
			}
			blocks = append(blocks, block{T: "ul", Items: items})
		}
		para, list = nil, nil
		return nil
	}
	for i, raw := range lines {
		line := strings.TrimRightFunc(raw, unicode.IsSpace)
		where := fmt.Sprintf("line %d", i+1)
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		if h := heading.FindStringSubmatch(line); h != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			in, err := inline(h[2], where)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block{T: fmt.Sprintf("h%d", len(h[1])), Inline: in})
			continue
		}
		if strings.HasPrefix(line, "- ") {
			if para != nil {
				if err := flush(); err != nil {
					return nil, err
				}
			}
			list = append(list, []string{line[2:]})
			continue
		}
		if strings.HasPrefix(line, "  ") && list != nil {
			list[len(list)-1] = append(list[len(list)-1], strings.TrimSpace(line))
			continue
		}
		if otherBlock.MatchString(line) {
			return nil, fmt.Errorf("%s: only headings, paragraphs and \"- \" lists are shown", where)
		}
		if list != nil {
			if err := flush(); err != nil {
				return nil, err
			}
		}
		para = append(para, line)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(blocks) == 0 || blocks[0].T != "h1" {
		return nil, errors.New(`a policy starts with a "# " title`)
	}
	return blocks, anchor(blocks)
}

// text is what a run of pieces says, without its markup.
func text(pieces []piece) string {
	var b strings.Builder
	for _, p := range pieces {
		b.WriteString(p.V)
	}
	return b.String()
}

// anchor gives every heading below the title an ID: a promise its own, so a
// link to one promise keeps working when its wording improves, and any other
// heading its words.
//
// A promise is a "###" heading whose section has a paragraph beginning
// "**How you check it.**" and ending "ID: `mohur-…`.", which is the shape
// Mohur's own test holds its promises to.
func anchor(blocks []block) error {
	seen := map[string]bool{}
	for i := range blocks {
		b := &blocks[i]
		switch b.T {
		case "h2":
			b.ID = slug(text(b.Inline))
		case "h3":
			b.ID = slug(text(b.Inline))
			for _, next := range blocks[i+1:] {
				if next.T != "p" && next.T != "ul" {
					break
				}
				if next.T == "p" && len(next.Inline) > 1 && next.Inline[0].T == "b" && next.Inline[0].V == "How you check it." {
					if m := promiseID.FindStringSubmatch(text(next.Inline)); m != nil && next.Inline[len(next.Inline)-2].T == "code" {
						b.ID = m[1]
					}
				}
			}
		default:
			continue
		}
		if b.ID == "" || seen[b.ID] {
			return fmt.Errorf("the heading %q has no anchor of its own", text(b.Inline))
		}
		seen[b.ID] = true
	}
	return nil
}

// slug is a heading's words as an anchor: lower case, and a hyphen for every
// run of anything else.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	return b.String()
}

// policyPage is what assets/policy.html reads.
type policyPage struct {
	Blocks []block
}
