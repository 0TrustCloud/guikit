package guikit

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// TestGMLParserQuotes verifies that the template engine successfully parses 
// single quotes, double quotes, backticks, and literal newlines within attributes or child text.
func TestGMLParserQuotes(t *testing.T) {
	gmlSource := `
	div.container#main:data-attr.'custom-value'(
		script(
			'function init() {
				console.log("Double quotes inside single-quoted script block");
				const template = "backtick value here";
				return \'single escaped quote\';
			}'
		),
		span("Standard double quote tag child"),
		p("Backtick paragraph block tracking")
	)
	`

	parser := NewParser(gmlSource)
	nodes := parser.Parse()

	if len(nodes) == 0 {
		t.Fatal("Parser returned zero root nodes; check for structural tracking fragmentation or scanner failures.")
	}

	rootElement, ok := nodes[0].(Element)
	if !ok {
		t.Fatalf("Expected root node to be an Element structure, received: %T", nodes[0])
	}

	if rootElement.Tag != "div" {
		t.Errorf("Expected root tag to be 'div', found: '%s'", rootElement.Tag)
	}

	if val, ok := rootElement.Attributes["data-attr"]; !ok || val != "custom-value" {
		t.Errorf("Attribute lookup failure or bad quote processing. Expected 'custom-value', captured: '%s'", val)
	}

	evaluatedHTML := rootElement.Eval()

	if !strings.Contains(evaluatedHTML, `console.log("Double quotes inside single-quoted script block")`) {
		t.Error("Single-quoted multiline script contents were corrupted or incorrectly fragmented during lexical pass.")
	}

	if !strings.Contains(evaluatedHTML, `<span>Standard double quote tag child</span>`) {
		t.Error("Double quote text children failed to evaluate clean output structures.")
	}
}

// TestScriptParserQuotes verifies that the controller script engine accurately tracks complex string 
// definitions, multi-line blocks, and statement patterns inside functions/handlers.
func TestScriptParserQuotes(t *testing.T) {
	scriptSource := `
	state {
		infoMessage = 'Initialized string state value via single quote execution tracking'
		sqlSnippet  = "SELECT * FROM accounts WHERE status = 'active'"
		jsPayload   = 'window.addEventListener("load", () => { console.log("Sub-execution block active"); });'
	}

	handle test_click {
		dml.setValue('output-box', "Execution response validation target tracking")
		dml.addClass('output-box', 'active-state-class')
	}
	`

	sc := &ScriptComponent{
		Id:        "test-component",
		State:     make(map[string]interface{}),
		Handlers:  make(map[string]*ScriptBlock),
		Functions: make(map[string]UserFunction),
	}

	parser := NewScriptParser(scriptSource)
	err := parser.Parse(sc, "")
	if err != nil {
		t.Fatalf("Script compilation failure detected inside script engine lex pass: %v", err)
	}

	if sc.State["infoMessage"] != "Initialized string state value via single quote execution tracking" {
		t.Errorf("Single quote scalar extraction missing or corrupted: %v", sc.State["infoMessage"])
	}

	if sc.State["sqlSnippet"] != "SELECT * FROM accounts WHERE status = 'active'" {
		t.Errorf("Double quote string wrapper containing inner single quotes broke boundary logic: %v", sc.State["sqlSnippet"])
	}

	handler, exists := sc.Handlers["test_click"]
	if !exists {
		t.Fatal("Target event handler structure 'test_click' was not captured by the script parser engine.")
	}

	if len(handler.Statements) != 2 {
		t.Fatalf("Expected exactly 2 functional statements inside event handler, processed: %d", len(handler.Statements))
	}
}

// TestGMLVirtualFSImports sets up a mock in-memory virtual filesystem to test
// layout importing, script integration, and placeholder element evaluation.
func TestGMLVirtualFSImports(t *testing.T) {
	AppFS = fstest.MapFS{
		"layouts/base.gml": &fstest.MapFile{
			Data: []byte(`html( body( main( slot() ) ) )`),
		},
		"views/dashboard.gml": &fstest.MapFile{
			Data: []byte(`wrapper("layouts/base.gml", div.dashboard-view( h1("Dashboard Frame Component Layer") ) )`),
		},
	}

	content, err := fs.ReadFile(AppFS, "views/dashboard.gml")
	if err != nil {
		t.Fatalf("Virtual file system initialization lookup fault: %v", err)
	}

	parser := NewParser(string(content))
	nodes := parser.Parse()

	if len(nodes) == 0 {
		t.Fatal("Failed to extract nodes from layout root view target descriptor.")
	}

	outputHTML := nodes[0].Eval()
	expectedFragment := `<main><div class="dashboard-view"><h1>Dashboard Frame Component Layer</h1></div></main>`

	if !strings.Contains(strings.ReplaceAll(strings.ReplaceAll(outputHTML, "\n", ""), " ", ""), strings.ReplaceAll(strings.ReplaceAll(expectedFragment, "\n", ""), " ", "")) {
		t.Errorf("Layout insertion node execution mismatch.\nGenerated: %s\nExpected Fragment: %s", outputHTML, expectedFragment)
	}
}
