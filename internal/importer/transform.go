package importer

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// ImageUploader re-hosts a remote image URL and returns the new hosted URL.
// It is injected so the transform stays pure/testable; in production it wraps
// the Chatwoot upload endpoint.
type ImageUploader func(srcURL string) (newURL string, err error)

// TransformResult is the rewritten body plus per-article counters.
type TransformResult struct {
	HTML            string
	ImagesSwapped   int
	ImagesFailed    int
	EmbedsRewritten int
}

// TransformBody rewrites an HTML article body: <img> sources are re-hosted via
// upload (best-effort — failures keep the original src), and recognized
// provider <iframe>s are replaced with the bare canonical URL on their own line
// so Chatwoot expands them into embeds at render time. If upload is nil, images
// are left untouched. Malformed HTML is returned unchanged.
func TransformBody(body string, upload ImageUploader, reg EmbedRegistry) TransformResult {
	res := TransformResult{HTML: body}
	if strings.TrimSpace(body) == "" {
		return res
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return res
	}

	type embedRepl struct {
		target *html.Node
		url    string
	}
	var embeds []embedRepl

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "img":
				if upload != nil {
					rewriteImg(n, upload, &res)
				}
			case "iframe":
				if src, ok := attr(n, "src"); ok {
					if canon, matched := reg.Match(src); matched {
						embeds = append(embeds, embedRepl{target: embedReplaceTarget(n), url: canon})
						res.EmbedsRewritten++
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	for _, e := range embeds {
		replaceWithBareURL(e.target, e.url)
	}

	bodyNode := findBody(doc)
	var buf bytes.Buffer
	if bodyNode != nil {
		for c := bodyNode.FirstChild; c != nil; c = c.NextSibling {
			_ = html.Render(&buf, c)
		}
	} else {
		_ = html.Render(&buf, doc)
	}
	res.HTML = buf.String()
	return res
}

func rewriteImg(n *html.Node, upload ImageUploader, res *TransformResult) {
	for i := range n.Attr {
		if !strings.EqualFold(n.Attr[i].Key, "src") {
			continue
		}
		src := n.Attr[i].Val
		if !shouldUploadImage(src) {
			return
		}
		newURL, err := upload(src)
		if err != nil || newURL == "" {
			res.ImagesFailed++
			return
		}
		n.Attr[i].Val = newURL
		res.ImagesSwapped++
		return
	}
}

func shouldUploadImage(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" {
		return false
	}
	lower := strings.ToLower(src)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false // skip data: URIs and relative paths
	}
	if strings.Contains(lower, "/rails/active_storage/") {
		return false // already hosted by Chatwoot
	}
	return true
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}

// embedReplaceTarget climbs from an iframe to the outermost wrapper element
// (div/p/figure/span) that contains only this iframe, so the bare URL lands as
// an isolated block rather than buried inside a wrapper.
func embedReplaceTarget(n *html.Node) *html.Node {
	target := n
	for {
		p := target.Parent
		if p == nil || p.Type != html.ElementNode {
			break
		}
		switch p.Data {
		case "div", "p", "figure", "span":
		default:
			return target
		}
		if !onlyElementChild(p, target) {
			break
		}
		target = p
	}
	return target
}

func onlyElementChild(parent, child *html.Node) bool {
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		if c == child {
			continue
		}
		if c.Type == html.ElementNode {
			return false
		}
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) != "" {
			return false
		}
	}
	return true
}

func replaceWithBareURL(n *html.Node, url string) {
	parent := n.Parent
	if parent == nil {
		return
	}
	text := &html.Node{Type: html.TextNode, Data: "\n\n" + url + "\n\n"}
	parent.InsertBefore(text, n)
	parent.RemoveChild(n)
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b := findBody(c); b != nil {
			return b
		}
	}
	return nil
}
