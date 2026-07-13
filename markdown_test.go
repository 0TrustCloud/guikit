package guikit

import (
	"strings"
	"testing"
)

func TestFormatMarkdownGIFImage(t *testing.T) {
	out := formatMarkdown("![lol](https://0trust.social/c/abc123deadbeef)")
	if !containsAll(out, `<img class="md-gif"`, "0trust.social/c/abc123deadbeef", `alt="lol"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFormatMarkdownGiphyMP4(t *testing.T) {
	url := "https://media0.giphy.com/media/v1.Y2lkPTM4NDBkYjM1Njl2Mnc2YnNpa3FoMDVlYmlsdmxrN2JkNnp6cXAxemtwM3E0Nm95eCZlcD12MV9naWZzX3NlYXJjaCZjdD1n/Ev477g37MJORyOWfdG/giphy.mp4"
	out := formatMarkdown("![cat](" + url + ")")
	if !containsAll(out, `<video class="md-gif"`, url, `autoplay loop muted playsinline`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFormatMarkdownBareGIFURL(t *testing.T) {
	out := formatMarkdown("https://0trust.social/c/abc123deadbeef")
	if !containsAll(out, `<img class="md-gif"`, "0trust.social/c/abc123deadbeef") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFormatMarkdownDecodesEntities(t *testing.T) {
	out := formatMarkdown("As The Gambia&#039;s president meets Trump")
	if !strings.Contains(out, "Gambia's") && !strings.Contains(out, "Gambia&#39;s") {
		t.Fatalf("expected decoded apostrophe, got: %s", out)
	}
	if strings.Contains(out, "&#039;") || strings.Contains(out, "&amp;#") {
		t.Fatalf("raw entity should be decoded, got: %s", out)
	}
}

func TestFormatMarkdownHashtagSkipsNumericEntities(t *testing.T) {
	out := formatMarkdown("They&#039;re fine #news today")
	if strings.Contains(out, "#039") {
		t.Fatalf("numeric entity fragment should not become hashtag: %s", out)
	}
	if !strings.Contains(out, "#news") {
		t.Fatalf("expected real hashtag link: %s", out)
	}
}

func TestFormatMarkdownBasics(t *testing.T) {
	src := "# Title\n\nHello **world** and *italic*.\n\n- one\n- two\n\n[link](https://example.com)\n\n```go\nfmt.Println(1)\n```\n\n> quote me\n\n`code`"
	out := formatMarkdown(src)
	for _, want := range []string{
		"<h1>Title</h1>",
		"<strong>world</strong>",
		"<em>italic</em>",
		"<ul>",
		"<li>one</li>",
		"<li>two</li>",
		`href="https://example.com"`,
		"<pre><code",
		"fmt.Println(1)",
		"<blockquote>quote me</blockquote>",
		"<code>code</code>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}