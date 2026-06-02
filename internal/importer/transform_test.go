package importer

import (
	"fmt"
	"strings"
	"testing"
)

func TestTransformSwapsImageSrc(t *testing.T) {
	upload := func(src string) (string, error) {
		return "https://app/rails/active_storage/blobs/x/img.png", nil
	}
	body := `<p>Hi</p><img src="https://cdn.intercom.io/a.png" alt="a">`
	res := TransformBody(body, upload, DefaultEmbedRegistry())
	if res.ImagesSwapped != 1 || res.ImagesFailed != 0 {
		t.Fatalf("swapped=%d failed=%d, want 1/0", res.ImagesSwapped, res.ImagesFailed)
	}
	if !strings.Contains(res.HTML, "rails/active_storage") {
		t.Errorf("src not swapped: %q", res.HTML)
	}
	if strings.Contains(res.HTML, "cdn.intercom.io") {
		t.Errorf("original src still present: %q", res.HTML)
	}
}

func TestTransformKeepsSrcOnUploadFailure(t *testing.T) {
	upload := func(src string) (string, error) { return "", fmt.Errorf("boom") }
	body := `<img src="https://cdn.intercom.io/a.png">`
	res := TransformBody(body, upload, DefaultEmbedRegistry())
	if res.ImagesFailed != 1 || res.ImagesSwapped != 0 {
		t.Fatalf("swapped=%d failed=%d, want 0/1", res.ImagesSwapped, res.ImagesFailed)
	}
	if !strings.Contains(res.HTML, "cdn.intercom.io") {
		t.Errorf("original src should be kept: %q", res.HTML)
	}
}

func TestTransformSkipsDataURIAndExistingChatwoot(t *testing.T) {
	calls := 0
	upload := func(src string) (string, error) { calls++; return "x", nil }
	body := `<img src="data:image/png;base64,AAAA"><img src="https://app/rails/active_storage/blobs/y/b.png">`
	res := TransformBody(body, upload, DefaultEmbedRegistry())
	if calls != 0 {
		t.Errorf("upload called %d times, want 0", calls)
	}
	if res.ImagesSwapped != 0 || res.ImagesFailed != 0 {
		t.Errorf("counts swapped=%d failed=%d, want 0/0", res.ImagesSwapped, res.ImagesFailed)
	}
}

func TestTransformRewritesIframeEmbedToBareURL(t *testing.T) {
	body := `<div class="wrap"><iframe src="https://www.youtube.com/embed/abc123" frameborder="0"></iframe></div>`
	res := TransformBody(body, nil, DefaultEmbedRegistry())
	if res.EmbedsRewritten != 1 {
		t.Fatalf("embeds=%d, want 1", res.EmbedsRewritten)
	}
	if strings.Contains(res.HTML, "<iframe") {
		t.Errorf("iframe should be removed: %q", res.HTML)
	}
	if !strings.Contains(res.HTML, "https://www.youtube.com/watch?v=abc123") {
		t.Errorf("bare URL missing: %q", res.HTML)
	}
	// The wrapping div contained only the iframe, so it should be replaced too.
	if strings.Contains(res.HTML, `class="wrap"`) {
		t.Errorf("wrapper-only div should be replaced: %q", res.HTML)
	}
}

func TestTransformLeavesUnknownIframe(t *testing.T) {
	body := `<iframe src="https://example.com/widget"></iframe>`
	res := TransformBody(body, nil, DefaultEmbedRegistry())
	if res.EmbedsRewritten != 0 {
		t.Fatalf("embeds=%d, want 0", res.EmbedsRewritten)
	}
	if !strings.Contains(res.HTML, "<iframe") {
		t.Errorf("unknown iframe should be left as-is: %q", res.HTML)
	}
}

func TestTransformMalformedHTMLDoesNotPanic(t *testing.T) {
	body := `<p>unclosed <img src="https://cdn.intercom.io/a.png"`
	res := TransformBody(body, func(string) (string, error) { return "ok", nil }, DefaultEmbedRegistry())
	// Should not panic; result is best-effort.
	_ = res.HTML
}

func TestTransformEmptyBody(t *testing.T) {
	res := TransformBody("", nil, DefaultEmbedRegistry())
	if res.HTML != "" {
		t.Errorf("empty body should stay empty, got %q", res.HTML)
	}
}
