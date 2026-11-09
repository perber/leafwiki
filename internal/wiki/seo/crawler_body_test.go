package seo_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/perber/wiki/internal/render"
	"github.com/perber/wiki/internal/wiki/seo"
)

const (
	googlebotUA = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	chromeUA    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

// fakeCrawlerBodyReader returns fixed child pages per page ID and resolves
// wikilinks from a map; block, when set, holds every resolve until closed.
type fakeCrawlerBodyReader struct {
	mu       sync.Mutex
	children map[string][]seo.ChildPage
	links    map[string]string
	err      error
	reads    int
	entered  chan struct{}
	block    chan struct{}
}

func (f *fakeCrawlerBodyReader) ChildPages(_ context.Context, pageID string) ([]seo.ChildPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.children[pageID], f.err
}

func (f *fakeCrawlerBodyReader) ResolveWikiLink(_ context.Context, target string) (string, bool) {
	if f.block != nil {
		f.entered <- struct{}{}
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	routePath, ok := f.links[target]
	return routePath, ok
}

func (f *fakeCrawlerBodyReader) setLink(target, routePath string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links[target] = routePath
}

// lockedPageReader serializes reads of a fakePageReader for concurrent tests.
type lockedPageReader struct {
	mu    sync.Mutex
	inner *fakePageReader
}

func (l *lockedPageReader) PageByRoutePath(ctx context.Context, routePath string) (*seo.Page, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inner.PageByRoutePath(ctx, routePath)
}

// countingRender wraps render.RenderHTML and counts the calls.
type countingRender struct {
	mu    sync.Mutex
	calls int
}

func (c *countingRender) render(content string, opt render.Options) (string, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return render.RenderHTML(content, opt)
}

func (c *countingRender) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func crawlerBodyUseCase(pages seo.PageReader, reader *fakeCrawlerBodyReader) (*seo.GetCrawlerBodyUseCase, *countingRender) {
	uc := seo.NewGetCrawlerBodyUseCase(pages, reader)
	counter := &countingRender{}
	seo.SetRenderFunc(uc, counter.render)
	return uc, counter
}

func crawlerInput(basePath, routePath string) seo.GetCrawlerBodyInput {
	return seo.GetCrawlerBodyInput{RoutePath: routePath, BasePath: basePath, UserAgent: googlebotUA}
}

func setupPage() seo.Page {
	return seo.Page{ID: "setup-id", Title: "Setup", Body: "Install it."}
}

func TestGetCrawlerBodyUseCase_UserAgent_OnlySearchEnginesGetABody(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		wantBody bool
	}{
		{name: "Googlebot desktop", ua: googlebotUA, wantBody: true},
		{name: "Googlebot smartphone", ua: "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", wantBody: true},
		{name: "Bingbot", ua: "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", wantBody: true},
		{name: "YandexBot", ua: "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)", wantBody: true},
		{name: "Applebot", ua: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/go/applebot)", wantBody: true},
		{name: "DuckDuckBot", ua: "DuckDuckBot/1.1; (+http://duckduckgo.com/duckduckbot.html)", wantBody: true},
		{name: "Baiduspider", ua: "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)", wantBody: true},
		{name: "Google-InspectionTool (URL Inspection, Rich Results Test)", ua: "Mozilla/5.0 (compatible; Google-InspectionTool/1.0;)", wantBody: true},
		{name: "upper case", ua: "GOOGLEBOT", wantBody: true},
		{name: "Chrome", ua: chromeUA},
		{name: "Edge", ua: chromeUA + " Edg/130.0.0.0"},
		{name: "Firefox", ua: "Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0"},
		{name: "empty", ua: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pages := pageAt("docs/setup", setupPage())
			uc, _ := crawlerBodyUseCase(pages, &fakeCrawlerBodyReader{})

			out, err := uc.Execute(context.Background(), seo.GetCrawlerBodyInput{RoutePath: "/docs/setup", UserAgent: tc.ua})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if (out != nil) != tc.wantBody {
				t.Fatalf("body = %+v, want body: %v", out, tc.wantBody)
			}
			if !tc.wantBody && pages.reads != 0 {
				t.Errorf("reads = %d, want none for a browser", pages.reads)
			}
		})
	}
}

func TestGetCrawlerBodyUseCase_Page_BuildsArticleWithLinksAndChildren(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		page     seo.Page
		children []seo.ChildPage
		want     string
	}{
		{
			name: "page without children",
			page: setupPage(),
			want: `<article><h1>Setup</h1><p>Install it.</p>` + "\n" + `</article>`,
		},
		{
			name:     "title and child titles escaped, child hrefs escaped below the base path",
			basePath: "/wiki",
			page:     seo.Page{ID: "setup-id", Title: `<Set & "up">`, Body: "Read [[Install]]."},
			children: []seo.ChildPage{
				{Title: "<b>Linux</b>", RoutePath: "docs/setup/linux"},
				{Title: "Mac", RoutePath: "docs/setup/mac os"},
				{Title: "broken route is skipped", RoutePath: "docs//x"},
			},
			want: `<article><h1>&lt;Set &amp; &#34;up&#34;&gt;</h1><p>Read <a href="/wiki/docs/install">Install</a>.</p>` + "\n" + `</article>` +
				`<nav><ul><li><a href="/wiki/docs/setup/linux">&lt;b&gt;Linux&lt;/b&gt;</a></li><li><a href="/wiki/docs/setup/mac%20os">Mac</a></li></ul></nav>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakeCrawlerBodyReader{
				children: map[string][]seo.ChildPage{"setup-id": tc.children},
				links:    map[string]string{"Install": "docs/install"},
			}
			uc, _ := crawlerBodyUseCase(pageAt("docs/setup", tc.page), reader)

			out, err := uc.Execute(context.Background(), crawlerInput(tc.basePath, "/docs/setup/"))
			if err != nil || out == nil {
				t.Fatalf("Execute = %+v, %v, want a body", out, err)
			}
			if out.HTML != tc.want {
				t.Errorf("HTML =\n%s\nwant\n%s", out.HTML, tc.want)
			}
		})
	}
}

func TestGetCrawlerBodyUseCase_NoPage_ReturnsNothing(t *testing.T) {
	tests := []struct {
		name      string
		routePath string
		pages     *fakePageReader
		wantReads int
	}{
		{name: "root", routePath: "/", pages: pageAt("docs/setup", setupPage()), wantReads: 0},
		{name: "permalink", routePath: "/p/setup-id", pages: pageAt("p/setup-id", setupPage()), wantReads: 0},
		{name: "unknown page", routePath: "/docs/missing", pages: pageAt("docs/setup", setupPage()), wantReads: 1},
		{name: "tree not loaded", routePath: "/docs/setup", pages: &fakePageReader{notLoaded: true}, wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakeCrawlerBodyReader{}
			uc, counter := crawlerBodyUseCase(tc.pages, reader)

			out, err := uc.Execute(context.Background(), crawlerInput("", tc.routePath))
			if out != nil || err != nil {
				t.Fatalf("Execute = %+v, %v, want nothing", out, err)
			}
			if tc.pages.reads != tc.wantReads || reader.reads != 0 || counter.count() != 0 {
				t.Errorf("page reads = %d (want %d), child reads = %d, renders = %d, want no child read or render",
					tc.pages.reads, tc.wantReads, reader.reads, counter.count())
			}
		})
	}
}

func TestGetCrawlerBodyUseCase_ReadError_IsReturned(t *testing.T) {
	readErr := errors.New("disk on fire")
	tests := []struct {
		name   string
		pages  *fakePageReader
		reader *fakeCrawlerBodyReader
	}{
		{name: "page read", pages: &fakePageReader{err: readErr}, reader: &fakeCrawlerBodyReader{}},
		{name: "child read", pages: pageAt("docs/setup", setupPage()), reader: &fakeCrawlerBodyReader{err: readErr}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc, _ := crawlerBodyUseCase(tc.pages, tc.reader)
			out, err := uc.Execute(context.Background(), crawlerInput("", "/docs/setup"))
			if out != nil || !errors.Is(err, readErr) {
				t.Fatalf("Execute = %+v, %v, want %v", out, err, readErr)
			}
		})
	}
}

// A page that is too complex to render gets no body; the failure is reported
// once and then remembered for this page version, so a crawler can't make
// the server retry the expensive part on every request.
func TestGetCrawlerBodyUseCase_RenderFails_ReportedOnceThenNoBody(t *testing.T) {
	huge := seo.Page{ID: "huge-id", Title: "Huge", Body: strings.Repeat("a", 300<<10)}
	pages := pageAt("huge", huge)
	uc, counter := crawlerBodyUseCase(pages, &fakeCrawlerBodyReader{})

	out, err := uc.Execute(context.Background(), crawlerInput("", "/huge"))
	if out != nil || !errors.Is(err, render.ErrTooComplex) {
		t.Fatalf("first Execute = %+v, %v, want ErrTooComplex", out, err)
	}
	out, err = uc.Execute(context.Background(), crawlerInput("", "/huge"))
	if out != nil || err != nil {
		t.Fatalf("second Execute = %+v, %v, want nothing", out, err)
	}
	if counter.count() != 1 {
		t.Errorf("renders = %d, want 1", counter.count())
	}

	pages.pages["huge"].Body = "Small now."
	if out, err := uc.Execute(context.Background(), crawlerInput("", "/huge")); err != nil || out == nil {
		t.Fatalf("Execute after an edit = %+v, %v, want a body", out, err)
	}
}

// The rendered HTML is cached per page version: the same content, route and
// wikilink targets render once; any of them changing renders again.
func TestGetCrawlerBodyUseCase_RenderCache_PerPageVersion(t *testing.T) {
	pages := &fakePageReader{pages: map[string]*seo.Page{
		"docs/setup": {ID: "setup-id", Title: "Setup", Body: "See [[Install]]."},
		"docs/copy":  {ID: "copy-id", Title: "Copy", Body: "See [[Install]]."},
	}}
	reader := &fakeCrawlerBodyReader{links: map[string]string{"Install": "docs/install"}}
	uc, counter := crawlerBodyUseCase(pages, reader)
	execute := func(routePath, wantLink string) {
		t.Helper()
		out, err := uc.Execute(context.Background(), crawlerInput("", routePath))
		if err != nil || out == nil || !strings.Contains(out.HTML, `href="`+wantLink+`"`) {
			t.Fatalf("Execute(%s) = %+v, %v, want a link to %s", routePath, out, err, wantLink)
		}
	}

	steps := []struct {
		name        string
		change      func()
		routePath   string
		wantLink    string
		wantRenders int
	}{
		{name: "first request renders", routePath: "/docs/setup", wantLink: "/docs/install", wantRenders: 1},
		{name: "same version is cached", routePath: "/docs/setup", wantLink: "/docs/install", wantRenders: 1},
		{name: "same content at another route renders", routePath: "/docs/copy", wantLink: "/docs/install", wantRenders: 2},
		{name: "wikilink target moved renders", change: func() { reader.setLink("Install", "guide/install") }, routePath: "/docs/setup", wantLink: "/guide/install", wantRenders: 3},
		{name: "edited content renders", change: func() { pages.pages["docs/setup"].Body = "See [[Install]] now." }, routePath: "/docs/setup", wantLink: "/guide/install", wantRenders: 4},
	}
	for _, step := range steps {
		if step.change != nil {
			step.change()
		}
		execute(step.routePath, step.wantLink)
		if counter.count() != step.wantRenders {
			t.Fatalf("%s: renders = %d, want %d", step.name, counter.count(), step.wantRenders)
		}
	}
}

// Renders beyond the cap are refused instead of queued, so a flood of
// crawler requests can't pile up render work; the shell is served plain.
func TestGetCrawlerBodyUseCase_ParallelRendersCapped(t *testing.T) {
	reader := &fakeCrawlerBodyReader{
		links:   map[string]string{},
		entered: make(chan struct{}),
		block:   make(chan struct{}),
	}
	pages := &fakePageReader{pages: map[string]*seo.Page{}}
	for i := range seo.MaxParallelRenders + 1 {
		routePath := "page" + string(rune('a'+i))
		pages.pages[routePath] = &seo.Page{ID: routePath, Title: routePath, Body: "[[" + routePath + "]]"}
	}
	uc, _ := crawlerBodyUseCase(&lockedPageReader{inner: pages}, reader)
	var wg sync.WaitGroup
	for i := range seo.MaxParallelRenders {
		routePath := "page" + string(rune('a'+i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := uc.Execute(context.Background(), crawlerInput("", "/"+routePath)); err != nil {
				t.Errorf("blocked render failed: %v", err)
			}
		}()
		<-reader.entered
	}

	last := "page" + string(rune('a'+seo.MaxParallelRenders))
	out, err := uc.Execute(context.Background(), crawlerInput("", "/"+last))
	if out != nil || !errors.Is(err, seo.ErrRendersBusy) {
		t.Errorf("Execute over the cap = %+v, %v, want ErrRendersBusy", out, err)
	}
	close(reader.block)
	wg.Wait()
}

// A render error other than ErrTooComplex may quote page markup; it is
// replaced by an opaque one so it never reaches the log.
func TestGetCrawlerBodyUseCase_RenderError_CarriesNoContent(t *testing.T) {
	uc := seo.NewGetCrawlerBodyUseCase(pageAt("docs/setup", setupPage()), &fakeCrawlerBodyReader{})
	seo.SetRenderFunc(uc, func(string, render.Options) (string, error) {
		return "", errors.New(`disallowed markup <b onclick="SECRET">`)
	})

	out, err := uc.Execute(context.Background(), crawlerInput("", "/docs/setup"))
	if out != nil || err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("Execute = %+v, %v, want an error without the cause", out, err)
	}
}

// The cache holds at most 16 MB of rendered HTML: the oldest pages are
// dropped first, and a page bigger than that is never kept.
func TestGetCrawlerBodyUseCase_RenderCache_Bounded(t *testing.T) {
	const mb = 1 << 20
	sizes := map[string]int{"a": 6 * mb, "b": 6 * mb, "c": 6 * mb, "huge": 17 * mb}
	pages := &fakePageReader{pages: map[string]*seo.Page{}}
	for routePath := range sizes {
		pages.pages[routePath] = &seo.Page{ID: routePath, Title: routePath, Body: routePath}
	}
	uc := seo.NewGetCrawlerBodyUseCase(pages, &fakeCrawlerBodyReader{})
	rendered := map[string]int{}
	seo.SetRenderFunc(uc, func(content string, _ render.Options) (string, error) {
		rendered[content]++
		return strings.Repeat("x", sizes[content]), nil
	})

	for _, routePath := range []string{"a", "b", "c", "a", "c", "huge", "huge"} {
		if out, err := uc.Execute(context.Background(), crawlerInput("", "/"+routePath)); err != nil || out == nil {
			t.Fatalf("Execute(%s) = %v, want a body", routePath, err)
		}
	}
	// c pushed a out (18 MB > 16 MB); a again pushed b out; c stayed cached.
	want := map[string]int{"a": 2, "b": 1, "c": 1, "huge": 2}
	if !reflect.DeepEqual(rendered, want) {
		t.Errorf("renders = %v, want %v", rendered, want)
	}
}

// The cache drops the least recently used page, so a crawl that keeps
// coming back to some pages keeps them cached while others come and go.
func TestGetCrawlerBodyUseCase_RenderCache_DropsLeastRecentlyUsed(t *testing.T) {
	const mb = 1 << 20
	pages := &fakePageReader{pages: map[string]*seo.Page{}}
	for _, routePath := range []string{"a", "b", "c"} {
		pages.pages[routePath] = &seo.Page{ID: routePath, Title: routePath, Body: routePath}
	}
	uc := seo.NewGetCrawlerBodyUseCase(pages, &fakeCrawlerBodyReader{})
	rendered := map[string]int{}
	seo.SetRenderFunc(uc, func(content string, _ render.Options) (string, error) {
		rendered[content]++
		return strings.Repeat("x", 6*mb), nil
	})

	// Three 6 MB pages don't fit into 16 MB. a is used again before c
	// arrives, so c pushes out b, and b then pushes out c.
	for _, routePath := range []string{"a", "b", "a", "c", "a", "b"} {
		if out, err := uc.Execute(context.Background(), crawlerInput("", "/"+routePath)); err != nil || out == nil {
			t.Fatalf("Execute(%s) = %v, want a body", routePath, err)
		}
	}
	want := map[string]int{"a": 1, "b": 2, "c": 1}
	if !reflect.DeepEqual(rendered, want) {
		t.Errorf("renders = %v, want %v", rendered, want)
	}
}

// A client that went away gets no render: the slot is released and nothing
// is cached, so the next request renders.
func TestGetCrawlerBodyUseCase_ClientGone_NoRender(t *testing.T) {
	uc, counter := crawlerBodyUseCase(pageAt("docs/setup", setupPage()), &fakeCrawlerBodyReader{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for range seo.MaxParallelRenders + 1 {
		out, err := uc.Execute(ctx, crawlerInput("", "/docs/setup"))
		if out != nil || err != nil {
			t.Fatalf("Execute with a cancelled context = %+v, %v, want nothing", out, err)
		}
	}
	if counter.count() != 0 {
		t.Fatalf("renders = %d, want none", counter.count())
	}
	if out, err := uc.Execute(context.Background(), crawlerInput("", "/docs/setup")); err != nil || out == nil {
		t.Fatalf("Execute afterwards = %+v, %v, want a body", out, err)
	}
}
