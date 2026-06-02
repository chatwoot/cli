package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/importer"
	"github.com/chatwoot/cli/internal/prompt"
	"github.com/chatwoot/cli/internal/sdk"
)

type fakeSource struct {
	hcs    []importer.HelpCenter
	corpus *importer.Corpus
}

func (f *fakeSource) Name() string { return "intercom" }
func (f *fakeSource) Validate(context.Context) (string, error) {
	return "ws1", nil
}
func (f *fakeSource) ListHelpCenters(context.Context) ([]importer.HelpCenter, error) {
	return f.hcs, nil
}
func (f *fakeSource) Scan(context.Context, string) (*importer.Corpus, error) {
	return f.corpus, nil
}

type fakeImportSink struct {
	ensureCalls int
	cats        int
	arts        int
}

func (f *fakeImportSink) EnsurePortal(t importer.PortalTarget, _ []string) (importer.PortalRef, bool, error) {
	f.ensureCalls++
	return importer.PortalRef{Slug: t.Slug(), ID: 1}, true, nil
}
func (f *fakeImportSink) CreateCategory(_ string, req sdk.CreateCategoryRequest) (sdk.HelpCenterCategory, error) {
	f.cats++
	return sdk.HelpCenterCategory{ID: f.cats, Slug: req.Slug, Locale: req.Locale}, nil
}
func (f *fakeImportSink) CreateArticle(_ string, _ sdk.CreateArticleRequest) (sdk.HelpCenterArticle, error) {
	f.arts++
	return sdk.HelpCenterArticle{ID: 100 + f.arts}, nil
}
func (f *fakeImportSink) UploadImage(url string) (string, error) { return url, nil }

func testCorpus() *importer.Corpus {
	return &importer.Corpus{
		HelpCenter:  importer.HelpCenter{ID: "hc1", Name: "Acme Support", DefaultLocale: "en"},
		Collections: []importer.Collection{{ID: "c1", Names: map[string]string{"en": "Getting Started"}}},
		Articles: []importer.Article{
			{ID: "a1", CollectionID: "c1", DefaultLocale: "en",
				Variants: map[string]importer.ArticleVariant{"en": {Locale: "en", Title: "Setup", BodyHTML: "<p>x</p>"}}},
		},
		Authors: map[string]importer.Author{},
		Locales: []string{"en"},
	}
}

func newDeps(input string, sink importer.Sink, stateDir string) (intercomImportDeps, *bytes.Buffer) {
	var out bytes.Buffer
	deps := intercomImportDeps{
		source:          &fakeSource{hcs: []importer.HelpCenter{{ID: "hc1", Name: "Acme Support", DefaultLocale: "en"}}, corpus: testCorpus()},
		prompter:        prompt.NewTermPrompter(strings.NewReader(input), &out),
		sink:            sink,
		existingPortals: nil,
		agentEmails:     map[string]int{},
		fallbackAuthor:  9,
		stateDir:        stateDir,
		out:             &out,
	}
	return deps, &out
}

func TestRunIntercomImportHappyPath(t *testing.T) {
	sink := &fakeImportSink{}
	// Single HC (auto), target=create new (1), name default, slug default,
	// single locale (auto, no prompt), confirm yes.
	deps, out := newDeps("1\n\n\ny\n", sink, t.TempDir())

	res, err := runIntercomImport(context.Background(), deps)
	if err != nil {
		t.Fatalf("runIntercomImport: %v", err)
	}
	if res == nil {
		t.Fatal("expected a result")
	}
	if sink.ensureCalls != 1 {
		t.Errorf("EnsurePortal calls = %d, want 1", sink.ensureCalls)
	}
	if sink.cats != 1 || sink.arts != 1 {
		t.Errorf("created cats=%d arts=%d, want 1/1", sink.cats, sink.arts)
	}
	if res.PortalSlug != "acme-support" {
		t.Errorf("portal slug = %q, want acme-support", res.PortalSlug)
	}
	if !strings.Contains(out.String(), "Done.") {
		t.Errorf("summary not printed:\n%s", out.String())
	}
}

func TestRunIntercomImportDeclineWritesNothing(t *testing.T) {
	sink := &fakeImportSink{}
	// Same as happy path but decline at the confirm prompt.
	deps, out := newDeps("1\n\n\nn\n", sink, t.TempDir())

	res, err := runIntercomImport(context.Background(), deps)
	if err != nil {
		t.Fatalf("runIntercomImport: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result on decline, got %#v", res)
	}
	if sink.ensureCalls != 0 || sink.cats != 0 || sink.arts != 0 {
		t.Errorf("nothing should be written on decline: ensure=%d cats=%d arts=%d", sink.ensureCalls, sink.cats, sink.arts)
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected abort message:\n%s", out.String())
	}
}
