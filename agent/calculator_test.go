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
	"math"
	"testing"
)

// Helper: evaluate expression and expect a specific result string.
func evalExpect(t *testing.T, expr, expected string) {
	t.Helper()
	result, err := evaluateExpression(expr)
	if err != nil {
		t.Errorf("evaluateExpression(%q) unexpected error: %v", expr, err)
		return
	}
	if result != expected {
		t.Errorf("evaluateExpression(%q) = %q, want %q", expr, result, expected)
	}
}

// Helper: evaluate expression and expect an error.
func evalError(t *testing.T, expr string) {
	t.Helper()
	_, err := evaluateExpression(expr)
	if err == nil {
		t.Errorf("evaluateExpression(%q) expected error, got nil", expr)
	}
}

// Helper: evaluate expression and expect a result close to a float value.
func evalApprox(t *testing.T, expr string, expected float64, tol float64) {
	t.Helper()
	p := newParser(expr)
	result, err := p.expr()
	if err != nil {
		t.Errorf("newParser(%q).expr() unexpected error: %v", expr, err)
		return
	}
	if p.cur.typ != tokEOF {
		t.Errorf("newParser(%q): trailing tokens after expression: %q", expr, p.cur.val)
		return
	}
	diff := math.Abs(result - expected)
	if diff > tol {
		t.Errorf("newParser(%q).expr() = %v, want %v (diff=%v)", expr, result, expected, diff)
	}
}

// TestBasicArithmetic tests simple integer and float arithmetic.
func TestBasicArithmetic(t *testing.T) {
	tests := []struct {
		expr     string
		expected string
	}{
		{"1 + 2", "3"},
		{"3 - 1", "2"},
		{"4 * 5", "20"},
		{"10 / 3", "3.3333333333333335"},
		{"10 / 2", "5"},
		{"7 % 3", "1"},
		{"100 % 7", "2"},
		{"0 + 0", "0"},
		{"1 - 1", "0"},
		{"0 * 999", "0"},
	}
	for _, tc := range tests {
		evalExpect(t, tc.expr, tc.expected)
	}
}

// TestFloatArithmetic tests expressions with floating-point numbers.
func TestFloatArithmetic(t *testing.T) {
	tests := []struct {
		expr     string
		expected string
	}{
		{"1.5 + 2.5", "4"},
		{"3.2 - 1.1", "2.1"},
		{"2.5 * 3", "7.5"},
		{"7.5 / 2.5", "3"},
		{"1.0 / 3.0", "0.3333333333333333"},
		{"3.14159 * 2", "6.28318"},
	}
	for _, tc := range tests {
		evalExpect(t, tc.expr, tc.expected)
	}
	// IEEE 754 floating point: 0.1 + 0.2 = 0.30000000000000004
	evalApprox(t, "0.1 + 0.2", 0.3, 0.0001)
}

// TestUnaryOperators tests unary plus and minus.
func TestUnaryOperators(t *testing.T) {
	tests := []struct {
		expr     string
		expected string
	}{
		{"-5", "-5"},
		{"--5", "5"},
		{"---5", "-5"},
		{"+5", "5"},
		{"-0", "-0"},
		{"-3 + 4", "1"},
		{"-3 - 4", "-7"},
		{"+3 + +4", "7"},
		{"- - 3", "3"},
		{"(-5)", "-5"},
		{"-(3 + 4)", "-7"},
	}
	for _, tc := range tests {
		evalExpect(t, tc.expr, tc.expected)
	}
}

// TestOperatorPrecedence tests that operator precedence is correct.
func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		expr     string
		expected string
	}{
		{"3 + 4 * 2", "11"},    // * higher than +
		{"3 * 4 + 2", "14"},    // * before +
		{"10 - 4 - 3", "3"},    // left-assoc: (10-4)-3
		{"16 / 4 / 2", "2"},    // left-assoc: (16/4)/2
		{"16 / 4 * 2", "8"},    // left-assoc: (16/4)*2
		{"2 + 3 * 4 - 5", "9"}, // mixed
		{"10 % 3 + 2", "3"},    // % higher than +
		{"10 % 3 * 2", "2"},    // % and * same precedence, left-assoc: (10%3)*2
		{"-3 * 4", "-12"},      // unary before *
		{"3 ^ 2 ^ 3", "6561"},  // right-assoc: 3^(2^3)=3^8=6561, NOT (3^2)^3=729
	}
	for _, tc := range tests {
		evalExpect(t, tc.expr, tc.expected)
	}
}

// TestExponentiation tests power operations thoroughly.
func TestExponentiation(t *testing.T) {
	evalApprox(t, "2 ^ 10", 1024, 0.0001)
	evalApprox(t, "2 ^ 0", 1, 0.0001)
	evalApprox(t, "2 ^ -1", 0.5, 0.0001)
	evalApprox(t, "10 ^ 2", 100, 0.0001)
	evalApprox(t, "3 ^ 3", 27, 0.0001)
	evalApprox(t, "0.5 ^ 2", 0.25, 0.0001)
}

// TestParentheses tests grouping with parentheses.
func TestParentheses(t *testing.T) {
	tests := []struct {
		expr     string
		expected string
	}{
		{"(3 + 4) * 2", "14"},
		{"3 + (4 * 2)", "11"},
		{"(10 - 4) - 3", "3"},
		{"10 - (4 - 3)", "9"},
		{"((3 + 4) * 2)", "14"},
		{"(1 + (2 * (3 + 4)))", "15"},
	}
	for _, tc := range tests {
		evalExpect(t, tc.expr, tc.expected)
	}
}

// TestTrigonometricFunctions tests sin, cos, tan.
func TestTrigonometricFunctions(t *testing.T) {
	evalApprox(t, "sin(0)", 0, 0.0001)
	evalApprox(t, "sin(pi / 2)", 1, 0.0001)
	evalApprox(t, "sin(pi)", 0, 0.0001)
	evalApprox(t, "cos(0)", 1, 0.0001)
	evalApprox(t, "cos(pi)", -1, 0.0001)
	evalApprox(t, "tan(0)", 0, 0.0001)
	evalApprox(t, "tan(pi / 4)", 1, 0.0001)
}

// TestInverseTrigonometricFunctions tests asin, acos, atan.
func TestInverseTrigonometricFunctions(t *testing.T) {
	evalApprox(t, "asin(0)", 0, 0.0001)
	evalApprox(t, "asin(1)", math.Pi/2, 0.0001)
	evalApprox(t, "acos(1)", 0, 0.0001)
	evalApprox(t, "acos(0)", math.Pi/2, 0.0001)
	evalApprox(t, "atan(0)", 0, 0.0001)
	evalApprox(t, "atan(1)", math.Pi/4, 0.0001)
}

// TestLogarithmFunctions tests log (base 10) and ln (natural log).
func TestLogarithmFunctions(t *testing.T) {
	evalApprox(t, "log(100)", 2, 0.0001)
	evalApprox(t, "log(1000)", 3, 0.0001)
	evalApprox(t, "log(1)", 0, 0.0001)
	evalApprox(t, "ln(e)", 1, 0.0001)
	evalApprox(t, "ln(1)", 0, 0.0001)
}

// TestSqrtFunction tests square root.
func TestSqrtFunction(t *testing.T) {
	evalApprox(t, "sqrt(0)", 0, 0.0001)
	evalApprox(t, "sqrt(1)", 1, 0.0001)
	evalApprox(t, "sqrt(4)", 2, 0.0001)
	evalApprox(t, "sqrt(144)", 12, 0.0001)
	evalApprox(t, "sqrt(2)", math.Sqrt2, 0.0001)
}

// TestAbsFunction tests absolute value.
func TestAbsFunction(t *testing.T) {
	evalExpect(t, "abs(-5)", "5")
	evalExpect(t, "abs(5)", "5")
	evalExpect(t, "abs(0)", "0")
	evalApprox(t, "abs(-3.14)", 3.14, 0.0001)
}

// TestRoundingFunctions tests ceil, floor, round.
func TestRoundingFunctions(t *testing.T) {
	evalExpect(t, "ceil(3.1)", "4")
	evalExpect(t, "ceil(3.9)", "4")
	evalExpect(t, "ceil(-3.1)", "-3")
	evalExpect(t, "ceil(-3.9)", "-3")
	evalExpect(t, "floor(3.1)", "3")
	evalExpect(t, "floor(3.9)", "3")
	evalExpect(t, "floor(-3.1)", "-4")
	evalExpect(t, "floor(-3.9)", "-4")
	evalExpect(t, "round(3.4)", "3")
	evalExpect(t, "round(3.5)", "4")
	evalExpect(t, "round(-3.4)", "-3")
	evalExpect(t, "round(-3.5)", "-4")
}

// TestConstants tests pi and e constants.
func TestConstants(t *testing.T) {
	evalApprox(t, "pi", math.Pi, 0.0001)
	evalApprox(t, "e", math.E, 0.0001)
	evalApprox(t, "pi * 2", 2*math.Pi, 0.0001)
	evalApprox(t, "e * 2", 2*math.E, 0.0001)
	evalApprox(t, "sin(pi/2)", 1, 0.0001)
	evalApprox(t, "ln(e)", 1, 0.0001)
}

// TestComplexExpressions tests more complex, real-world expressions.
func TestComplexExpressions(t *testing.T) {
	evalApprox(t, "45 * (1 + 0.05) ^ 10", 45*math.Pow(1.05, 10), 0.0001)
	evalApprox(t, "sin(pi/4) + cos(pi/4)", math.Sin(math.Pi/4)+math.Cos(math.Pi/4), 0.0001)
	evalApprox(t, "sqrt(3^2 + 4^2)", 5, 0.0001)
	evalApprox(t, "abs(-3) + round(3.7) + ceil(2.1)", 3+4+3, 0.0001)
	evalApprox(t, "log(1000) + ln(e) + sqrt(16)", 3+1+4, 0.0001)
	evalApprox(t, "tan(pi/4) * 2 + 1", 3, 0.0001)
	evalApprox(t, "sin(pi/6) * 2", 1, 0.0001)
	evalApprox(t, "(2 + 3) * (4 + 5)", 45, 0.0001)
	evalApprox(t, "10 / (1 + 4)", 2, 0.0001)
}

// TestChainedFunctionCalls tests combinations of nested function calls.
func TestChainedFunctionCalls(t *testing.T) {
	evalApprox(t, "ceil(sqrt(10))", 4, 0.0001)
	evalApprox(t, "round(sqrt(2) ^ 2)", 2, 0.0001)
	evalApprox(t, "abs(sin(-pi/2))", 1, 0.0001)
	evalApprox(t, "sqrt(abs(-16))", 4, 0.0001)
}

// TestWhitespaceTolerance tests that the parser handles various whitespace.
func TestWhitespaceTolerance(t *testing.T) {
	evalExpect(t, "1+2", "3")
	evalExpect(t, "   1   +   2   ", "3")
	evalExpect(t, "3+4*2", "11")
	evalExpect(t, "(3+4)*2", "14")
	evalExpect(t, "sin(0)", "0")
	evalExpect(t, "  sin  (  pi  /  2  )  ", "1")
}

// TestIntegerResult tests that the calculator returns integer-formatted strings
// when the result is a whole number within range.
func TestIntegerResult(t *testing.T) {
	tests := []struct {
		expr     string
		expected string
	}{
		// Results that are exact integers should not have decimal point
		{"2 + 2", "4"},
		{"10 / 2", "5"},
		{"100 / 4", "25"},
		{"3 * 7", "21"},
		{"sqrt(144)", "12"},
		{"cos(0)", "1"},
		{"abs(-3)", "3"},
		{"floor(5.0)", "5"},
		// Floating results
		{"1 / 3", "0.3333333333333333"},
		{"10 / 3", "3.3333333333333335"},
	}
	for _, tc := range tests {
		evalExpect(t, tc.expr, tc.expected)
	}
}

// TestErrorCases tests that the parser properly reports errors.
func TestErrorCases(t *testing.T) {
	errors := []string{
		"1 / 0",
		"5 % 0",
		"sqrt(-1)",
		"log(-1)",
		"ln(0)",
		"asin(2)",
		"acos(2)",
		"asin(-2)",
		"(",
		")",
		"1 2",
		"unknown_func(3)",
		"unknown_const",
		"sin()",
		"sin(1, 2)",
		"log(1, 2)",
		"1 + +", // incomplete expression
	}
	for _, expr := range errors {
		evalError(t, expr)
	}
}

// TestEmptyAndInvalidSyntax tests edge cases with empty or malformed input.
func TestEmptyAndInvalidSyntax(t *testing.T) {
	evalError(t, "")
	evalError(t, "   ")
	evalError(t, "+")
	evalError(t, "-")
	evalError(t, "*")
	evalError(t, "1 + * 2") // missing operand
	evalError(t, "1 (2)")   // no operator between numbers
	evalError(t, "1 + (2")  // unmatched opening paren
	evalError(t, "1 + 2)")  // unmatched closing paren
	evalError(t, "1 + @ 2") // invalid character
	evalError(t, "1 + 2 +") // trailing operator
}

// TestTrailingContent tests that trailing content after a valid expression is detected.
func TestTrailingContent(t *testing.T) {
	// These should work: the expression part is valid
	evalExpect(t, "1 + 2", "3")
	evalExpect(t, " 1 + 2 ", "3")

	// These should fail: there's content after the expression
	evalError(t, "1 + 2 abc")
	evalError(t, "3 + 4 extra")
}

// TestModuloPrecision tests modulo with various inputs.
func TestModuloPrecision(t *testing.T) {
	evalExpect(t, "5 % 2", "1")
	evalExpect(t, "100 % 3", "1")
	evalExpect(t, "10 % 10", "0")
	evalExpect(t, "7 % 100", "7")
	evalExpect(t, "0 % 5", "0")
	evalExpect(t, "999 % 1", "0")
}

// TestNegativeResults tests expressions that produce negative results.
func TestNegativeResults(t *testing.T) {
	evalExpect(t, "1 - 5", "-4")
	evalExpect(t, "3 - 10", "-7")
	evalExpect(t, "-2 * 3", "-6")
	evalExpect(t, "2 * -3", "-6")
	evalExpect(t, "-10 / 2", "-5")
	evalExpect(t, "10 / -2", "-5")
	evalExpect(t, "-(3 + 4)", "-7")
	evalExpect(t, "-2 ^ 2", "-4") // unary minus before exponentiation: -(2^2)
}
