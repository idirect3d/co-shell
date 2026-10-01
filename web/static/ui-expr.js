// co-shell Web UI - safe math expression parser for the figure component
// (FEATURE-570).
//
// Hand-written lexer + recursive-descent parser. Dynamic code execution is
// deliberately avoided: the source text becomes a small AST that the evaluator
// walks, so only whitelisted functions and constants are reachable and an
// expression such as "window.constructor(...)" has no path to a host object.
// Identifiers are either a whitelisted constant, a whitelisted function being
// called, or a variable that the caller must inject by name.
//
// Grammar (power is right associative and binds tighter than unary minus,
// so "-x^2" means -(x^2) exactly as in maths):
//   expr    := term (('+' | '-') term)*
//   term    := factor (('*' | '/' | '%') factor)*
//   factor  := unary
//   unary   := ('-' | '+') unary | power
//   power   := primary ('^' unary)?
//   primary := number | ident | ident '(' args? ')' | '(' expr ')'
//   args    := expr (',' expr)*
//
// Public API (window.UIExpr):
//   compile(expr, allowedVars)  -> {ok:true, node, vars} | {ok:false, code, ...}
//   evaluate(expr, vars)        -> number (NaN when the expression is invalid)
//   evalNode(node, vars)        -> number
//   sample(expr, name, from, to, count) -> {ok:true, points:[[x,y],...]}
//   errorText(res)              -> one-line localized message for a failure
//
// Author: L.Shuang
// Created: 2026-10-01
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  var MAX_LEN = 500; // characters accepted for a single expression
  var MAX_DEPTH = 32; // recursion guard shared by parser and evaluator
  var MAX_SAMPLES = 400; // sample points allowed for one function curve

  // name -> [arity, implementation]; arity -1 means "one or more arguments".
  // "log" follows the math convention (natural log); log10 is explicit.
  var FUNCS = {
    sin: [1, Math.sin], cos: [1, Math.cos], tan: [1, Math.tan],
    asin: [1, Math.asin], acos: [1, Math.acos], atan: [1, Math.atan],
    sinh: [1, Math.sinh], cosh: [1, Math.cosh], tanh: [1, Math.tanh],
    sqrt: [1, Math.sqrt], abs: [1, Math.abs], exp: [1, Math.exp],
    log: [1, Math.log], ln: [1, Math.log],
    log10: [1, function (v) { return Math.log(v) / Math.LN10; }],
    floor: [1, Math.floor], ceil: [1, Math.ceil], round: [1, Math.round],
    sign: [1, Math.sign],
    min: [-1, Math.min], max: [-1, Math.max],
    pow: [2, Math.pow], atan2: [2, Math.atan2]
  };

  var CONSTS = { pi: Math.PI, e: Math.E };

  var NAME_RE = /^[A-Za-z_][A-Za-z0-9_]*$/;
  var DIGIT_RE = /[0-9]/;

  // fail builds the structured failure object every public entry point returns,
  // so a caller can branch on "ok" without catching exceptions.
  function fail(code, extra) {
    var res = { ok: false, code: code, message: message(code) };
    if (extra) {
      for (var k in extra) {
        if (Object.prototype.hasOwnProperty.call(extra, k)) res[k] = extra[k];
      }
    }
    return res;
  }

  // message gives the internal English wording of an error code; the UI layer
  // turns it into zh/en text through errorText().
  function message(code) {
    switch (code) {
      case "empty": return "expression is empty";
      case "too_long": return "expression is too long";
      case "bad_char": return "unexpected character";
      case "bad_number": return "invalid number literal";
      case "syntax": return "syntax error";
      case "unbalanced": return "unbalanced parenthesis";
      case "unknown_ident": return "unknown variable";
      case "unknown_func": return "unknown function";
      case "func_no_call": return "function used without arguments";
      case "arity": return "wrong number of arguments";
      case "depth": return "expression is nested too deeply";
      case "samples": return "too many sample points";
      default: return "invalid expression";
    }
  }

  // errorText renders a failure as a single localized line; callers use it for
  // the in-figure degraded notice (the figure keeps drawing everything else).
  function errorText(res) {
    var code = res && res.code ? res.code : "invalid";
    var zh = {
      empty: "表达式为空",
      too_long: "表达式超过 500 字符",
      bad_char: "表达式含非法字符",
      bad_number: "数字格式错误",
      syntax: "表达式语法错误",
      unbalanced: "括号不匹配",
      unknown_ident: "未知变量：" + (res && res.name ? res.name : ""),
      unknown_func: "未知函数：" + (res && res.name ? res.name : ""),
      func_no_call: "函数缺少参数：" + (res && res.name ? res.name : ""),
      arity: "函数参数个数不正确",
      depth: "表达式嵌套过深",
      samples: "采样点数超过上限",
      invalid: "表达式无效"
    };
    var en = {
      empty: "expression is empty",
      too_long: "expression exceeds 500 characters",
      bad_char: "expression contains an invalid character",
      bad_number: "invalid number literal",
      syntax: "expression has a syntax error",
      unbalanced: "unbalanced parenthesis",
      unknown_ident: "unknown variable: " + (res && res.name ? res.name : ""),
      unknown_func: "unknown function: " + (res && res.name ? res.name : ""),
      func_no_call: "function needs arguments: " + (res && res.name ? res.name : ""),
      arity: "wrong number of function arguments",
      depth: "expression is nested too deeply",
      samples: "too many sample points",
      invalid: "invalid expression"
    };
    var zhText = zh[code] || zh.invalid;
    var enText = en[code] || en.invalid;
    if (global.UI && typeof global.UI.t === "function") return global.UI.t(zhText, enText);
    return zhText;
  }

  // lex turns the source into a flat token list. Numbers are scanned by hand
  // (digits, one dot, optional exponent) so "1e3" and ".5" work while strings
  // such as "0x10" or "1__0" are rejected.
  function lex(src) {
    var toks = [];
    var i = 0;
    var n = src.length;
    while (i < n) {
      var c = src.charAt(i);
      if (c === " " || c === "\t" || c === "\n" || c === "\r") { i++; continue; }
      if (DIGIT_RE.test(c) || (c === "." && DIGIT_RE.test(src.charAt(i + 1)))) {
        var j = i;
        while (j < n && (DIGIT_RE.test(src.charAt(j)) || src.charAt(j) === ".")) j++;
        if (src.charAt(j) === "e" || src.charAt(j) === "E") {
          var k = j + 1;
          if (src.charAt(k) === "+" || src.charAt(k) === "-") k++;
          if (DIGIT_RE.test(src.charAt(k))) {
            j = k;
            while (j < n && DIGIT_RE.test(src.charAt(j))) j++;
          }
        }
        var numText = src.slice(i, j);
        var v = parseFloat(numText);
        if (!isFinite(v) || numText.split(".").length > 2) return fail("bad_number", { pos: i });
        toks.push({ k: "num", v: v });
        i = j;
        continue;
      }
      if (NAME_RE.test(c)) {
        var e = i;
        while (e < n && /[A-Za-z0-9_]/.test(src.charAt(e))) e++;
        toks.push({ k: "id", v: src.slice(i, e) });
        i = e;
        continue;
      }
      if ("+-*/%^(),".indexOf(c) >= 0) { toks.push({ k: c }); i++; continue; }
      return fail("bad_char", { pos: i });
    }
    return { ok: true, toks: toks };
  }

  // Parser walks the token list with one token of lookahead. Errors are thrown
  // as {code,pos} and converted to a normal failure object by compile().
  function Parser(toks) {
    this.toks = toks;
    this.pos = 0;
  }

  Parser.prototype.peek = function () {
    return this.pos < this.toks.length ? this.toks[this.pos] : { k: "eof" };
  };

  Parser.prototype.take = function () {
    var tok = this.peek();
    if (tok.k !== "eof") this.pos++;
    return tok;
  };

  Parser.prototype.expect = function (kind) {
    var tok = this.take();
    if (tok.k !== kind) throw { code: kind === ")" ? "unbalanced" : "syntax", pos: this.pos };
    return tok;
  };

  Parser.prototype.at = function (kind) {
    return this.peek().k === kind;
  };

  // expr := term (('+' | '-') term)*
  Parser.prototype.expr = function (depth) {
    var node = this.term(depth);
    while (this.at("+") || this.at("-")) {
      var op = this.take().k;
      node = { t: "bin", op: op, l: node, r: this.term(depth) };
    }
    return node;
  };

  // term := factor (('*' | '/' | '%') factor)*
  Parser.prototype.term = function (depth) {
    var node = this.factor(depth);
    while (this.at("*") || this.at("/") || this.at("%")) {
      var op = this.take().k;
      node = { t: "bin", op: op, l: node, r: this.factor(depth) };
    }
    return node;
  };

  // factor := unary
  Parser.prototype.factor = function (depth) {
    return this.unary(depth);
  };

  // unary := ('-' | '+') unary | power
  Parser.prototype.unary = function (depth) {
    if (depth > MAX_DEPTH) throw { code: "depth", pos: this.pos };
    if (this.at("-") || this.at("+")) {
      var op = this.take().k;
      return { t: "un", op: op, a: this.unary(depth + 1) };
    }
    return this.power(depth);
  };

  // power := primary ('^' unary)?  -- right associative
  Parser.prototype.power = function (depth) {
    var base = this.primary(depth);
    if (this.at("^")) {
      this.take();
      return { t: "bin", op: "^", l: base, r: this.unary(depth + 1) };
    }
    return base;
  };

  // primary := number | ident | ident '(' args? ')' | '(' expr ')'
  Parser.prototype.primary = function (depth) {
    if (depth > MAX_DEPTH) throw { code: "depth", pos: this.pos };
    var tok = this.peek();
    if (tok.k === "num") {
      this.take();
      return { t: "num", v: tok.v };
    }
    if (tok.k === "id") {
      this.take();
      if (Object.prototype.hasOwnProperty.call(CONSTS, tok.v)) {
        if (this.at("(")) throw { code: "syntax", pos: this.pos };
        return { t: "num", v: CONSTS[tok.v] };
      }
      if (this.at("(")) {
        if (!Object.prototype.hasOwnProperty.call(FUNCS, tok.v)) {
          throw { code: "unknown_func", name: tok.v, pos: this.pos };
        }
        this.take();
        var args = [];
        if (!this.at(")")) {
          args.push(this.expr(depth + 1));
          while (this.at(",")) {
            this.take();
            args.push(this.expr(depth + 1));
          }
        }
        this.expect(")");
        return { t: "call", f: tok.v, args: args };
      }
      if (Object.prototype.hasOwnProperty.call(FUNCS, tok.v)) {
        throw { code: "func_no_call", name: tok.v, pos: this.pos };
      }
      return { t: "var", n: tok.v };
    }
    if (tok.k === "(") {
      this.take();
      var inner = this.expr(depth + 1);
      this.expect(")");
      return inner;
    }
    throw { code: "syntax", pos: this.pos };
  };

  // walk collects the free variable names used by the AST, so compile() can
  // reject names the caller does not inject.
  function walkVars(node, out) {
    if (!node) return;
    if (node.t === "var") {
      if (out.indexOf(node.n) < 0) out.push(node.n);
    } else if (node.t === "un") {
      walkVars(node.a, out);
    } else if (node.t === "bin") {
      walkVars(node.l, out);
      walkVars(node.r, out);
    } else if (node.t === "call") {
      for (var i = 0; i < node.args.length; i++) walkVars(node.args[i], out);
    }
  }

  // arityError checks the argument count of a whitelisted call node.
  function arityError(node) {
    var spec = FUNCS[node.f];
    if (!spec) return true;
    if (spec[0] === -1) return node.args.length < 1;
    return node.args.length !== spec[0];
  }

  // checkCalls validates arity across the whole AST (compile-time, so a bad
  // call is reported once instead of on every sampled point).
  function checkCalls(node) {
    if (!node) return true;
    if (node.t === "call") {
      if (arityError(node) || !checkCalls(node.args)) return false;
    }
    if (node.t === "un") return checkCalls(node.a);
    if (node.t === "bin") return checkCalls(node.l) && checkCalls(node.r);
    return true;
  }

  // compile parses expr and returns either {ok:true, node, vars} or a failure
  // object. allowedVars is an optional array of variable names the caller will
  // inject; any other identifier is rejected at compile time.
  function compile(expr, allowedVars) {
    if (typeof expr !== "string") return fail("syntax", { raw: expr });
    if (expr.length > MAX_LEN) return fail("too_long");
    if (expr.replace(/[\s]/g, "") === "") return fail("empty");
    var lexed = lex(expr);
    if (!lexed.ok) return lexed;
    var parser = new Parser(lexed.toks);
    var node;
    try {
      node = parser.expr(0);
    } catch (e) {
      return fail(e && e.code ? e.code : "syntax", e && e.name ? { name: e.name } : null);
    }
    if (parser.peek().k !== "eof") {
      return fail(parser.at(")") ? "unbalanced" : "syntax");
    }
    if (!checkCalls(node)) return fail("arity");
    var used = [];
    walkVars(node, used);
    if (allowedVars) {
      for (var i = 0; i < used.length; i++) {
        if (allowedVars.indexOf(used[i]) < 0) return fail("unknown_ident", { name: used[i] });
      }
    }
    return { ok: true, node: node, vars: used };
  }

  // evalNode walks the AST. Every read from vars goes through hasOwnProperty so
  // an identifier such as "constructor" can never reach Object.prototype.
  function evalNode(node, vars, depth) {
    if (!node || depth > MAX_DEPTH) return NaN;
    switch (node.t) {
      case "num":
        return node.v;
      case "var": {
        if (!vars || !Object.prototype.hasOwnProperty.call(vars, node.n)) return NaN;
        var v = vars[node.n];
        return typeof v === "number" ? v : NaN;
      }
      case "un": {
        var a = evalNode(node.a, vars, depth + 1);
        return node.op === "-" ? -a : a;
      }
      case "bin": {
        var l = evalNode(node.l, vars, depth + 1);
        var r = evalNode(node.r, vars, depth + 1);
        if (node.op === "+") return l + r;
        if (node.op === "-") return l - r;
        if (node.op === "*") return l * r;
        if (node.op === "/") return l / r;
        if (node.op === "%") return l % r;
        if (node.op === "^") return Math.pow(l, r);
        return NaN;
      }
      case "call": {
        var spec = FUNCS[node.f];
        if (!spec) return NaN;
        var args = [];
        for (var i = 0; i < node.args.length; i++) {
          args.push(evalNode(node.args[i], vars, depth + 1));
        }
        return spec[1].apply(null, args);
      }
      default:
        return NaN;
    }
  }

  // evaluate compiles and evaluates in one step; an invalid expression yields
  // NaN so the caller can simply skip the point.
  function evaluate(expr, vars) {
    var res = compile(expr, vars ? Object.keys(vars) : null);
    if (!res.ok) return NaN;
    return evalNode(res.node, vars || {}, 0);
  }

  // sampleCurve walks [from,to] in count steps and returns [[x,y],...] with all
  // non-finite results dropped (a pole such as tan(pi/2) leaves a gap rather
  // than breaking the polyline).
  function sampleCurve(expr, name, from, to, count) {
    var n = Math.floor(typeof count === "number" ? count : 0);
    if (!isFinite(n) || n < 2) n = 100;
    if (n > MAX_SAMPLES) return fail("samples");
    var a = typeof from === "number" ? from : 0;
    var b = typeof to === "number" ? to : 1;
    if (!isFinite(a) || !isFinite(b)) return fail("bad_number");
    var res = compile(expr, [name]);
    if (!res.ok) return res;
    var pts = [];
    var vars = {};
    var step = (b - a) / (n - 1);
    for (var i = 0; i < n; i++) {
      vars[name] = a + step * i;
      var y = evalNode(res.node, vars, 0);
      if (isFinite(y)) pts.push([vars[name], y]);
    }
    return { ok: true, points: pts };
  }

  global.UIExpr = {
    MAX_LENGTH: MAX_LEN,
    MAX_DEPTH: MAX_DEPTH,
    MAX_SAMPLES: MAX_SAMPLES,
    FUNCTIONS: Object.keys(FUNCS),
    CONSTANTS: Object.keys(CONSTS),
    compile: compile,
    evaluate: evaluate,
    evalNode: evalNode,
    sample: sampleCurve,
    errorText: errorText
  };
})(typeof window !== "undefined" ? window : this);
