package expr

import (
	"fmt"
	"strings"
	"unicode"
)

type Lexer struct {
	input string
	pos   int
	len   int
}

func NewLexer(input string) *Lexer {
	return &Lexer{
		input: input,
		pos:   0,
		len:   len(input),
	}
}

func (l *Lexer) peek() byte {
	if l.pos >= l.len {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) peekNext() byte {
	if l.pos+1 >= l.len {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *Lexer) next() byte {
	if l.pos >= l.len {
		return 0
	}
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < l.len {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.pos++
		} else {
			break
		}
	}
}

func (l *Lexer) NextToken() (Token, error) {
	l.skipWhitespace()
	if l.pos >= l.len {
		return Token{Type: TokenEOF, Pos: l.pos}, nil
	}

	start := l.pos
	ch := l.next()

	switch ch {
	case '(':
		return Token{Type: TokenLParen, Value: "(", Pos: start}, nil
	case ')':
		return Token{Type: TokenRParen, Value: ")", Pos: start}, nil
	case '[':
		return Token{Type: TokenLBracket, Value: "[", Pos: start}, nil
	case ']':
		return Token{Type: TokenRBracket, Value: "]", Pos: start}, nil
	case '.':
		return Token{Type: TokenDot, Value: ".", Pos: start}, nil
	case ',':
		return Token{Type: TokenComma, Value: ",", Pos: start}, nil
	case '*':
		return Token{Type: TokenStar, Value: "*", Pos: start}, nil
	case '!':
		if l.peek() == '=' {
			l.next()
			return Token{Type: TokenNeq, Value: "!=", Pos: start}, nil
		}
		return Token{Type: TokenNot, Value: "!", Pos: start}, nil
	case '=':
		if l.peek() == '=' {
			l.next()
			return Token{Type: TokenEq, Value: "==", Pos: start}, nil
		}
		return Token{Type: TokenError, Value: "=", Pos: start}, fmt.Errorf("unexpected character '=' at pos %d, did you mean '=='?", start)
	case '<':
		if l.peek() == '=' {
			l.next()
			return Token{Type: TokenLte, Value: "<=", Pos: start}, nil
		}
		return Token{Type: TokenLt, Value: "<", Pos: start}, nil
	case '>':
		if l.peek() == '=' {
			l.next()
			return Token{Type: TokenGte, Value: ">=", Pos: start}, nil
		}
		return Token{Type: TokenGt, Value: ">", Pos: start}, nil
	case '&':
		if l.peek() == '&' {
			l.next()
			return Token{Type: TokenAnd, Value: "&&", Pos: start}, nil
		}
		return Token{Type: TokenError, Value: "&", Pos: start}, fmt.Errorf("unexpected character '&' at pos %d, did you mean '&&'?", start)
	case '|':
		if l.peek() == '|' {
			l.next()
			return Token{Type: TokenOr, Value: "||", Pos: start}, nil
		}
		return Token{Type: TokenError, Value: "|", Pos: start}, fmt.Errorf("unexpected character '|' at pos %d, did you mean '||'?", start)
	case '\'':
		// Single-quoted string literal with '' escaping
		var sb strings.Builder
		for {
			if l.pos >= l.len {
				return Token{Type: TokenError, Pos: start}, fmt.Errorf("unclosed single-quoted string starting at pos %d", start)
			}
			c := l.next()
			if c == '\'' {
				if l.peek() == '\'' {
					// Escaped single quote
					l.next()
					sb.WriteByte('\'')
				} else {
					// End of string
					break
				}
			} else {
				sb.WriteByte(c)
			}
		}
		return Token{Type: TokenString, Value: sb.String(), Pos: start}, nil
	}

	// Negative number or number
	if ch == '-' && isDigit(l.peek()) {
		numStr := l.readNumber(string(ch))
		return Token{Type: TokenNumber, Value: numStr, Pos: start}, nil
	}

	if isDigit(ch) {
		numStr := l.readNumber(string(ch))
		return Token{Type: TokenNumber, Value: numStr, Pos: start}, nil
	}

	// Identifiers (including true, false, null)
	if isIdentStart(ch) {
		var sb strings.Builder
		sb.WriteByte(ch)
		for l.pos < l.len && isIdentPart(l.peek()) {
			sb.WriteByte(l.next())
		}
		val := sb.String()
		lowerVal := strings.ToLower(val)
		switch lowerVal {
		case "true":
			return Token{Type: TokenBool, Value: "true", Pos: start}, nil
		case "false":
			return Token{Type: TokenBool, Value: "false", Pos: start}, nil
		case "null":
			return Token{Type: TokenNull, Value: "null", Pos: start}, nil
		case "nan":
			return Token{Type: TokenNumber, Value: "NaN", Pos: start}, nil
		case "infinity":
			return Token{Type: TokenNumber, Value: "Infinity", Pos: start}, nil
		default:
			return Token{Type: TokenIdent, Value: val, Pos: start}, nil
		}
	}

	return Token{Type: TokenError, Value: string(ch), Pos: start}, fmt.Errorf("unexpected character %q at pos %d", ch, start)
}

func (l *Lexer) readNumber(prefix string) string {
	var sb strings.Builder
	sb.WriteString(prefix)
	hasDot := false
	hasExp := false

	// Check if hex: 0x...
	if prefix == "0" && (l.peek() == 'x' || l.peek() == 'X') {
		sb.WriteByte(l.next())
		for l.pos < l.len && isHexDigit(l.peek()) {
			sb.WriteByte(l.next())
		}
		return sb.String()
	}

	for l.pos < l.len {
		c := l.peek()
		if isDigit(c) {
			sb.WriteByte(l.next())
		} else if c == '.' && !hasDot && !hasExp && isDigit(l.peekNext()) {
			hasDot = true
			sb.WriteByte(l.next())
		} else if (c == 'e' || c == 'E') && !hasExp {
			hasExp = true
			sb.WriteByte(l.next())
			if l.peek() == '+' || l.peek() == '-' {
				sb.WriteByte(l.next())
			}
		} else {
			break
		}
	}
	return sb.String()
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isIdentStart(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || ch == '_'
}

func isIdentPart(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_' || ch == '-'
}

// TokenizeAll tokenizes the entire input into a slice.
func Tokenize(input string) ([]Token, error) {
	lex := NewLexer(input)
	var tokens []Token
	for {
		tok, err := lex.NextToken()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return tokens, nil
}
