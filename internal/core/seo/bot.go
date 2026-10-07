package seo

import (
	"net/http"
	"strings"
)

// botSubstrings contains lowercase substrings that identify search engine
// crawlers, AI/LLM agents, link-preview bots, and programmatic HTTP libraries.
var botSubstrings = []string{
	// Search engine crawlers
	"googlebot",
	"bingbot",
	"bingpreview",
	"yandex",
	"baiduspider",
	"duckduckbot",
	"sogou",
	"exabot",
	"slurp",
	"seznambot",
	"ia_archiver",

	// AI & LLM crawlers / agents
	"perplexity",
	"gptbot",
	"chatgpt-user",
	"oai-searchbot",
	"claudebot",
	"claude-web",
	"anthropic",
	"ccbot",
	"diffbot",
	"bytespider",
	"applebot",
	"cohere-ai",
	"meta-externalagent",

	// Social media & link preview bots
	"twitterbot",
	"facebookexternalhit",
	"facebot",
	"linkedinbot",
	"slackbot",
	"telegrambot",
	"discordbot",
	"whatsapp",
	"pinterest",
	"skypeuripreview",
	"vkshare",
	"redditbot",

	// CLI & programmatic HTTP clients
	"httpie",
	"go-http-client",
	"python-requests",
	"python-urllib",
	"aiohttp",
	"httpx",
	"node-fetch",
	"axios/",
	"undici",
	"postmanruntime",
	"insomnia",

	// Generic crawler tokens
	"bot",
	"crawler",
	"spider",
	"scraper",
	"archiver",
}

// cliPrefixes contains lowercase prefixes for CLI tools and terminal browsers.
var cliPrefixes = []string{
	"curl",
	"wget",
	"httpie",
	"lynx",
	"w3m",
	"links",
	"axios",
}

// IsBot inspects a User-Agent string and returns true if it matches
// known search engines, AI/LLM crawlers, link-preview scrapers, or CLI tools.
func IsBot(userAgent string) bool {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return false
	}

	for _, prefix := range cliPrefixes {
		if strings.HasPrefix(ua, prefix) {
			return true
		}
	}

	for _, token := range botSubstrings {
		if strings.Contains(ua, token) {
			return true
		}
	}

	return false
}

// ShouldServeSSR determines whether an incoming HTTP request should receive
// server-rendered markdown content. It checks:
//  1. Explicit query parameter "?ssr=1" or "?ssr=true" (forces SSR delivery)
//     and "?ssr=0" or "?ssr=false" (forces clean SPA shell).
//  2. Fallback to IsBot(req.UserAgent()).
func ShouldServeSSR(req *http.Request) bool {
	if req == nil {
		return false
	}

	if req.URL != nil {
		switch strings.ToLower(strings.TrimSpace(req.URL.Query().Get("ssr"))) {
		case "1", "true", "yes":
			return true
		case "0", "false", "no":
			return false
		}
	}

	return IsBot(req.UserAgent())
}
