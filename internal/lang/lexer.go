package lang

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexer tokenizes a Scute source file
type Lexer struct {
	src      string
	filename string
	pos      int
	line     int
	col      int
	errors   []error
}

// NewLexer creates a lexer for the given source text
func NewLexer(src, filename string) *Lexer {
	return &Lexer{src: src, filename: filename, line: 1, col: 1}
}

// Tokenize processes the entire source and returns all tokens
func (l *Lexer) Tokenize() ([]Token, []error) {
	var tokens []Token
	for {
		tok := l.next()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}
	return tokens, l.errors
}

// ─── Navigation ───────────────────────────────────────────────────────────────

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos:])
	return r
}

func (l *Lexer) peekN(n int) rune {
	p := l.pos
	for i := 0; i < n; i++ {
		if p >= len(l.src) {
			return 0
		}
		_, sz := utf8.DecodeRuneInString(l.src[p:])
		p += sz
	}
	if p >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[p:])
	return r
}

func (l *Lexer) advance() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	l.pos += size
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) skipWS() {
	for {
		r := l.peek()
		if r == ' ' || r == '\t' || r == '\r' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) tok(t TokenType, lit string, line, col int) Token {
	return Token{Type: t, Literal: lit, Line: line, Col: col}
}

func (l *Lexer) errorf(line, col int, format string, args ...interface{}) {
	l.errors = append(l.errors, &ParseError{
		Pos:     Pos{File: l.filename, Line: line, Col: col},
		Message: fmt.Sprintf(format, args...),
	})
}

// ─── Main Dispatch ────────────────────────────────────────────────────────────

func (l *Lexer) next() Token {
	l.skipWS()

	if l.pos >= len(l.src) {
		return l.tok(EOF, "", l.line, l.col)
	}

	line, col := l.line, l.col
	r := l.peek()

	// Newlines are significant (block structure)
	if r == '\n' {
		l.advance()
		return l.tok(NEWLINE, "\n", line, col)
	}

	// Comments: # or //
	if r == '#' && l.peekN(1) != '{' { // # not followed by { is a comment
		return l.lineComment(line, col)
	}
	if r == '/' && l.peekN(1) == '/' {
		return l.lineComment(line, col)
	}
	if r == '/' && l.peekN(1) == '*' {
		return l.blockComment(line, col)
	}

	// Identifiers and keywords
	if unicode.IsLetter(r) || r == '_' {
		return l.ident(line, col)
	}

	// Numbers (including negative)
	if unicode.IsDigit(r) {
		return l.number(line, col)
	}

	// Strings
	if r == '"' {
		return l.doubleString(line, col)
	}
	if r == '`' {
		return l.rawString(line, col)
	}

	l.advance()
	switch r {
	case '@':
		return l.tok(AT, "@", line, col)
	case '[':
		return l.tok(LBRACKET, "[", line, col)
	case ']':
		return l.tok(RBRACKET, "]", line, col)
	case '(':
		return l.tok(LPAREN, "(", line, col)
	case ')':
		return l.tok(RPAREN, ")", line, col)
	case ',':
		return l.tok(COMMA, ",", line, col)
	case '.':
		if l.peek() == '.' {
			l.advance()
			return l.tok(DOTDOT, "..", line, col)
		}
		return l.tok(DOT, ".", line, col)
	case ':':
		return l.tok(COLON, ":", line, col)
	case '|':
		if l.peek() == '|' {
			l.advance()
			return l.tok(OR, "||", line, col)
		}
		return l.tok(PIPE, "|", line, col)
	case '!':
		if l.peek() == '=' {
			l.advance()
			return l.tok(NEQ, "!=", line, col)
		}
		// spawn! — handled inside ident()
		return l.tok(BANG, "!", line, col)
	case '=':
		if l.peek() == '=' {
			l.advance()
			return l.tok(EQ, "==", line, col)
		}
		// bare = is not valid in Scute; give a helpful error
		l.errorf(line, col, "unexpected '=' — Scute uses ':' for assignment, not '='")
		return l.tok(ILLEGAL, "=", line, col)
	case '<':
		if l.peek() == '=' {
			l.advance()
			return l.tok(LTE, "<=", line, col)
		}
		return l.tok(LT, "<", line, col)
	case '>':
		if l.peek() == '=' {
			l.advance()
			return l.tok(GTE, ">=", line, col)
		}
		return l.tok(GT, ">", line, col)
	case '&':
		if l.peek() == '&' {
			l.advance()
			return l.tok(AND, "&&", line, col)
		}
	case '+':
		return l.tok(PLUS, "+", line, col)
	case '-':
		// Negative number literal
		if unicode.IsDigit(l.peek()) {
			l.pos-- // back up so number() sees the -
			l.col--
			return l.number(line, col)
		}
		return l.tok(MINUS, "-", line, col)
	case '*':
		return l.tok(STAR, "*", line, col)
	case '/':
		return l.tok(SLASH, "/", line, col)
	case '%':
		return l.tok(PERCENT, "%", line, col)
	case '?':
		return l.tok(QUESTION, "?", line, col)
	}

	l.errorf(line, col, "unexpected character: %q", r)
	return l.tok(ILLEGAL, string(r), line, col)
}

// ─── Token Readers ────────────────────────────────────────────────────────────

func (l *Lexer) lineComment(line, col int) Token {
	start := l.pos
	for l.peek() != '\n' && l.peek() != 0 {
		l.advance()
	}
	return l.tok(COMMENT, strings.TrimSpace(l.src[start:l.pos]), line, col)
}

func (l *Lexer) blockComment(line, col int) Token {
	l.advance() // /
	l.advance() // *
	start := l.pos
	for {
		if l.pos >= len(l.src) {
			l.errorf(line, col, "unterminated block comment")
			break
		}
		if l.peek() == '*' && l.peekN(1) == '/' {
			break
		}
		l.advance()
	}
	lit := strings.TrimSpace(l.src[start:l.pos])
	l.advance() // *
	l.advance() // /
	return l.tok(COMMENT, lit, line, col)
}

func (l *Lexer) ident(line, col int) Token {
	start := l.pos
	for {
		r := l.peek()
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			l.advance()
		} else {
			break
		}
	}
	lit := l.src[start:l.pos]

	// Handle spawn! — protected spawn keyword
	tt := LookupIdent(lit)
	if tt == SPAWN && l.peek() == '!' {
		l.advance()
		return l.tok(PROTECTED, "spawn!", line, col)
	}
	return l.tok(tt, lit, line, col)
}

func (l *Lexer) number(line, col int) Token {
	start := l.pos
	if l.peek() == '-' {
		l.advance()
	}
	for unicode.IsDigit(l.peek()) {
		l.advance()
	}
	if l.peek() == '.' && unicode.IsDigit(l.peekN(1)) {
		l.advance()
		for unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}
	// Scientific notation
	if l.peek() == 'e' || l.peek() == 'E' {
		l.advance()
		if l.peek() == '+' || l.peek() == '-' {
			l.advance()
		}
		for unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}
	return l.tok(NUMBER, l.src[start:l.pos], line, col)
}

// doubleString reads a double-quoted string. Scute uses #{...} for interpolation
// (not ${...} like most languages). The lexer encodes interpolation segments
// with NUL-separated markers that the parser splits apart.
func (l *Lexer) doubleString(line, col int) Token {
	l.advance() // opening "
	var sb strings.Builder
	for {
		r := l.peek()
		if r == 0 {
			l.errorf(line, col, "unterminated string")
			break
		}
		if r == '"' {
			l.advance()
			break
		}
		if r == '\\' {
			l.advance()
			esc := l.advance()
			switch esc {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '"':
				sb.WriteRune('"')
			case '\\':
				sb.WriteRune('\\')
			case '#':
				sb.WriteRune('#') // escaped interpolation start
			default:
				sb.WriteRune('\\')
				sb.WriteRune(esc)
			}
			continue
		}
		// Scute interpolation: #{ expr }
		if r == '#' && l.peekN(1) == '{' {
			l.advance() // #
			l.advance() // {
			sb.WriteString("\x00INTERP\x00")
			depth := 1
			for depth > 0 && l.peek() != 0 {
				ch := l.advance()
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				if depth > 0 {
					sb.WriteRune(ch)
				}
			}
			sb.WriteString("\x00END\x00")
			continue
		}
		sb.WriteRune(l.advance())
	}
	return l.tok(STRING, sb.String(), line, col)
}

// rawString reads a backtick-quoted string — no escapes, no interpolation
func (l *Lexer) rawString(line, col int) Token {
	l.advance() // opening `
	start := l.pos
	for l.peek() != '`' && l.peek() != 0 {
		l.advance()
	}
	lit := l.src[start:l.pos]
	if l.peek() == '`' {
		l.advance()
	}
	return l.tok(STRING, lit, line, col)
}
