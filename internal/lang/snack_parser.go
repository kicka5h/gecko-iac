package lang

// ParseSnackFile parses a .snack file into a SnackFile AST.
// A .snack file contains one or more export blocks.
func ParseSnackFile(src, filename string) (*SnackFile, []error) {
	l := NewLexer(src, filename)
	tokens, lexErrs := l.Tokenize()
	p := &Parser{tokens: filterTokens(tokens), file: filename}
	sf := p.parseSnackFile()
	return sf, append(lexErrs, p.errors...)
}

func (p *Parser) parseSnackFile() *SnackFile {
	sf := &SnackFile{Pos: Pos{File: p.file, Line: 1}}
	p.skipNL()
	for !p.check(EOF) {
		if p.check(EXPORT) {
			sf.Exports = append(sf.Exports, p.parseExport())
		} else if p.check(COMMENT) || p.check(NEWLINE) {
			p.advance()
		} else {
			t := p.cur()
			p.errorf(t, "unexpected %q in snack file — expected 'export'", t.Literal)
			p.advance()
		}
		p.skipNL()
	}
	return sf
}

// parseExport parses an export block in a .snack file:
//
//	export "monitoring-stack"
//	  param namespace  string: "monitoring"
//	  param replicas   number: 2
//	  camouflage ... end
//	  spawn "proxmox:vm" as "api" ... end
//	  emit "endpoint"
//	    value: "..."
//	  end
//	end
func (p *Parser) parseExport() *ExportBlock {
	tok := p.advance() // export
	b := &ExportBlock{Pos: p.posOf(tok)}
	if p.check(STRING) {
		b.Name = p.advance().Literal
	} else if p.check(IDENT) {
		b.Name = p.advance().Literal
	}
	p.skipNL()
	for !p.check(END) && !p.check(EOF) {
		switch p.cur().Type {
		case PARAM:
			b.Params = append(b.Params, p.parseParam())
		case CAMOUFLAGE:
			b.Camo = p.parseCamouflage()
		case SPAWN:
			b.Spawns = append(b.Spawns, p.parseSpawn(false))
		case PROTECTED:
			b.Spawns = append(b.Spawns, p.parseSpawn(true))
		case EMIT:
			b.Emits = append(b.Emits, p.parseEmit())
		case NEWLINE:
			p.advance()
		default:
			t := p.cur()
			p.errorf(t, "unexpected %q in export block — expected param, spawn, camouflage, or emit", t.Literal)
			p.advance()
		}
		p.skipNL()
	}
	p.expect(END)
	return b
}

// parseParam parses a param declaration inside an export block:
//
//	param namespace  string: "monitoring"
//	param replicas   number: 2
func (p *Parser) parseParam() *ParamDecl {
	tok := p.advance() // param
	pd := &ParamDecl{Pos: p.posOf(tok)}
	if p.check(IDENT) {
		pd.Name = p.advance().Literal
	}
	if p.isTypeHint() {
		pd.TypeHint = p.advance().Type
	}
	if p.check(COLON) {
		p.advance()
		pd.Default = p.parseExpr()
	}
	return pd
}

// parseEmit parses an emit block inside an export block:
//
//	emit "endpoint"
//	  value: "http://prometheus.#{namespace}.svc:9090"
//	end
func (p *Parser) parseEmit() *EmitBlock {
	tok := p.advance() // emit
	b := &EmitBlock{Pos: p.posOf(tok)}
	if p.check(STRING) {
		b.Name = p.advance().Literal
	} else if p.check(IDENT) {
		b.Name = p.advance().Literal
	}
	p.skipNL()
	for !p.check(END) && !p.check(EOF) {
		if p.check(IDENT) && p.cur().Literal == "value" {
			f := p.parseField()
			if f != nil {
				b.Value = f.Value
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

// parseSnackImport parses a snack import in a .scute file:
//
//	snack "monitoring" from "./modules/monitoring.snack"
//	  namespace: "observability"
//	  replicas:  3
//	end
func (p *Parser) parseSnackImport() *SnackImport {
	tok := p.advance() // snack
	si := &SnackImport{Pos: p.posOf(tok)}

	// local alias name: snack "monitoring" ...
	if p.check(STRING) {
		si.Name = p.advance().Literal
	} else if p.check(IDENT) {
		si.Name = p.advance().Literal
	}

	// from "source"
	if p.check(FROM) {
		p.advance()
		if p.check(STRING) {
			si.Source = p.advance().Literal
		} else if p.check(IDENT) {
			si.Source = p.advance().Literal
		}
	}

	// optional parameter overrides: ... end
	p.skipNL()
	for !p.check(END) && !p.check(EOF) {
		if p.check(IDENT) {
			f := p.parseField()
			if f != nil {
				si.Params = append(si.Params, f)
			}
		} else if p.check(NEWLINE) {
			p.advance()
		} else {
			break
		}
		p.skipNL()
	}
	// end is optional if no params were given (single-line snack import)
	if p.check(END) {
		p.advance()
	}
	return si
}
