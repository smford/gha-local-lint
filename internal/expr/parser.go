package expr

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Parser struct {
	tokens []Token
	pos    int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peek() Token {
	if p.pos+1 >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos+1]
}

func (p *Parser) advance() Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) match(types ...TokenType) bool {
	curr := p.current().Type
	for _, t := range types {
		if curr == t {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) expect(expected TokenType) (Token, error) {
	curr := p.current()
	if curr.Type != expected {
		return curr, fmt.Errorf("expected token %s but got %s at pos %d", expected, curr.Type, curr.Pos)
	}
	p.advance()
	return curr, nil
}

// Parse parses a full expression.
func Parse(input string) (Node, error) {
	// Strip optional leading/trailing ${{ and }}
	cleaned := strings.TrimSpace(input)
	if strings.HasPrefix(cleaned, "${{") && strings.HasSuffix(cleaned, "}}") {
		cleaned = strings.TrimSpace(cleaned[3 : len(cleaned)-2])
	}
	if cleaned == "" {
		return &LiteralNode{Value: ""}, nil
	}

	tokens, err := Tokenize(cleaned)
	if err != nil {
		return nil, err
	}

	parser := NewParser(tokens)
	node, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}

	if parser.current().Type != TokenEOF {
		return nil, fmt.Errorf("unexpected token %s (%q) at pos %d after valid expression",
			parser.current().Type, parser.current().Value, parser.current().Pos)
	}

	return node, nil
}

func (p *Parser) parseExpression() (Node, error) {
	return p.parseLogicalOr()
}

func (p *Parser) parseLogicalOr() (Node, error) {
	left, err := p.parseLogicalAnd()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenOr {
		p.advance()
		right, err := p.parseLogicalAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Op: "||", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseLogicalAnd() (Node, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenAnd {
		p.advance()
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Op: "&&", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseEquality() (Node, error) {
	left, err := p.parseRelational()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenEq || p.current().Type == TokenNeq {
		op := p.advance().Value
		right, err := p.parseRelational()
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseRelational() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenLt || p.current().Type == TokenLte ||
		p.current().Type == TokenGt || p.current().Type == TokenGte {
		op := p.advance().Value
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseUnary() (Node, error) {
	if p.current().Type == TokenNot {
		p.advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryNode{Op: "!", Expr: expr}, nil
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() (Node, error) {
	node, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		switch p.current().Type {
		case TokenDot:
			p.advance()
			if p.current().Type == TokenIdent {
				prop := p.advance().Value
				node = &MemberNode{Object: node, Property: prop}
			} else if p.current().Type == TokenStar {
				p.advance()
				node = &MemberNode{Object: node, Property: "*"}
			} else {
				return nil, fmt.Errorf("expected property name after '.' at pos %d, got %s", p.current().Pos, p.current().Type)
			}
		case TokenLBracket:
			p.advance()
			indexExpr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TokenRBracket); err != nil {
				return nil, err
			}
			node = &IndexNode{Object: node, Index: indexExpr}
		default:
			return node, nil
		}
	}
}

func (p *Parser) parsePrimary() (Node, error) {
	tok := p.current()

	switch tok.Type {
	case TokenString:
		p.advance()
		return &LiteralNode{Value: tok.Value}, nil

	case TokenNumber:
		p.advance()
		if tok.Value == "NaN" {
			return &LiteralNode{Value: math.NaN()}, nil
		}
		if tok.Value == "Infinity" {
			return &LiteralNode{Value: math.Inf(1)}, nil
		}
		val, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q at pos %d: %w", tok.Value, tok.Pos, err)
		}
		return &LiteralNode{Value: val}, nil

	case TokenBool:
		p.advance()
		val := strings.ToLower(tok.Value) == "true"
		return &LiteralNode{Value: val}, nil

	case TokenNull:
		p.advance()
		return &LiteralNode{Value: nil}, nil

	case TokenStar:
		p.advance()
		return &StarNode{}, nil

	case TokenLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil

	case TokenIdent:
		name := p.advance().Value
		// Check if function call
		if p.current().Type == TokenLParen {
			p.advance()
			var args []Node
			if p.current().Type != TokenRParen {
				for {
					arg, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.current().Type == TokenComma {
						p.advance()
						continue
					}
					break
				}
			}
			if _, err := p.expect(TokenRParen); err != nil {
				return nil, err
			}
			return &CallNode{Name: name, Args: args}, nil
		}
		return &IdentNode{Name: name}, nil

	default:
		return nil, fmt.Errorf("unexpected token %s (%q) at pos %d", tok.Type, tok.Value, tok.Pos)
	}
}
