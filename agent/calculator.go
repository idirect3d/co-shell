// Author: L.Shuang
// Created: 2026-06-05
// Last Modified: 2026-06-05
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// tokenType represents the type of a lexical token.
type tokenType int

const (
	tokNumber tokenType = iota
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokPercent
	tokCaret
	tokLParen
	tokRParen
	tokComma
	tokIdent
	tokEOF
	tokInvalid
)

// token represents a single lexical token.
type token struct {
	typ tokenType
	val string
	num float64
}

// lexer converts an expression string into a stream of tokens.
type lexer struct {
	input string
	pos   int
}

func newLexer(input string) *lexer {
	return &lexer{input: input}
}

// skipWhitespace advances past whitespace characters.
func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

// peek returns the current character without consuming it.
func (l *lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

// next consumes and returns the next character.
func (l *lexer) next() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	ch := l.input[l.pos]
	l.pos++
	return ch
}

// nextToken returns the next token from the input.
func (l *lexer) nextToken() token {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return token{typ: tokEOF}
	}

	ch := l.peek()

	// Numbers: integer or floating-point
	if ch == '.' || unicode.IsDigit(rune(ch)) {
		start := l.pos
		hasDot := false
		for l.pos < len(l.input) {
			c := l.input[l.pos]
			if c == '.' {
				if hasDot {
					break
				}
				hasDot = true
				l.pos++
			} else if unicode.IsDigit(rune(c)) {
				l.pos++
			} else {
				break
			}
		}
		val := l.input[start:l.pos]
		num, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return token{typ: tokInvalid, val: val}
		}
		return token{typ: tokNumber, val: val, num: num}
	}

	// Identifiers: function names or constants
	if unicode.IsLetter(rune(ch)) || ch == '_' {
		start := l.pos
		for l.pos < len(l.input) && (unicode.IsLetter(rune(l.input[l.pos])) || unicode.IsDigit(rune(l.input[l.pos])) || l.input[l.pos] == '_') {
			l.pos++
		}
		val := l.input[start:l.pos]
		return token{typ: tokIdent, val: val}
	}

	// Single-character operators
	l.next()
	switch ch {
	case '+':
		return token{typ: tokPlus, val: "+"}
	case '-':
		return token{typ: tokMinus, val: "-"}
	case '*':
		return token{typ: tokStar, val: "*"}
	case '/':
		return token{typ: tokSlash, val: "/"}
	case '%':
		return token{typ: tokPercent, val: "%"}
	case '^':
		return token{typ: tokCaret, val: "^"}
	case '(':
		return token{typ: tokLParen, val: "("}
	case ')':
		return token{typ: tokRParen, val: ")"}
	case ',':
		return token{typ: tokComma, val: ","}
	default:
		return token{typ: tokInvalid, val: string(ch)}
	}
}

// parser implements a recursive descent parser for arithmetic expressions.
type parser struct {
	lex  *lexer
	cur  token
	peek token
}

func newParser(input string) *parser {
	p := &parser{lex: newLexer(input)}
	p.advance() // load first token
	p.advance() // load peek token
	return p
}

func (p *parser) advance() {
	p.cur = p.peek
	p.peek = p.lex.nextToken()
}

// expr parses addition and subtraction: expr = term (('+' | '-') term)*
func (p *parser) expr() (float64, error) {
	result, err := p.term()
	if err != nil {
		return 0, err
	}

	for p.cur.typ == tokPlus || p.cur.typ == tokMinus {
		op := p.cur.typ
		p.advance()
		right, err := p.term()
		if err != nil {
			return 0, err
		}
		switch op {
		case tokPlus:
			result += right
		case tokMinus:
			result -= right
		}
	}

	return result, nil
}

// term parses multiplication, division, and modulo: term = factor (('*' | '/' | '%') factor)*
func (p *parser) term() (float64, error) {
	result, err := p.power()
	if err != nil {
		return 0, err
	}

	for p.cur.typ == tokStar || p.cur.typ == tokSlash || p.cur.typ == tokPercent {
		op := p.cur.typ
		p.advance()
		right, err := p.power()
		if err != nil {
			return 0, err
		}
		switch op {
		case tokStar:
			result *= right
		case tokSlash:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			result /= right
		case tokPercent:
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			result = float64(int64(result) % int64(right))
		}
	}

	return result, nil
}

// power parses exponentiation: power = unary ('^' power)*
// Note: exponentiation is right-associative (e.g., 2^3^4 = 2^(3^4)).
// Unary minus/plus in the exponent binds tighter than '^':
// -2 ^ 2 means -(2 ^ 2) = -4, NOT (-2) ^ 2 = 4.
// To get (-2)^2, use parentheses: (-2) ^ 2.
func (p *parser) power() (float64, error) {
	// Handle unary operators before the base.
	// These are applied AFTER exponentiation, so -2^2 = -(2^2).
	negateCount := 0
	for p.cur.typ == tokPlus || p.cur.typ == tokMinus {
		if p.cur.typ == tokMinus {
			negateCount++
		}
		p.advance()
	}

	result, err := p.call()
	if err != nil {
		return 0, err
	}

	if p.cur.typ == tokCaret {
		p.advance()
		right, err := p.power() // right-associative
		if err != nil {
			return 0, err
		}
		result = math.Pow(result, right)
	}

	// Apply unary operators after exponentiation: -(x ^ y)
	if negateCount%2 == 1 {
		result = -result
	}

	return result, nil
}

// unary parses unary operators: unary = ('+' | '-')* call
// Note: Only handles unary operators at the call level (not in power context).
// The call from term → power handles unary operators internally for correct
// precedence with exponentiation.
func (p *parser) unary() (float64, error) {
	// Handle consecutive unary operators
	negateCount := 0
	for p.cur.typ == tokPlus || p.cur.typ == tokMinus {
		if p.cur.typ == tokMinus {
			negateCount++
		}
		p.advance()
	}

	result, err := p.call()
	if err != nil {
		return 0, err
	}

	if negateCount%2 == 1 {
		result = -result
	}
	return result, nil
}

// call parses function calls: call = ident '(' expr (',' expr)* ')' | atom
func (p *parser) call() (float64, error) {
	if p.cur.typ == tokIdent {
		name := p.cur.val
		p.advance()

		// If next token is '(', it's a function call
		if p.cur.typ == tokLParen {
			p.advance()

			// Parse arguments
			var args []float64
			if p.cur.typ != tokRParen {
				arg, err := p.expr()
				if err != nil {
					return 0, err
				}
				args = append(args, arg)

				for p.cur.typ == tokComma {
					p.advance()
					arg, err := p.expr()
					if err != nil {
						return 0, err
					}
					args = append(args, arg)
				}
			}

			if p.cur.typ != tokRParen {
				return 0, fmt.Errorf("expected ')' after function arguments")
			}
			p.advance()

			return callFunction(name, args)
		}

		// Not a function call — check for constants
		return constantValue(name)
	}

	return p.atom()
}

// atom parses numbers and parenthesized expressions.
func (p *parser) atom() (float64, error) {
	switch p.cur.typ {
	case tokNumber:
		val := p.cur.num
		p.advance()
		return val, nil
	case tokLParen:
		p.advance()
		result, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.cur.typ != tokRParen {
			return 0, fmt.Errorf("expected ')'")
		}
		p.advance()
		return result, nil
	case tokMinus:
		// Unary minus
		p.advance()
		result, err := p.atom()
		if err != nil {
			return 0, err
		}
		return -result, nil
	case tokPlus:
		// Unary plus
		p.advance()
		return p.atom()
	default:
		return 0, fmt.Errorf("unexpected token: %q", p.cur.val)
	}
}

// constantValue returns the numeric value for a known constant name.
func constantValue(name string) (float64, error) {
	switch strings.ToLower(name) {
	case "pi", "π":
		return math.Pi, nil
	case "e":
		return math.E, nil
	default:
		return 0, fmt.Errorf("unknown identifier: %q (not a function or constant)", name)
	}
}

// callFunction evaluates a function call with given arguments.
func callFunction(name string, args []float64) (float64, error) {
	switch strings.ToLower(name) {
	case "sin":
		if len(args) != 1 {
			return 0, fmt.Errorf("sin() requires exactly 1 argument, got %d", len(args))
		}
		return math.Sin(args[0]), nil
	case "cos":
		if len(args) != 1 {
			return 0, fmt.Errorf("cos() requires exactly 1 argument, got %d", len(args))
		}
		return math.Cos(args[0]), nil
	case "tan":
		if len(args) != 1 {
			return 0, fmt.Errorf("tan() requires exactly 1 argument, got %d", len(args))
		}
		return math.Tan(args[0]), nil
	case "asin":
		if len(args) != 1 {
			return 0, fmt.Errorf("asin() requires exactly 1 argument, got %d", len(args))
		}
		if args[0] < -1 || args[0] > 1 {
			return 0, fmt.Errorf("asin() domain error: argument must be in [-1, 1], got %f", args[0])
		}
		return math.Asin(args[0]), nil
	case "acos":
		if len(args) != 1 {
			return 0, fmt.Errorf("acos() requires exactly 1 argument, got %d", len(args))
		}
		if args[0] < -1 || args[0] > 1 {
			return 0, fmt.Errorf("acos() domain error: argument must be in [-1, 1], got %f", args[0])
		}
		return math.Acos(args[0]), nil
	case "atan":
		if len(args) != 1 {
			return 0, fmt.Errorf("atan() requires exactly 1 argument, got %d", len(args))
		}
		return math.Atan(args[0]), nil
	case "log":
		if len(args) != 1 {
			return 0, fmt.Errorf("log() requires exactly 1 argument, got %d", len(args))
		}
		if args[0] <= 0 {
			return 0, fmt.Errorf("log() domain error: argument must be positive, got %f", args[0])
		}
		return math.Log10(args[0]), nil
	case "ln":
		if len(args) != 1 {
			return 0, fmt.Errorf("ln() requires exactly 1 argument, got %d", len(args))
		}
		if args[0] <= 0 {
			return 0, fmt.Errorf("ln() domain error: argument must be positive, got %f", args[0])
		}
		return math.Log(args[0]), nil
	case "sqrt":
		if len(args) != 1 {
			return 0, fmt.Errorf("sqrt() requires exactly 1 argument, got %d", len(args))
		}
		if args[0] < 0 {
			return 0, fmt.Errorf("sqrt() domain error: argument must be non-negative, got %f", args[0])
		}
		return math.Sqrt(args[0]), nil
	case "abs":
		if len(args) != 1 {
			return 0, fmt.Errorf("abs() requires exactly 1 argument, got %d", len(args))
		}
		return math.Abs(args[0]), nil
	case "ceil":
		if len(args) != 1 {
			return 0, fmt.Errorf("ceil() requires exactly 1 argument, got %d", len(args))
		}
		return math.Ceil(args[0]), nil
	case "floor":
		if len(args) != 1 {
			return 0, fmt.Errorf("floor() requires exactly 1 argument, got %d", len(args))
		}
		return math.Floor(args[0]), nil
	case "round":
		if len(args) != 1 {
			return 0, fmt.Errorf("round() requires exactly 1 argument, got %d", len(args))
		}
		return math.Round(args[0]), nil
	default:
		return 0, fmt.Errorf("unknown function: %q", name)
	}
}

// evaluateExpressionTool is the LLM tool callback for evaluating mathematical expressions.
func (a *Agent) evaluateExpressionTool(ctx context.Context, args map[string]interface{}) (string, error) {
	expression, _ := args["expression"].(string)
	if expression == "" {
		return "", fmt.Errorf("expression is required")
	}
	result, err := evaluateExpression(expression)
	if err != nil {
		return "", fmt.Errorf("expression evaluation failed: %w", err)
	}
	return result, nil
}

// Evaluate parses and evaluates an arithmetic expression string,
// returning the result as a formatted string.
func evaluateExpression(expression string) (string, error) {
	p := newParser(expression)
	result, err := p.expr()
	if err != nil {
		return "", fmt.Errorf("cannot evaluate expression: %w", err)
	}

	// Check for unexpected trailing tokens
	if p.cur.typ != tokEOF {
		return "", fmt.Errorf("unexpected trailing content after expression: %q", p.cur.val)
	}

	// Format the result nicely
	if result == math.Trunc(result) && !math.IsInf(result, 0) && math.Abs(result) < 1e15 {
		return strconv.FormatFloat(result, 'f', 0, 64), nil
	}
	return strconv.FormatFloat(result, 'g', -1, 64), nil
}
