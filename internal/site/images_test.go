package site

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

// pngShape reads a PNG's size and colour type, and fails on any chunk that is
// not part of the picture. A screenshot or an editor writes its own name, a
// path or a date into a PNG unless it is told not to, and none of those is
// anybody's business.
func pngShape(t *testing.T, name string, raw []byte) (width, height, colour int) {
	t.Helper()
	if !bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("%s holds something that is not a PNG", name)
	}
	for at := 8; at+8 <= len(raw); {
		length := int(binary.BigEndian.Uint32(raw[at:]))
		switch kind := string(raw[at+4 : at+8]); kind {
		case "IHDR":
			width = int(binary.BigEndian.Uint32(raw[at+8:]))
			height = int(binary.BigEndian.Uint32(raw[at+12:]))
			colour = int(raw[at+17])
		case "PLTE", "tRNS", "IDAT", "IEND":
		default:
			t.Errorf("%s carries a %s chunk, which is not part of the picture", name, kind)
		}
		at += 12 + length
	}
	return width, height, colour
}

// The tab icon is the mark and only shapes: nothing in it can run, load or
// link anything.
func TestTheTabIconIsTheMarkAndOnlyShapes(t *testing.T) {
	svg := strings.ToLower(asset(t, "assets/denyfirst.svg"))
	for _, banned := range []string{"<script", "href", "<image", "<use", "<foreignobject", "<style", "url(", "javascript:", " on"} {
		if strings.Contains(svg, banned) {
			t.Errorf("the icon contains %q", banned)
		}
	}
	if !strings.Contains(svg, `aria-label="denyfirst"`) || !strings.Contains(svg, `fill="#ed4552"`) {
		t.Error("the icon is not denyfirst's mark with its red stop")
	}
	if n := strings.Count(svg, "<path"); n != 2 {
		t.Errorf("the icon draws %d paths, not a letter and a stop", n)
	}
	for path := range pages {
		body := pageBody(t, path)
		if n := strings.Count(body, `rel="icon"`); n != 1 || !strings.Contains(body, `<link rel="icon" href="/icon-denyfirst.svg" type="image/svg+xml">`) {
			t.Errorf("%s links %d icons, not the mark once", path, n)
		}
	}
}

// For what reads no SVG: the mark at 16, 32 and 48 pixels in an .ico, and
// an opaque square of it at 180 for a phone's home screen.
func TestTheFallbackIconsAreTheMarks(t *testing.T) {
	raw := []byte(asset(t, "assets/denyfirst.ico"))
	if len(raw) < 6 || binary.LittleEndian.Uint16(raw[0:]) != 0 || binary.LittleEndian.Uint16(raw[2:]) != 1 {
		t.Fatal("denyfirst.ico is not an icon file")
	}
	var sizes []int
	for i := 0; i < int(binary.LittleEndian.Uint16(raw[4:])); i++ {
		entry := raw[6+16*i:]
		size := int(entry[0])
		length := int(binary.LittleEndian.Uint32(entry[8:]))
		offset := int(binary.LittleEndian.Uint32(entry[12:]))
		if offset+length > len(raw) {
			t.Fatal("denyfirst.ico points past its end")
		}
		if w, h, _ := pngShape(t, "denyfirst.ico", raw[offset:offset+length]); w != size || h != size {
			t.Errorf("denyfirst.ico says %dx%d and holds %dx%d", size, size, w, h)
		}
		sizes = append(sizes, size)
	}
	if fmt.Sprint(sizes) != "[16 32 48]" {
		t.Errorf("denyfirst.ico holds %v, want 16, 32 and 48", sizes)
	}
	if w, h, c := pngShape(t, "denyfirst-touch.png", []byte(asset(t, "assets/denyfirst-touch.png"))); w != 180 || h != 180 || c != 2 {
		t.Errorf("denyfirst-touch.png is %dx%d with colour type %d, want an opaque 180x180", w, h, c)
	}
	// The addresses a browser asks for unprompted answer with the same mark.
	for alias, file := range map[string]string{"/favicon.ico": "assets/denyfirst.ico", "/apple-touch-icon.png": "assets/denyfirst-touch.png", "/favicon.svg": "assets/denyfirst.svg"} {
		if get(t, alias).Body.String() != asset(t, file) {
			t.Errorf("%s is not the mark", alias)
		}
	}
}

// The picture a link preview shows is the size the pages say it is.
func TestTheSocialCardIsTheSizeThePagesState(t *testing.T) {
	w, h, _ := pngShape(t, "og-denyfirst.png", []byte(asset(t, "assets/og-denyfirst.png")))
	if w != 1200 || h != 630 {
		t.Errorf("og-denyfirst.png is %dx%d, and the pages say 1200x630", w, h)
	}
	for path := range pages {
		body := pageBody(t, path)
		for _, want := range []string{`<meta property="og:image:width" content="1200">`, `<meta property="og:image:height" content="630">`, `<meta property="og:image:type" content="image/png">`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not say %s", path, want)
			}
		}
	}
}
