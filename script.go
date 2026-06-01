package guikit

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/scanner"

	"github.com/0TrustCloud/ultimate_db"
)

var errReturnSignal = errors.New("return statement reached")

// ==========================================
// 1. Transactional DML & Context Modeling
// ==========================================

type DMLInstruction struct {
	Action   string `json:"action"` // "append", "remove", "addClass", "removeClass", "setValue"
	TargetID string `json:"target_id"`
	Value    string `json:"value"`
}

type UserFunction struct {
	Params []string
	Body   []ScriptNode
}

type BuiltinFunc func(ctx *ExecContext, args []interface{}) (interface{}, error)

var builtins = map[string]BuiltinFunc{
	"len": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if len(args) != 1 { return 0, errors.New("len() expects 1 argument") }
		if arr, ok := args[0].([]interface{}); ok { return len(arr), nil }
		if str, ok := args[0].(string); ok { return len(str), nil }
		return 0, nil
	},
	"append": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if len(args) != 2 { return nil, errors.New("append() expects 2 arguments") }
		arr, ok := args[0].([]interface{})
		if !ok { return nil, errors.New("append targets arrays exclusively") }
		return append(arr, args[1]), nil
	},
	"dml.append": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if len(args) != 2 { return nil, errors.New("dml.append expects (target_id, raw_html)") }
		ctx.DMLActions = append(ctx.DMLActions, DMLInstruction{
			Action:   "append",
			TargetID: fmt.Sprintf("%v", args[0]),
			Value:    fmt.Sprintf("%v", args[1]),
		})
		return nil, nil
	},
	"dml.remove": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if len(args) != 1 { return nil, errors.New("dml.remove expects (target_id)") }
		ctx.DMLActions = append(ctx.DMLActions, DMLInstruction{
			Action:   "remove",
			TargetID: fmt.Sprintf("%v", args[0]),
		})
		return nil, nil
	},
	"dml.addClass": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if len(args) != 2 { return nil, errors.New("dml.addClass expects (target_id, class_name)") }
		ctx.DMLActions = append(ctx.DMLActions, DMLInstruction{
			Action:   "addClass",
			TargetID: fmt.Sprintf("%v", args[0]),
			Value:    fmt.Sprintf("%v", args[1]),
		})
		return nil, nil
	},
	"dml.removeClass": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if len(args) != 2 { return nil, errors.New("dml.removeClass expects (target_id, class_name)") }
		ctx.DMLActions = append(ctx.DMLActions, DMLInstruction{
			Action:   "removeClass",
			TargetID: fmt.Sprintf("%v", args[0]),
			Value:    fmt.Sprintf("%v", args[1]),
		})
		return nil, nil
	},
	"dml.setValue": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if len(args) != 2 { return nil, errors.New("dml.setValue expects (target_id, value)") }
		ctx.DMLActions = append(ctx.DMLActions, DMLInstruction{
			Action:   "setValue",
			TargetID: fmt.Sprintf("%v", args[0]),
			Value:    fmt.Sprintf("%v", args[1]),
		})
		return nil, nil
	},
	"dbFind": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if len(args) != 2 { return nil, errors.New("dbFind expects (table, id)") }
		id, ok := toInt(args[1])
		if !ok { return nil, errors.New("invalid table identifier key format") }
		var target map[string]interface{}
		_ = ctx.ORM.Find(uint64(id), &target)
		return target, nil
	},
	"cloudUser": func(ctx *ExecContext, args []interface{}) (interface{}, error) {
		if user, exists := ctx.Payload["_0trust_user_identity"]; exists { return user, nil }
		return "anonymous", nil
	},
}

type ExecContext struct {
	Component  *ScriptComponent
	Payload    map[string]string
	Locals     map[string]interface{}
	DB         *ultimate_db.DB
	ORM        *ultimate_db.ORM
	DMLActions []DMLInstruction 
}

func (ctx *ExecContext) ResolvePath(path string) interface{} {
	parts := strings.Split(path, ".")
	if len(parts) == 0 { return nil }

	var current interface{}
	if parts[0] == "data" {
		if len(parts) == 2 { return ctx.Payload[parts[1]] }
		current = ctx.Payload
	} else if val, exists := ctx.Locals[parts[0]]; exists {
		current = val
	} else {
		ctx.Component.mu.RLock()
		val, exists := ctx.Component.State[parts[0]]
		ctx.Component.mu.RUnlock()
		if exists { current = val }
	}

	for i := 1; i < len(parts); i++ {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[parts[i]]
		} else if mStr, ok := current.(map[string]string); ok {
			current = mStr[parts[i]]
		} else {
			return nil
		}
	}
	return current
}

func (ctx *ExecContext) SetPath(path string, val interface{}) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 { return }

	if len(parts) == 1 {
		if _, exists := ctx.Locals[parts[0]]; exists {
			ctx.Locals[parts[0]] = val
			return
		}
		ctx.Component.mu.Lock()
		ctx.Component.State[parts[0]] = val
		ctx.Component.mu.Unlock()
		return
	}

	var current interface{}
	if val, exists := ctx.Locals[parts[0]]; exists { current = val } else {
		ctx.Component.mu.Lock()
		current = ctx.Component.State[parts[0]]
		ctx.Component.mu.Unlock()
	}

	for i := 1; i < len(parts)-1; i++ {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[parts[i]]
		}
	}

	if m, ok := current.(map[string]interface{}); ok {
		m[parts[len(parts)-1]] = val
	}
}

type ScriptComponent struct {
	Id         string
	GmlPath    string
	ScriptPath string
	State      map[string]interface{}
	Handlers   map[string]*ScriptBlock
	Functions  map[string]UserFunction
	Db         *ultimate_db.DB
	Orm        *ultimate_db.ORM
	mu         sync.RWMutex
}

func (sc *ScriptComponent) ID() string { return sc.Id }

func (sc *ScriptComponent) Render() string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	input, err := fs.ReadFile(AppFS, sc.GmlPath)
	if err != nil { return fmt.Sprintf(`<div>GML Source Missing: %s</div>`, sc.GmlPath) }
	return string(input)
}

type ScriptBlock struct{ Statements []ScriptNode }

// ==========================================
// 2. AST Evaluator Hierarchy
// ==========================================

type Expr interface {
	Eval(ctx *ExecContext) interface{}
}

type LiteralExpr struct{ Value interface{} }
func (le LiteralExpr) Eval(ctx *ExecContext) interface{} { return le.Value }

type VariableExpr struct{ Path string }
func (ve VariableExpr) Eval(ctx *ExecContext) interface{} { return ctx.ResolvePath(ve.Path) }

type BinaryExpr struct {
	Left  Expr
	Op    string
	Right Expr
}
func (be BinaryExpr) Eval(ctx *ExecContext) interface{} {
	leftVal := be.Left.Eval(ctx)
	if be.Op == "&&" { return toBool(leftVal) && toBool(be.Right.Eval(ctx)) }
	if be.Op == "||" { return toBool(leftVal) || toBool(be.Right.Eval(ctx)) }

	rightVal := be.Right.Eval(ctx)
	lInt, lIsInt := toInt(leftVal)
	rInt, rIsInt := toInt(rightVal)

	if lIsInt && rIsInt {
		switch be.Op {
		case "+":  return lInt + rInt
		case "-":  return lInt - rInt
		case "*":  return lInt * rInt
		case "/":  if rInt == 0 { return 0 }; return lInt / rInt
		case "==": return lInt == rInt
		case "!=": return lInt != rInt
		case "<":  return lInt < rInt
		case ">":  return lInt < rInt
		case "<=": return lInt <= rInt
		case ">=": return lInt >= rInt
		}
	}

	lStr := fmt.Sprintf("%v", leftVal)
	rStr := fmt.Sprintf("%v", rightVal)
	switch be.Op {
	case "+":  return lStr + rStr
	case "==": return lStr == rStr
	case "!=": return lStr != rStr
	}
	return nil
}

type CallExpr struct {
	Name string
	Args []Expr
}
func (ce CallExpr) Eval(ctx *ExecContext) interface{} {
	if userFn, exists := ctx.Component.Functions[ce.Name]; exists {
		fnCtx := &ExecContext{
			Component:  ctx.Component,
			Payload:    ctx.Payload,
			Locals:     make(map[string]interface{}),
			DB:         ctx.DB,
			ORM:        ctx.ORM,
			DMLActions: ctx.DMLActions,
		}
		for i, param := range userFn.Params {
			if i < len(ce.Args) { fnCtx.Locals[param] = ce.Args[i].Eval(ctx) }
		}
		for _, stmt := range userFn.Body {
			if err := stmt.Execute(fnCtx); err == errReturnSignal { break }
		}
		ctx.DMLActions = fnCtx.DMLActions 
		return fnCtx.Locals["_return"]
	}

	if fn, exists := builtins[ce.Name]; exists {
		evaluatedArgs := make([]interface{}, len(ce.Args))
		for i, arg := range ce.Args { evaluatedArgs[i] = arg.Eval(ctx) }
		val, err := fn(ctx, evaluatedArgs)
		if err == nil { return val }
	}
	return nil
}

type ScriptNode interface {
	Execute(ctx *ExecContext) error
}

type ReturnNode struct{ Expression Expr }
func (rn ReturnNode) Execute(ctx *ExecContext) error {
	if rn.Expression != nil { ctx.Locals["_return"] = rn.Expression.Eval(ctx) }
	return errReturnSignal
}

type AssignNode struct {
	Path       string
	Expression Expr
	Op         string
}
func (an AssignNode) Execute(ctx *ExecContext) error {
	rhs := an.Expression.Eval(ctx)
	if an.Op == "=" { ctx.SetPath(an.Path, rhs); return nil }

	lhs := ctx.ResolvePath(an.Path)
	lInt, lOk := toInt(lhs)
	rInt, rOk := toInt(rhs)
	if lOk && rOk {
		if an.Op == "+=" { ctx.SetPath(an.Path, lInt + rInt) }
		if an.Op == "-=" { ctx.SetPath(an.Path, lInt - rInt) }
	}
	return nil
}

type IfNode struct {
	Condition Expr
	Body      []ScriptNode
	ElseBody  []ScriptNode
}
func (in IfNode) Execute(ctx *ExecContext) error {
	target := in.ElseBody
	if toBool(in.Condition.Eval(ctx)) { target = in.Body }
	for _, stmt := range target {
		if err := stmt.Execute(ctx); err != nil { return err }
	}
	return nil
}

type ForNode struct {
	IteratorKey string
	IterableKey string
	Body        []ScriptNode
}
func (fn ForNode) Execute(ctx *ExecContext) error {
	collection := ctx.ResolvePath(fn.IterableKey)
	sliceVal, ok := collection.([]interface{})
	if !ok { return nil }

	for _, item := range sliceVal {
		ctx.Locals[fn.IteratorKey] = item
		for _, stmt := range fn.Body {
			if err := stmt.Execute(ctx); err != nil { return err }
		}
	}
	delete(ctx.Locals, fn.IteratorKey)
	return nil
}

// ==========================================
// 3. Parser Blueprint Logic
// ==========================================

type ScriptParser struct {
	s   scanner.Scanner
	tok rune
}

func NewScriptParser(src string) *ScriptParser {
	var s scanner.Scanner
	s.Init(strings.NewReader(src))
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
	}
	p := &ScriptParser{s: s}
	p.next()
	return p
}

func (p *ScriptParser) next() { p.tok = p.s.Scan() }

func (p *ScriptParser) Parse(sc *ScriptComponent, namespace string) error {
	for p.tok != scanner.EOF {
		if p.tok == scanner.Ident {
			keyword := p.s.TokenText()
			p.next()

			if keyword == "state" && namespace == "" {
				if p.tok != '{' { return errors.New("expected '{' opening state block") }
				p.next()
				if err := p.parseStateBlock(sc); err != nil { return err }

			} else if keyword == "import" {
				modulePath := stripQuotes(p.s.TokenText())
				p.next()
				moduleSrc, err := fs.ReadFile(AppFS, modulePath+".gs")
				if err != nil { return fmt.Errorf("module lookup unresolvable: %s", modulePath) }
				ns := filepath.Base(modulePath)
				subParser := NewScriptParser(string(moduleSrc))
				if err := subParser.Parse(sc, ns); err != nil { return err }

			} else if keyword == "func" {
				funcName := p.s.TokenText()
				if namespace != "" { funcName = namespace + "." + funcName }
				p.next() 
				p.next() 
				params := []string{}
				for p.tok != ')' && p.tok != scanner.EOF {
					if p.tok == scanner.Ident { params = append(params, p.s.TokenText()) }
					p.next()
					if p.tok == ',' { p.next() }
				}
				p.next() 
				p.next() 
				bodyBlock, err := p.parseBlock()
				if err != nil { return err }
				sc.Functions[funcName] = UserFunction{Params: params, Body: bodyBlock.Statements}

			} else if keyword == "handle" && namespace == "" {
				eventName := p.s.TokenText()
				p.next() 
				p.next() 
				block, err := p.parseBlock()
				if err != nil { return err }
				sc.Handlers[eventName] = block
			}
		} else {
			p.next()
		}
	}
	return nil
}

func (p *ScriptParser) parseStateBlock(sc *ScriptComponent) error {
	for p.tok != '}' && p.tok != scanner.EOF {
		if p.tok == scanner.Ident {
			key := p.s.TokenText()
			p.next()
			p.next() 
			val, err := p.parseStateValue()
			if err != nil { return err }
			sc.State[key] = val
		} else {
			p.next()
		}
	}
	if p.tok == '}' { p.next() }
	return nil
}

func (p *ScriptParser) parseStateValue() (interface{}, error) {
	if p.tok == scanner.Int {
		val, _ := strconv.Atoi(p.s.TokenText())
		p.next()
		return val, nil
	}
	if p.tok == scanner.String || p.tok == scanner.RawString {
		val := stripQuotes(p.s.TokenText())
		p.next()
		return val, nil
	}
	if p.tok == '[' {
		p.next() 
		arr := []interface{}{}
		for p.tok != ']' && p.tok != scanner.EOF {
			val, err := p.parseStateValue()
			if err != nil { return nil, err }
			arr = append(arr, val)
			if p.tok == ',' { p.next() }
		}
		if p.tok == ']' { p.next() }
		return arr, nil
	}
	return nil, fmt.Errorf("invalid state format configuration sequence near %s", p.s.TokenText())
}

func (p *ScriptParser) parseBlock() (*ScriptBlock, error) {
	block := &ScriptBlock{Statements: []ScriptNode{}}
	for p.tok != '}' && p.tok != scanner.EOF {
		if p.tok == scanner.Ident {
			ident := p.parseDottedIdentifier()

			if ident == "if" {
				node, err := p.parseIf()
				if err != nil { return nil, err }
				block.Statements = append(block.Statements, node)
			} else if ident == "for" {
				node, err := p.parseFor()
				if err != nil { return nil, err }
				block.Statements = append(block.Statements, node)
			} else if ident == "return" {
				expr, err := p.parseExpression(0)
				if err != nil { return nil, err }
				block.Statements = append(block.Statements, ReturnNode{Expression: expr})
			} else {
				node, err := p.parseAssignment(ident)
				if err != nil { return nil, err }
				block.Statements = append(block.Statements, node)
			}
		} else {
			p.next()
		}
	}
	if p.tok == '}' { p.next() }
	return block, nil
}

func (p *ScriptParser) parseIf() (IfNode, error) {
	node := IfNode{}
	expr, err := p.parseExpression(0)
	if err != nil { return node, err }
	node.Condition = expr

	p.next() 
	bodyBlock, err := p.parseBlock()
	if err != nil { return node, err }
	node.Body = bodyBlock.Statements

	if p.tok == scanner.Ident && p.s.TokenText() == "else" {
		p.next() 
		p.next() 
		elseBlock, err := p.parseBlock()
		if err != nil { return node, err }
		node.ElseBody = elseBlock.Statements
	}
	return node, nil
}

func (p *ScriptParser) parseFor() (ForNode, error) {
	node := ForNode{}
	node.IteratorKey = p.s.TokenText()
	p.next() 
	p.next() 
	node.IterableKey = p.parseDottedIdentifier()
	p.next() 

	body, err := p.parseBlock()
	if err != nil { return node, err }
	node.Body = body.Statements
	return node, nil
}

func (p *ScriptParser) parseAssignment(path string) (AssignNode, error) {
	node := AssignNode{Path: path}
	op := p.s.TokenText()
	if op == "+" || op == "-" {
		p.next()
		op += p.s.TokenText()
	}
	node.Op = op
	p.next() 

	expr, err := p.parseExpression(0)
	if err != nil { return node, err }
	node.Expression = expr
	return node, nil
}

var opPrecedence = map[string]int{
	"||": 1, "&&": 2,
	"==": 3, "!=": 3, "<": 3, ">": 3, "<=": 3, ">=": 3,
	"+":  4, "-":  4, "*":  5, "/":  5,
}

func (p *ScriptParser) parseExpression(precedence int) (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil { return nil, err }

	for {
		tokText := p.s.TokenText()
		if p.tok == '&' || p.tok == '|' || p.tok == '=' || p.tok == '!' || p.tok == '<' || p.tok == '>' {
			lookahead := p.s.Peek()
			if (p.tok == '&' && lookahead == '&') || (p.tok == '|' && lookahead == '|') || (lookahead == '=') {
				p.next()
				tokText += p.s.TokenText()
			}
		}

		prec, exists := opPrecedence[tokText]
		if !exists || prec < precedence { break }

		p.next()
		right, err := p.parseExpression(prec + 1)
		if err != nil { return nil, err }
		left = BinaryExpr{Left: left, Op: tokText, Right: right}
	}
	return left, nil
}

func (p *ScriptParser) parsePrimary() (Expr, error) {
	if p.tok == scanner.Int {
		val, _ := strconv.Atoi(p.s.TokenText())
		p.next()
		return LiteralExpr{Value: val}, nil
	}
	if p.tok == scanner.String || p.tok == scanner.RawString {
		val := stripQuotes(p.s.TokenText())
		p.next()
		return LiteralExpr{Value: val}, nil
	}
	if p.tok == scanner.Ident {
		text := p.parseDottedIdentifier()
		if p.tok == '(' {
			p.next() 
			call := CallExpr{Name: text, Args: []Expr{}}
			for p.tok != ')' && p.tok != scanner.EOF {
				arg, err := p.parseExpression(0)
				if err != nil { return nil, err }
				call.Args = append(call.Args, arg)
				if p.tok == ',' { p.next() }
			}
			p.next() 
			return call, nil
		}
		return VariableExpr{Path: text}, nil
	}
	return nil, errors.New("expression parsing error tree terminal mismatch")
}

func (p *ScriptParser) parseDottedIdentifier() string {
	ident := p.s.TokenText()
	p.next()
	for p.tok == '.' {
		p.next() 
		ident += "." + p.s.TokenText()
		p.next() 
	}
	return ident
}

// ==========================================
// 4. Framework Hook Integration Layer
// ==========================================

func LoadScriptComponent(gk *GUIKit, id string, viewPath string) (*ScriptComponent, error) {
	sc := &ScriptComponent{
		Id:         id,
		GmlPath:    viewPath + ".gml",
		ScriptPath: viewPath + ".gs",
		State:      make(map[string]interface{}),
		Handlers:   make(map[string]*ScriptBlock),
		Functions:  make(map[string]UserFunction),
		Db:         gk.DB,
		ORM:        gk.ORM,
	}

	scriptContent, err := fs.ReadFile(AppFS, sc.ScriptPath)
	if err != nil { return nil, fmt.Errorf("target controller source missing: %w", err) }

	parser := NewScriptParser(string(scriptContent))
	if err := parser.Parse(sc, ""); err != nil {
		return nil, fmt.Errorf("compilation failure in %s: %w", sc.ScriptPath, err)
	}

	gk.RegisterComponent(sc)
	return sc, nil
}

func (sc *ScriptComponent) InvokeEvent(name string, data map[string]string) ([]DMLInstruction, error) {
	sc.mu.Lock()
	block, exists := sc.Handlers[name]
	sc.mu.Unlock()

	if !exists { return nil, fmt.Errorf("handler %s unconfigured", name) }

	ctx := &ExecContext{
		Component:  sc,
		Payload:    data,
		Locals:     make(map[string]interface{}),
		DB:         sc.Db,
		ORM:        sc.ORM,
		DMLActions: []DMLInstruction{},
	}

	for _, stmt := range block.Statements {
		if err := stmt.Execute(ctx); err != nil && err != errReturnSignal { return nil, err }
	}
	return ctx.DMLActions, nil
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int: return val, true
	case float64: return int(val), true
	case string:
		if i, err := strconv.Atoi(val); err == nil { return i, true }
	}
	return 0, false
}

func toBool(v interface{}) bool {
	if b, ok := v.(bool); ok { return b }
	if i, ok := toInt(v); ok { return i != 0 }
	if s, ok := v.(string); ok { return s != "" && s != "false" }
	return v != nil
}
