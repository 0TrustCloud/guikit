package guikit

import (
	"testing"
)

// TestGMLParser verifies that the GUI Markup Language lexer and AST parser
// correctly process tags, classes, structural IDs, and child nodes.
func TestGMLParser(t *testing.T) {
	// FIX: Chained attributes must use the modifier prefix (:) in strict GML syntax
	gmlSource := `div.container.flex#main-window(
		h1("System Dashboard"),
		span:id."status-text":class."active"("Online")
	)`

	parser := NewParser(gmlSource)
	nodes := parser.Parse()

	if len(nodes) != 1 {
		t.Fatalf("Expected 1 root node, got %d", len(nodes))
	}

	root, ok := nodes[0].(Element)
	if !ok {
		t.Fatalf("Root node is not an Element type")
	}

	if root.Tag != "div" {
		t.Errorf("Expected tag 'div', got '%s'", root.Tag)
	}

	if root.Attributes["class"] != "container flex" {
		t.Errorf("Expected class 'container flex', got '%s'", root.Attributes["class"])
	}

	if root.Attributes["id"] != "main-window" {
		t.Errorf("Expected id 'main-window', got '%s'", root.Attributes["id"])
	}

	if len(root.Children) != 2 {
		t.Fatalf("Expected 2 child nodes, got %d", len(root.Children))
	}
}

// TestGUIScriptExecution verifies that the GUIScript engine correctly compiles
// initial state boundaries, interprets assignment operations, parses conditionals,
// and mutates internal tracking variables during live event invocations.
func TestGUIScriptExecution(t *testing.T) {
	gsSource := `
	state {
		count: 5
		status: "stabled"
		flags: ["alpha", "beta"]
	}

	handle increment {
		count = count + 1
		if count > 5 {
			status = "throttled"
			dml.addClass("metric-box", "warn-state")
		}
		dml.setValue("display", count)
	}
	`

	sc := &ScriptComponent{
		Id:        "test_component",
		State:     make(map[string]interface{}),
		Handlers:  make(map[string]*ScriptBlock),
		Functions: make(map[string]UserFunction),
	}

	parser := NewScriptParser(gsSource)
	if err := parser.Parse(sc, ""); err != nil {
		t.Fatalf("Failed to parse GUIScript controller: %v", err)
	}

	// Verify Initial State Hydration
	initialCount, ok := sc.State["count"].(int)
	if !ok || initialCount != 5 {
		t.Errorf("Expected initial count of 5, got %v", sc.State["count"])
	}

	initialStatus := sc.State["status"].(string)
	if initialStatus != "stabled" {
		t.Errorf("Expected initial status 'stabled', got '%s'", initialStatus)
	}

	// Trigger the "increment" handle block execution sequence
	mockPayload := make(map[string]string)
	dmlInstructions, err := sc.InvokeEvent("increment", mockPayload)
	if err != nil {
		t.Fatalf("Failed to invoke event handler: %v", err)
	}

	// Verify Mutations after State Execution Sequence
	updatedCount := sc.State["count"].(int)
	if updatedCount != 6 {
		t.Errorf("Expected mutated count to be 6, got %d", updatedCount)
	}

	updatedStatus := sc.State["status"].(string)
	if updatedStatus != "throttled" {
		t.Errorf("Expected conditional branch status modification to be 'throttled', got '%s'", updatedStatus)
	}

	// Verify Atomic Transacted DML Instruction Generation Outputs
	if len(dmlInstructions) != 2 {
		t.Fatalf("Expected 2 transacted DML actions, emitted %d", len(dmlInstructions))
	}

	if dmlInstructions[0].Action != "addClass" || dmlInstructions[0].TargetID != "metric-box" || dmlInstructions[0].Value != "warn-state" {
		t.Errorf("Malformed transacted addClass instruction payload layout: %v", dmlInstructions[0])
	}

	if dmlInstructions[1].Action != "setValue" || dmlInstructions[1].TargetID != "display" || dmlInstructions[1].Value != "6" {
		t.Errorf("Malformed transacted setValue instruction payload layout: %v", dmlInstructions[1])
	}
}

// TestPathResolution checks that dotted path tracking strings cleanly resolve
// down across local execution stacks and multi-tiered payload dictionaries.
func TestPathResolution(t *testing.T) {
	sc := &ScriptComponent{
		State: map[string]interface{}{
			"user": map[string]interface{}{
				"profile": map[string]interface{}{
					"role": "administrator",
				},
			},
		},
	}

	ctx := &ExecContext{
		Component: sc,
		Locals:    make(map[string]interface{}),
		Payload: map[string]string{
			"slug": "security-matrix-post",
		},
	}

	// 1. Verify Deep Nested Struct Mapping Logic
	role := ctx.ResolvePath("user.profile.role")
	if role != "administrator" {
		t.Errorf("Failed deep path traversal resolution, got: %v", role)
	}

	// 2. Verify Incoming HTTP Data Variable Mapping Logic
	slug := ctx.ResolvePath("data.slug")
	if slug != "security-matrix-post" {
		t.Errorf("Failed request packet tracing data context resolution, got: %v", slug)
	}

	// 3. Verify Local Scope Stack Assignments
	ctx.Locals["active_thread"] = 443
	threadID := ctx.ResolvePath("active_thread")
	if threadID != 443 {
		t.Errorf("Failed local thread variable lookup map match, got: %v", threadID)
	}
}
