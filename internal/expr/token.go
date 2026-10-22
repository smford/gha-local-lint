package expr

import "fmt"

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError

	// Literals
	TokenIdent  // identifier, context name, function name
	TokenString // 'single-quoted string'
	TokenNumber // integer or float
	TokenBool   // true, false
	TokenNull   // null

	// Operators
	TokenEq  // ==
	TokenNeq // !=
	TokenLt  // <
	TokenLte // <=
	TokenGt  // >
	TokenGte // >=
	TokenAnd // &&
	TokenOr  // ||
	TokenNot // !

	// Punctuation
	TokenDot      // .
	TokenComma    // ,
	TokenLParen   // (
	TokenRParen   // )
	TokenLBracket // [
	TokenRBracket // ]
	TokenStar     // * (wildcard filter)
)

func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenError:
		return "ERROR"
	case TokenIdent:
		return "IDENT"
	case TokenString:
		return "STRING"
	case TokenNumber:
		return "NUMBER"
	case TokenBool:
		return "BOOL"
	case TokenNull:
		return "NULL"
	case TokenEq:
		return "=="
	case TokenNeq:
		return "!="
	case TokenLt:
		return "<"
	case TokenLte:
		return "<="
	case TokenGt:
		return ">"
	case TokenGte:
		return ">="
	case TokenAnd:
		return "&&"
	case TokenOr:
		return "||"
	case TokenNot:
		return "!"
	case TokenDot:
		return "."
	case TokenComma:
		return ","
	case TokenLParen:
		return "("
	case TokenRParen:
		return ")"
	case TokenLBracket:
		return "["
	case TokenRBracket:
		return "]"
	case TokenStar:
		return "*"
	default:
		return fmt.Sprintf("TOKEN(%d)", t)
	}
}

type Token struct {
	Type  TokenType
	Value string
	Pos   int // byte offset in input
}

func (t Token) String() string {
	return fmt.Sprintf("%s(%q)@%d", t.Type, t.Value, t.Pos)
}
