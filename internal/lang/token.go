// Package lang implements Scute — Gecko's native IaC language.
//
// Scute is purpose-built for infrastructure. It is end-terminated (no braces),
// colon-assigned (no equals signs), uses gecko-themed vocabulary, and has
// first-class support for secrets, environment variables, and cross-resource
// references via the @ sigil.
//
// A Scute program looks like:
//
//	territory "my-cluster"
//	  workspace: env("GECKO_WORKSPACE") | "dev"
//	end
//
//	habitat "k8s"
//	  kubeconfig: "~/.kube/config"
//	end
//
//	mark replicas number: 3
//
//	camouflage
//	  prefix: "#{app}-#{workspace}"
//	end
//
//	spawn "k8s:deployment" as "api"
//	  needs:    @monitoring
//	  replicas: replicas
//	  when debug
//	    log_level: "debug"
//	  else
//	    log_level: "info"
//	  end
//	end
//
//	signal "endpoint"
//	  value: "http://#{@api.ip}:8080"
//	end
package lang

import "fmt"

// TokenType identifies a lexical token in the Scute language
type TokenType int

const (
	ILLEGAL TokenType = iota
	EOF
	COMMENT
	NEWLINE

	// ── Literals ──────────────────────────────────────────────────────────────
	IDENT  // any identifier
	STRING // "..." or `...`
	NUMBER // integer or float
	BOOL   // true / false
	NULL   // null

	// ── Gecko-Themed Keywords ─────────────────────────────────────────────────
	TERRITORY  // territory  → project-level metadata block
	HABITAT    // habitat    → provider configuration block
	SPAWN      // spawn      → resource declaration
	AS         // as         → names a spawn: spawn "type" as "name"
	ACROSS     // across     → iteration: spawn "type" as "x" across [list]
	ITEM       // item       → current element inside an across block
	MARK       // mark       → variable declaration
	CAMOUFLAGE // camouflage → locally-computed values (like locals)
	SIGNAL     // signal     → output declaration
	NEEDS      // needs      → explicit dependency on a @resource
	END        // end        → block terminator (no braces in Scute)
	WHEN       // when       → conditional block
	UNLESS     // unless     → negative conditional
	ELSE       // else       → alternative branch in when/unless
	PROTECTED  // protected  → marks a spawn as destroy-protected
	STORE      // store      → state backend configuration block

	// ── Snack Module System ───────────────────────────────────────────────────
	SNACK  // snack  — consume a module: snack "name" from "path/url"
	FROM   // from   — source specifier in snack declaration
	EXPORT // export — define a reusable module in a .snack file
	PARAM  // param  — parameter declaration inside an export block
	EMIT   // emit   — output signal declaration inside an export block

	// ── Type Keywords (used in mark declarations) ─────────────────────────────
	TYPE_STRING // string
	TYPE_NUMBER // number
	TYPE_BOOL   // bool
	TYPE_LIST   // list
	TYPE_MAP    // map
	TYPE_ANY    // any

	// ── Operators ─────────────────────────────────────────────────────────────
	COLON    // :   assignment operator
	PIPE     // |   null-coalesce: expr | fallback
	AT       // @   cross-resource reference sigil: @monitoring.name
	EQ       // ==
	NEQ      // !=
	LT       // <
	GT       // >
	LTE      // <=
	GTE      // >=
	AND      // && or "and"
	OR       // || or "or"
	BANG     // !  or "not"
	PLUS     // +
	MINUS    // -
	STAR     // *
	SLASH    // /
	PERCENT  // %
	QUESTION // ?   ternary condition operator
	DOTDOT   // ..  range: 1..5

	// ── Delimiters ────────────────────────────────────────────────────────────
	LBRACKET // [
	RBRACKET // ]
	LPAREN   // (
	RPAREN   // )
	COMMA    // ,
	DOT      // .
)

var tokenNames = map[TokenType]string{
	ILLEGAL: "ILLEGAL", EOF: "EOF", COMMENT: "COMMENT", NEWLINE: "NEWLINE",
	IDENT: "IDENT", STRING: "STRING", NUMBER: "NUMBER", BOOL: "BOOL", NULL: "null",
	TERRITORY: "territory", HABITAT: "habitat", SPAWN: "spawn",
	AS: "as", ACROSS: "across", ITEM: "item", MARK: "mark",
	CAMOUFLAGE: "camouflage", SIGNAL: "signal", NEEDS: "needs",
	END: "end", WHEN: "when", UNLESS: "unless", ELSE: "else", PROTECTED: "protected",
	STORE: "store",
	TYPE_STRING: "string", TYPE_NUMBER: "number", TYPE_BOOL: "bool",
	TYPE_LIST: "list", TYPE_MAP: "map", TYPE_ANY: "any",
	COLON: ":", PIPE: "|", AT: "@",
	EQ: "==", NEQ: "!=", LT: "<", GT: ">", LTE: "<=", GTE: ">=",
	SNACK: "snack", FROM: "from", EXPORT: "export", PARAM: "param", EMIT: "emit",
	AND: "&&", OR: "||", BANG: "!", PLUS: "+", MINUS: "-",
	STAR: "*", SLASH: "/", PERCENT: "%", QUESTION: "?", DOTDOT: "..",
	LBRACKET: "[", RBRACKET: "]", LPAREN: "(", RPAREN: ")",
	COMMA: ",", DOT: ".",
}

func (t TokenType) String() string {
	if s, ok := tokenNames[t]; ok {
		return s
	}
	return fmt.Sprintf("Token(%d)", int(t))
}

var keywords = map[string]TokenType{
	"territory":  TERRITORY,
	"habitat":    HABITAT,
	"spawn":      SPAWN,
	"as":         AS,
	"across":     ACROSS,
	"item":       ITEM,
	"mark":       MARK,
	"camouflage": CAMOUFLAGE,
	"signal":     SIGNAL,
	"needs":      NEEDS,
	"end":        END,
	"when":       WHEN,
	"unless":     UNLESS,
	"else":       ELSE,
	"protected":  PROTECTED,
	"true":       BOOL,
	"false":      BOOL,
	"null":       NULL,
	"string":     TYPE_STRING,
	"number":     TYPE_NUMBER,
	"bool":       TYPE_BOOL,
	"list":       TYPE_LIST,
	"map":        TYPE_MAP,
	"any":        TYPE_ANY,
	"and":        AND,
	"or":         OR,
	"not":        BANG,
	"snack":      SNACK,
	"from":       FROM,
	"export":     EXPORT,
	"param":      PARAM,
	"emit":       EMIT,
	"store":      STORE,
}

func LookupIdent(ident string) TokenType {
	if tt, ok := keywords[ident]; ok {
		return tt
	}
	return IDENT
}

// Token is a lexical unit with position info
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

func (t Token) String() string {
	return fmt.Sprintf("Token{%s %q %d:%d}", t.Type, t.Literal, t.Line, t.Col)
}

// Pos is a source location for error messages
type Pos struct {
	File string
	Line int
	Col  int
}

func (p Pos) String() string {
	if p.File != "" {
		return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
	}
	return fmt.Sprintf("line %d, col %d", p.Line, p.Col)
}

// ParseError is a syntax error with location
type ParseError struct {
	Pos     Pos
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Message)
}

// EvalError is a runtime evaluation error with location
type EvalError struct {
	Pos     Pos
	Message string
}

func (e *EvalError) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Message)
}
