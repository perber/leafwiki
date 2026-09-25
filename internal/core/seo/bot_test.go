package seo_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/perber/wiki/internal/core/seo"
)

func TestIsBot_KnownCrawlersAndBots(t *testing.T) {
	bots := []string{
		// Search engines
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Googlebot-Image/1.0",
		"Googlebot-News",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"Mozilla/5.0 (compatible; BingPreview/1.0b; +http://www.bing.com/bingbot.htm)",
		"Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)",
		"Baiduspider+(+http://www.baidu.com/search/spider.htm)",
		"DuckDuckBot/1.0; (+http://duckduckgo.com/duckduckbot.html)",
		"Sogou web spider/4.0(+http://www.sogou.com/docs/help/webmasters.htm#07)",
		"Mozilla/5.0 (compatible; Exabot/3.0; +http://www.exabot.com/go/robot)",
		"Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)",
		"SeznamBot/3.2 (+http://napoveda.seznam.cz/en/seznambot-intro/)",
		"ia_archiver (+http://www.alexa.com/site/help/webmasters; crawler@alexa.com)",

		// AI & LLM crawlers
		"Mozilla/5.0 (compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)",
		"Perplexity-User/1.0",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.0; +https://openai.com/gptbot)",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; OAI-SearchBot/1.0; +https://openai.com/searchbot",
		"Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)",
		"Mozilla/5.0 (compatible; Claude-Web/1.0; +https://www.anthropic.com)",
		"anthropic-ai/1.0",
		"CCBot/2.0 (https://commoncrawl.org/faq/)",
		"Diffbot/0.1; (+http://www.diffbot.com)",
		"Bytespider",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/go/applebot)",
		"cohere-ai",
		"Meta-ExternalAgent/1.0",

		// CLI & programmatic HTTP clients
		"curl/8.4.0",
		"curl",
		"Wget/1.21.4",
		"Wget",
		"HTTPie/3.2.2",
		"httpie",
		"Go-http-client/1.1",
		"python-requests/2.31.0",
		"Python-urllib/3.10",
		"aiohttp/3.8.5",
		"HTTPX/0.25.0",
		"node-fetch/1.0",
		"axios/1.6.0",
		"undici",
		"PostmanRuntime/7.32.3",
		"insomnia/8.4.5",

		// Terminal browsers
		"Lynx/2.8.9rel.1 libwww-FM/2.14 SSL-MM/1.4.1 OpenSSL/1.1.1",
		"w3m/0.5.3",
		"Links (2.29; Linux x86_64; GNU/Linux)",

		// Social & link preview bots
		"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
		"Twitterbot/1.0",
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		"Facebot",
		"LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)",
		"TelegramBot (like TwitterBot)",
		"Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)",
		"WhatsApp/2.21.12.21 A",
		"Pinterest/0.2 (+http://www.pinterest.com/bot.html)",
		"SkypeUriPreview Preview/0.5",
		"vkShare; http://vk.com/share.php",
		"RedditBot/1.0",

		// Generic crawler tokens
		"SomeCustomCrawler/1.0",
		"SiteSpider/2.0",
		"FastWebScraper/1.1",
	}

	for _, ua := range bots {
		t.Run(ua, func(t *testing.T) {
			if !seo.IsBot(ua) {
				t.Fatalf("expected IsBot(%q) to be true, got false", ua)
			}
		})
	}
}

func TestIsBot_HumanBrowsers(t *testing.T) {
	browsers := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:123.0) Gecko/20100101 Firefox/123.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_3_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.6261.64 Mobile Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 OPR/107.0.0.0",
		"Mozilla/5.0 (Linux; Android 13; SM-S908B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.5481.153 Mobile Safari/537.36 SamsungBrowser/20.0",
		"",
		"   ",
	}

	for _, ua := range browsers {
		t.Run(ua, func(t *testing.T) {
			if seo.IsBot(ua) {
				t.Fatalf("expected IsBot(%q) to be false, got true", ua)
			}
		})
	}
}

func TestShouldServeSSR(t *testing.T) {
	chromeUA := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	botUA := "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	curlUA := "curl/8.4.0"

	t.Run("nil request", func(t *testing.T) {
		if seo.ShouldServeSSR(nil) {
			t.Fatal("expected false for nil request")
		}
	})

	t.Run("standard browser defaults to false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/intro", nil)
		req.Header.Set("User-Agent", chromeUA)
		if seo.ShouldServeSSR(req) {
			t.Fatal("expected false for standard browser request")
		}
	})

	t.Run("bot user agent defaults to true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/intro", nil)
		req.Header.Set("User-Agent", botUA)
		if !seo.ShouldServeSSR(req) {
			t.Fatal("expected true for Googlebot request")
		}
	})

	t.Run("curl defaults to true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/intro", nil)
		req.Header.Set("User-Agent", curlUA)
		if !seo.ShouldServeSSR(req) {
			t.Fatal("expected true for curl request")
		}
	})

	t.Run("query override ssr=1 forces SSR for browser", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/intro?ssr=1", nil)
		req.Header.Set("User-Agent", chromeUA)
		if !seo.ShouldServeSSR(req) {
			t.Fatal("expected true with ?ssr=1 override")
		}
	})

	t.Run("query override ssr=true forces SSR for browser", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/intro?ssr=true", nil)
		req.Header.Set("User-Agent", chromeUA)
		if !seo.ShouldServeSSR(req) {
			t.Fatal("expected true with ?ssr=true override")
		}
	})

	t.Run("query override ssr=0 disables SSR for bot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/intro?ssr=0", nil)
		req.Header.Set("User-Agent", botUA)
		if seo.ShouldServeSSR(req) {
			t.Fatal("expected false with ?ssr=0 override")
		}
	})

	t.Run("query override ssr=false disables SSR for bot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/intro?ssr=false", nil)
		req.Header.Set("User-Agent", botUA)
		if seo.ShouldServeSSR(req) {
			t.Fatal("expected false with ?ssr=false override")
		}
	})
}
