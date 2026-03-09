package lang

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser builds a Scute AST from a token stream
type Parser struct {
	tokens []Token
	pos    int
	file   string
	errors []error
}

// ParseFile lexes and parses a .scute file, returning a Program AST
func ParseFile(src, filename string) (*Program, []error) {
	l := NewLexer(src, filename)
	tokens, lexErrs := l.Tokenize()
	p := &Parser{tokens: filterTokens(tokens), file: filename}
	prog := p.parseProgram()
	return prog, append(lexErrs, p.errors...)
}

// filterTokens removes comments but keeps newlines (they're structurally significant)
func filterTokens(tokens []Token) []Token {
	var out []Token
	for _, t := range tokens {
		if t.Type != COMMENT {
			out = append(out, t)
		}
	}
	return out
}

// ─── Navigation ───────────────────────────────────────────────────────────────

func (p *Parser) cur() Token {
	// Skip newlines in most positions
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == NEWLINE {
		p.pos++
	}
	if p.pos >= len(p.tokens) {
		return Token{Type: EOF}
	}
	return p.tokens[p.pos]
}

// curRaw returns the current token WITHOUT skipping newlines
func (p *Parser) curRaw() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	t := p.cur()
	p.pos++
	return t
}

func (p *Parser) skipNL() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == NEWLINE {
		p.pos++
	}
}

func (p *Parser) check(tt TokenType) bool {
	return p.cur().Type == tt
}

func (p *Parser) match(types ...TokenType) bool {
	for _, tt := range types {
		if p.cur().Type == tt {
			return true
		}
	}
	return false
}

func (p *Parser) expect(tt TokenType) (Token, bool) {
	t := p.cur()
	if t.Type != tt {
		p.errorf(t, "expected %s, got %q", tt, t.Literal)
		return t, false
	}
	p.advance()
	return t, true
}

func (p *Parser) posOf(t Token) Pos {
	return Pos{File: p.file, Line: t.Line, Col: t.Col}
}

func (p *Parser) errorf(t Token, format string, args ...interface{}) {
	p.errors = append(p.errors, &ParseError{
		Pos:     p.posOf(t),
		Message: fmt.Sprintf(format, args...),
	})
}

// isTypeHint checks if the current token is a type keyword
func (p *Parser) isTypeHint() bool {
	return p.match(TYPE_STRING, TYPE_NUMBER, TYPE_BOOL, TYPE_LIST, TYPE_MAP, TYPE_ANY)
}

// ─── Program ──────────────────────────────────────────────────────────────────

func (p *Parser) parseProgram() *Program {
	prog := &Program{Pos: Pos{File: p.file, Line: 1}}
	p.skipNL()
	for !p.check(EOF) {
		stmt := p.parseTopLevel()
		if stmt != nil {
			prog.Statements = append(prog.Statements, stmt)
		}
		p.skipNL()
	}
	return prog
}

func (p *Parser) parseTopLevel() Node {
	t := p.cur()
	switch t.Type {
	case TERRITORY:
		return p.parseTerritory()
	case HABITAT:
		return p.parseHabitat()
	case SPAWN:
		return p.parseSpawn(false)
	case PROTECTED:
		return p.parseSpawn(true)
	case MARK:
		return p.parseMark()
	case CAMOUFLAGE:
		return p.parseCamouflage()
	case SIGNAL:
		return p.parseSignal()
	case SNACK:
		return p.parseSnackImport()
	default:
		p.errorf(t, "unexpected %q at top level — expected territory, habitat, spawn, mark, camouflage, signal, or snack", t.Literal)
		p.advance()
		return nil
	}
}

// ─── Block Parsers ────────────────────────────────────────────────────────────

func (p *Parser) parseTerritory() *TerritoryBlock {
	tok := p.advance() // territory
	b := &TerritoryBlock{Pos: p.posOf(tok)}
	if p.check(STRING) {
		b.Name = p.advance().Literal
	} else if p.check(IDENT) {
		b.Name = p.advance().Literal
	}
	b.Fields = p.parseFieldsUntilEnd()
	return b
}

func (p *Parser) parseHabitat() *HabitatBlock {
	tok := p.advance() // habitat
	b := &HabitatBlock{Pos: p.posOf(tok)}
	if p.check(STRING) {
		b.Name = p.advance().Literal
	} else if p.check(IDENT) {
		b.Name = p.advance().Literal
	}
	b.Fields = p.parseFieldsUntilEnd()
	return b
}

func (p *Parser) parseSpawn(protected bool) *SpawnBlock {
	tok := p.advance() // spawn or spawn!
	b := &SpawnBlock{Pos: p.posOf(tok), Protected: protected}

	// Type string: "k8s:namespace" or unquoted k8s:namespace
	if p.check(STRING) {
		b.TypeStr = p.advance().Literal
	} else {
		b.TypeStr = p.parseUnquotedType()
	}

	// as "name"
	if p.check(AS) {
		p.advance()
		if p.check(STRING) {
			b.Name = p.advance().Literal
		} else if p.check(IDENT) {
			b.Name = p.advance().Literal
		}
	}

	// across [list] or across expr
	if p.check(ACROSS) {
		p.advance()
		b.Across = p.parseExpr()
	}

	p.skipNL()

	// Parse body until end
	for !p.check(END) && !p.check(EOF) {
		t := p.cur()
		switch t.Type {
		case WHEN, UNLESS:
			b.Whens = append(b.Whens, p.parseWhen())
		case NEEDS:
			// needs: @resource or needs: [@a, @b]
			p.advance()
			p.expect(COLON)
			dep := p.parseExpr()
			b.Needs = append(b.Needs, dep)
		case IDENT:
			// Check for key: value or key: \n ... end (sub-block)
			f := p.parseField()
			if f != nil {
				b.Fields = append(b.Fields, f)
			}
		case NEWLINE:
			p.advance()
		default:
			p.errorf(t, "unexpected %q inside spawn block", t.Literal)
			p.advance()
		}
		p.skipNL()
	}
	p.expect(END)
	return b
}

func (p *Parser) parseMark() *MarkDecl {
	tok := p.advance() // mark
	m := &MarkDecl{Pos: p.posOf(tok)}
	if p.check(IDENT) {
		m.Name = p.advance().Literal
	}
	// Optional type hint
	if p.isTypeHint() {
		m.TypeHint = p.advance().Type
	}
	// : default_value
	if p.check(COLON) {
		p.advance()
		m.Default = p.parseExpr()
	}
	return m
}

func (p *Parser) parseCamouflage() *CamouflageBlock {
	tok := p.advance() // camouflage
	b := &CamouflageBlock{Pos: p.posOf(tok)}
	b.Fields = p.parseFieldsUntilEnd()
	return b
}

func (p *Parser) parseSignal() *SignalBlock {
	tok := p.advance() // signal
	b := &SignalBlock{Pos: p.posOf(tok)}
	if p.check(STRING) {
		b.Name = p.advance().Literal
	} else if p.check(IDENT) {
		b.Name = p.advance().Literal
	}
	p.skipNL()
	for !p.check(END) && !p.check(EOF) {
		if p.check(IDENT) {
			key := p.cur().Literal
			f := p.parseField()
			if f == nil {
				continue
			}
			switch key {
			case "value":
				b.Value = f.Value
			case "description":
				if sl, ok := f.Value.(*StringLiteral); ok {
					b.Description = sl.Raw
				}
			case "sensitive":
				if bl, ok := f.Value.(*BoolLiteral); ok {
					b.Sensitive = bl.Value
				}
			}
		} else if p.check(NEWLINE) {
			p.advance()
		} else {
			p.advance()
		}
		p.skipNL()
	}
	p.expect(END)
	return b
}

// ─── Field Parsing ────────────────────────────────────────────────────────────

// parseFieldsUntilEnd reads key: value fields until 'end'
func (p *Parser) parseFieldsUntilEnd() []*Field {
	var fields []*Field
	p.skipNL()
	for !p.check(END) && !p.check(EOF) {
		if p.check(IDENT) {
			f := p.parseField()
			if f != nil {
				fields = append(fields, f)
			}
		} else if p.check(NEWLINE) {
			p.advance()
		} else {
			p.advance()
		}
		p.skipNL()
	}
	p.expect(END)
	return fields
}

// parseField reads a single "key: value" or "key:\n ... end" field
func (p *Parser) parseField() *Field {
	tok := p.cur()
	key := p.advance().Literal
	if !p.check(COLON) {
		p.errorf(tok, "expected ':' after field name %q (Scute uses ':' not '=')", key)
		return nil
	}
	p.advance() // consume ':'

	// Detect sub-block: "key:\n" where the next non-whitespace line starts a new field
	// Sub-blocks are ended with 'end'
	if p.curRaw().Type == NEWLINE {
		// Peek ahead — if the next content is a field assignment, treat as sub-block
		p.skipNL()
		if p.check(IDENT) || p.check(END) {
			// Sub-block
			sub := &SubBlock{Pos: p.posOf(tok)}
			for !p.check(END) && !p.check(EOF) {
				if p.check(IDENT) {
					sf := p.parseField()
					if sf != nil {
						sub.Fields = append(sub.Fields, sf)
					}
				} else if p.check(NEWLINE) {
					p.advance()
				} else {
					break
				}
				p.skipNL()
			}
			p.expect(END)
			return &Field{Pos: p.posOf(tok), Key: key, Value: sub}
		}
	}

	val := p.parseExpr()
	return &Field{Pos: p.posOf(tok), Key: key, Value: val}
}

// ─── When Block ───────────────────────────────────────────────────────────────

func (p *Parser) parseWhen() *WhenBlock {
	tok := p.cur()
	negate := tok.Type == UNLESS
	p.advance() // when / unless
	w := &WhenBlock{Pos: p.posOf(tok), Negate: negate}
	w.Condition = p.parseExpr()
	p.skipNL()
	for !p.check(END) && !p.check(ELSE) && !p.check(EOF) {
		if p.check(IDENT) {
			f := p.parseField()
			if f != nil {
				w.Then = append(w.Then, f)
			}
		} else if p.check(NEWLINE) {
			p.advance()
		} else {
			p.advance()
		}
		p.skipNL()
	}
	if p.check(ELSE) {
		p.advance()
		p.skipNL()
		for !p.check(END) && !p.check(EOF) {
			if p.check(IDENT) {
				f := p.parseField()
				if f != nil {
					w.Else = append(w.Else, f)
				}
			} else if p.check(NEWLINE) {
				p.advance()
			} else {
				p.advance()
			}
			p.skipNL()
		}
	}
	p.expect(END)
	return w
}

// ─── Type String ──────────────────────────────────────────────────────────────

// parseUnquotedType reads: k8s:namespace (ident : ident)
func (p *Parser) parseUnquotedType() string {
	var sb strings.Builder
	if p.check(IDENT) {
		sb.WriteString(p.advance().Literal)
	}
	if p.check(COLON) {
		p.advance()
		sb.WriteRune(':')
		if p.check(IDENT) {
			sb.WriteString(p.advance().Literal)
		}
	}
	return sb.String()
}

// ─── Expression Parsing (Pratt) ───────────────────────────────────────────────

func (p *Parser) parseExpr() Node {
	return p.parsePipe()
}

func (p *Parser) parsePipe() Node {
	left := p.parseTernary()
	for p.check(PIPE) {
		tok := p.advance()
		right := p.parseTernary()
		left = &PipeExpr{Pos: p.posOf(tok), Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseTernary() Node {
	cond := p.parseOr()
	if p.check(QUESTION) {
		tok := p.advance()
		then := p.parseOr()
		if !p.check(COLON) {
			p.errorf(p.cur(), "expected ':' in ternary expression")
		} else {
			p.advance()
		}
		els := p.parseOr()
		return &TernaryExpr{Pos: p.posOf(tok), Condition: cond, Then: then, Else: els}
	}
	return cond
}

func (p *Parser) parseOr() Node {
	left := p.parseAnd()
	for p.check(OR) {
		tok := p.advance()
		right := p.parseAnd()
		left = &BinaryExpr{Pos: p.posOf(tok), Op: "||", Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseAnd() Node {
	left := p.parseEquality()
	for p.check(AND) {
		tok := p.advance()
		right := p.parseEquality()
		left = &BinaryExpr{Pos: p.posOf(tok), Op: "&&", Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseEquality() Node {
	left := p.parseComparison()
	for p.match(EQ, NEQ) {
		tok := p.cur()
		op := p.advance().Literal
		right := p.parseComparison()
		left = &BinaryExpr{Pos: p.posOf(tok), Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseComparison() Node {
	left := p.parseAddSub()
	for p.match(LT, GT, LTE, GTE) {
		tok := p.cur()
		op := p.advance().Literal
		right := p.parseAddSub()
		left = &BinaryExpr{Pos: p.posOf(tok), Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseAddSub() Node {
	left := p.parseMulDiv()
	for p.match(PLUS, MINUS) {
		tok := p.cur()
		op := p.advance().Literal
		right := p.parseMulDiv()
		left = &BinaryExpr{Pos: p.posOf(tok), Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseMulDiv() Node {
	left := p.parseUnary()
	for p.match(STAR, SLASH, PERCENT) {
		tok := p.cur()
		op := p.advance().Literal
		right := p.parseUnary()
		left = &BinaryExpr{Pos: p.posOf(tok), Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseUnary() Node {
	if p.check(BANG) || p.check(MINUS) {
		tok := p.cur()
		op := p.advance().Literal
		expr := p.parseUnary()
		return &UnaryExpr{Pos: p.posOf(tok), Op: op, Expr: expr}
	}
	return p.parseRange()
}

func (p *Parser) parseRange() Node {
	left := p.parsePrimary()
	if p.check(DOTDOT) {
		tok := p.advance()
		right := p.parsePrimary()
		return &RangeExpr{Pos: p.posOf(tok), Start: left, End: right}
	}
	return left
}

func (p *Parser) parsePrimary() Node {
	t := p.cur()
	pos := p.posOf(t)

	switch t.Type {
	case NUMBER:
		p.advance()
		v, _ := strconv.ParseFloat(t.Literal, 64)
		return &NumberLiteral{Pos: pos, Value: v, Raw: t.Literal}

	case BOOL:
		p.advance()
		return &BoolLiteral{Pos: pos, Value: t.Literal == "true"}

	case NULL:
		p.advance()
		return &NullLiteral{Pos: pos}

	case STRING:
		p.advance()
		return p.buildStringLiteral(t)

	case LBRACKET:
		return p.parseArray(pos)

	case LPAREN:
		p.advance()
		expr := p.parseExpr()
		p.expect(RPAREN)
		return expr

	case AT:
		return p.parseResourceRef(pos)

	case ITEM:
		p.advance()
		ref := &Reference{Pos: pos, Parts: []string{"item"}}
		// item.key or item.value
		for p.check(DOT) {
			p.advance()
			if p.check(IDENT) {
				ref.Parts = append(ref.Parts, p.advance().Literal)
			}
		}
		return ref

	case IDENT, CAMOUFLAGE:
		return p.parseRefOrCall()

	case NEEDS:
		// 'needs' used as a field name inside spawn blocks; parsed as ident here
		p.advance()
		return &Reference{Pos: pos, Parts: []string{"needs"}}
	}

	p.errorf(t, "unexpected token in expression: %q", t.Literal)
	p.advance()
	return &NullLiteral{Pos: pos}
}

func (p *Parser) parseResourceRef(pos Pos) *ResourceRef {
	p.advance() // @
	ref := &ResourceRef{Pos: pos}
	if p.check(IDENT) {
		ref.Name = p.advance().Literal
	}
	if p.check(DOT) {
		p.advance()
		if p.check(IDENT) {
			ref.Field = p.advance().Literal
			// allow further chaining: @monitoring.name.sub
			for p.check(DOT) {
				p.advance()
				if p.check(IDENT) {
					ref.Field += "." + p.advance().Literal
				}
			}
		}
	}
	return ref
}

func (p *Parser) parseArray(pos Pos) *ArrayLiteral {
	p.advance() // [
	a := &ArrayLiteral{Pos: pos}
	p.skipNL()
	for !p.check(RBRACKET) && !p.check(EOF) {
		a.Elements = append(a.Elements, p.parseExpr())
		p.skipNL()
		if p.check(COMMA) {
			p.advance()
			p.skipNL()
		}
	}
	p.expect(RBRACKET)
	return a
}

func (p *Parser) parseRefOrCall() Node {
	tok := p.cur()
	pos := p.posOf(tok)
	name := p.advance().Literal

	// Function call: name(args...)
	if p.check(LPAREN) {
		p.advance()
		var args []Node
		p.skipNL()
		for !p.check(RPAREN) && !p.check(EOF) {
			args = append(args, p.parseExpr())
			p.skipNL()
			if p.check(COMMA) {
				p.advance()
				p.skipNL()
			}
		}
		p.expect(RPAREN)
		return &FunctionCall{Pos: pos, Name: name, Args: args}
	}

	// Dotted reference: name.field.sub
	parts := []string{name}
	for p.check(DOT) {
		p.advance()
		if p.check(IDENT) {
			parts = append(parts, p.advance().Literal)
		}
	}
	return &Reference{Pos: pos, Parts: parts}
}

// buildStringLiteral splits a STRING token on \x00INTERP\x00...\x00END\x00 markers
func (p *Parser) buildStringLiteral(t Token) *StringLiteral {
	lit := t.Literal
	pos := p.posOf(t)
	sl := &StringLiteral{Pos: pos, Raw: lit}
	const startMark = "\x00INTERP\x00"
	const endMark = "\x00END\x00"

	if !strings.Contains(lit, startMark) {
		sl.Parts = []StringPart{{Text: lit}}
		return sl
	}

	remaining := lit
	for len(remaining) > 0 {
		si := strings.Index(remaining, startMark)
		if si < 0 {
			sl.Parts = append(sl.Parts, StringPart{Text: remaining})
			break
		}
		if si > 0 {
			sl.Parts = append(sl.Parts, StringPart{Text: remaining[:si]})
		}
		remaining = remaining[si+len(startMark):]
		ei := strings.Index(remaining, endMark)
		exprSrc := remaining
		if ei >= 0 {
			exprSrc = remaining[:ei]
			remaining = remaining[ei+len(endMark):]
		} else {
			remaining = ""
		}
		// Parse the interpolated expression in its own mini-parser
		innerLex := NewLexer(exprSrc, "<interp>")
		innerToks, _ := innerLex.Tokenize()
		inner := &Parser{tokens: filterTokens(innerToks), file: "<interp>"}
		expr := inner.parseExpr()
		sl.Parts = append(sl.Parts, StringPart{IsInterp: true, Expr: expr})
	}
	return sl
}
