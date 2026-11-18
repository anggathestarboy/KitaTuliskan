package forms

// Port Support/BardHtml.php: HTML field bard dari pengunjung anonim tidak dipercaya — hanya tag toolbar Bard yang lolos,
// atribut minimal, tanpa URL javascript:.

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var allowedTags = map[string][]string{
	"p": {}, "br": {}, "strong": {}, "b": {}, "em": {}, "i": {}, "u": {}, "s": {},
	"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
	"ul": {}, "ol": {}, "li": {}, "blockquote": {},
	"a": {"href"}, "img": {"src", "alt"},
}
var droppedTags = map[string]bool{"script": true, "style": true, "iframe": true, "object": true, "embed": true, "template": true, "noscript": true}
var safeLink = regexp.MustCompile(`(?i)^(https?://|mailto:|/[^/\\]|/$|#)`)
var safeImage = regexp.MustCompile(`(?i)^(https?://|/[^/\\]|/$)`)

func Sanitize(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	root := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := xhtml.ParseFragment(strings.NewReader(raw), root)
	if err != nil {
		return ""
	}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	clean(root)
	var b bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		xhtml.Render(&b, c)
	}
	return b.String()
}

func clean(parent *xhtml.Node) {
	for child := parent.FirstChild; child != nil; {
		next := child.NextSibling
		switch {
		case child.Type == xhtml.TextNode:
		case child.Type != xhtml.ElementNode || droppedTags[child.Data]:
			parent.RemoveChild(child)
		default:
			clean(child)
			allowed, ok := allowedTags[child.Data]
			if !ok { // unwrap: naikkan anak-anaknya, buang tag-nya
				for child.FirstChild != nil {
					gc := child.FirstChild
					child.RemoveChild(gc)
					parent.InsertBefore(gc, child)
				}
				parent.RemoveChild(child)
			} else {
				cleanAttrs(child, allowed)
			}
		}
		child = next
	}
}

func cleanAttrs(el *xhtml.Node, allowed []string) {
	kept := el.Attr[:0]
	for _, a := range el.Attr {
		name := strings.ToLower(a.Key)
		ok := false
		for _, al := range allowed {
			ok = ok || al == name
		}
		if ok && (name == "href" || name == "src") {
			re := safeLink
			if el.Data == "img" {
				re = safeImage
			}
			ok = re.MatchString(strings.TrimSpace(a.Val))
		}
		if ok {
			kept = append(kept, a)
		}
	}
	el.Attr = kept
	if el.Data == "a" {
		for _, a := range el.Attr {
			if a.Key == "href" {
				el.Attr = append(el.Attr, xhtml.Attribute{Key: "rel", Val: "noopener noreferrer"}, xhtml.Attribute{Key: "target", Val: "_blank"})
				break
			}
		}
	}
}

var blockEnd = regexp.MustCompile(`(?i)</(p|li|h[1-6]|blockquote)>|<br\s*/?>`)
var anyTag = regexp.MustCompile(`<[^>]*>`)
var spaces = regexp.MustCompile(`\s+`)

// ToText: versi teks polos (sel tabel, export TXT); akhir blok jadi spasi.
func ToText(raw string) string {
	s := blockEnd.ReplaceAllString(raw, " ")
	return strings.TrimSpace(spaces.ReplaceAllString(html.UnescapeString(anyTag.ReplaceAllString(s, "")), " "))
}

// Excerpt = Str::limit(toText, 80).
func Excerpt(raw string) string {
	t := []rune(ToText(raw))
	if len(t) <= 80 {
		return string(t)
	}
	return strings.TrimRight(string(t[:80]), " ") + "..."
}
