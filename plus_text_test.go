package guikit

import "testing"

func TestPlusInQuotedText(t *testing.T) {
	src := `button.btn("+ Sub-page")`
	got := NewParser(src).Parse()[0].Eval()
	want := `<button class="btn">+ Sub-page</button>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPlusOnlyButton(t *testing.T) {
	src := `button.nav-add-btn:type."button":title."New page"("+")`
	got := NewParser(src).Parse()[0].Eval()
	want := `<button class="nav-add-btn" type="button" title="New page">+</button>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPlusAsSpanChild(t *testing.T) {
	src := `button.btn(span("+ "), span("Sub-page"))`
	got := NewParser(src).Parse()[0].Eval()
	want := `<button class="btn"><span>+ </span><span>Sub-page</span></button>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPlusInUnquotedText(t *testing.T) {
	src := `button.btn(+ Sub-page)`
	got := NewParser(src).Parse()[0].Eval()
	t.Logf("unquoted plus render: %q", got)
	if got == `<button class="btn">+ Sub-page</button>` {
		t.Fatal("unexpected: unquoted should not render as plain text")
	}
}