package node
import (
	"fmt"
	"go/token"
	"go/scanner"
	"go/ast"
	"io/ioutil"
	"errors"
	"io"
	"bytes"
)

// The parser structure holds the parser's internal state.
type parser struct {
	file    *token.File
	errors  scanner.ErrorList
	scanner scanner.Scanner

								  // Tracing/debugging
	trace  bool // == (mode & Trace != 0)
	indent int  // indentation used for tracing output

								  // Comments
	comments    []*ast.CommentGroup
	leadComment *ast.CommentGroup // last lead comment
	lineComment *ast.CommentGroup // last line comment

								  // Next token
	pos token.Pos   // token position
	tok token.Token // one token look-ahead
	lit string      // token literal

								  // Error recovery
								  // (used to limit the number of calls to syncXXX functions
								  // w/o making scanning progress - avoids potential endless
								  // loops across multiple parser functions during error recovery)
	syncPos token.Pos // last synchronization position
	syncCnt int       // number of calls to syncXXX without progress

								  // Non-syntactic parser control
	exprLev int  // < 0: in control clause, >= 0: in expression
	inRhs   bool // if set, the parser is parsing a rhs expression

								  // Ordinary identifier scopes
	pkgScope   *ast.Scope        // pkgScope.Outer == nil
	topScope   *ast.Scope        // top-most scope; may be pkgScope
	unresolved []*ast.Ident      // unresolved identifiers
	imports    []*ast.ImportSpec // list of imports

								  // Label scopes
								  // (maintained by open/close LabelScope)
	labelScope  *ast.Scope     // label scope for current function
	targetStack [][]*ast.Ident // stack of unresolved labels
}


// parseOperand may return an expression or a raw type (incl. array
// types of the form [...]T. Callers must verify the result.
// If lhs is set and the result is an identifier, it is not resolved.
//
func (p *parser) parseOperand(lhs bool) ast.Expr {

	switch p.tok {
	case token.IDENT:
		x := p.parseIdent()
		//		if !lhs {
		//			p.resolve(x)
		//		}
		return x

	case token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING:
		x := &ast.BasicLit{ValuePos: p.pos, Kind: p.tok, Value: p.lit}
		p.next()
		return x

	case token.LPAREN:
		lparen := p.pos
		p.next()
		p.exprLev++
		x := p.parseOperand(false)
		p.exprLev--
		rparen := p.expect(token.RPAREN)
		return &ast.ParenExpr{Lparen: lparen, X: x, Rparen: rparen}

	default:
		p.errorExpected(p.pos, "operand")
		return &ast.BadExpr{From: p.pos - 10, To: p.pos + 10}
	}

}


// Advance to the next non-comment token. In the process, collect
// any comment groups encountered, and remember the last lead and
// and line comments.
//
// A lead comment is a comment group that starts and ends in a
// line without any other tokens and that is followed by a non-comment
// token on the line immediately after the comment group.
//
// A line comment is a comment group that follows a non-comment
// token on the same line, and that has no tokens after it on the line
// where it ends.
//
// Lead and line comments may be considered documentation that is
// stored in the AST.
//
func (p *parser) next() {
	p.leadComment = nil
	p.lineComment = nil
	p.pos, p.tok, p.lit = p.scanner.Scan()
}

func (p *parser) printTrace(a ...interface{}) {
	const dots = ". . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . "
	const n = len(dots)
	pos := p.file.Position(p.pos)
	fmt.Printf("%5d:%3d: ", pos.Line, pos.Column)
	i := 2 * p.indent
	for i > n {
		fmt.Print(dots)
		i -= n
	}
	// i <= n
	fmt.Print(dots[0:i])
	fmt.Println(a...)
}


func trace(p *parser, msg string) *parser {
	p.printTrace(msg, "(")
	p.indent++
	return p
}

// Usage pattern: defer un(trace(p, "..."))
func un(p *parser) {
	p.indent--
	p.printTrace(")")
}




func (p *parser) init(fset *token.FileSet, filename string, src []byte) {
	p.file = fset.AddFile(filename, -1, len(src))
	var m scanner.Mode
	eh := func(pos token.Position, msg string) { p.errors.Add(pos, msg) }
	p.scanner.Init(p.file, src, eh, m)

	p.next()
}

// A bailout panic is raised to indicate early termination.
type bailout struct{}

func (p *parser) error(pos token.Pos, msg string) {
	epos := p.file.Position(pos)

	p.errors.Add(epos, msg)
}

func (p *parser) errorExpected(pos token.Pos, msg string) {
	msg = "expected " + msg
	if pos == p.pos {
		// the error happened at the current position;
		// make the error message more specific
		if p.tok == token.SEMICOLON && p.lit == "\n" {
			msg += ", found newline"
		} else {
			msg += ", found '" + p.tok.String() + "'"
			if p.tok.IsLiteral() {
				msg += " " + p.lit
			}
		}
	}
	p.error(pos, msg)
}


func (p *parser) expect(tok token.Token) token.Pos {
	pos := p.pos
	if p.tok != tok {
		p.errorExpected(pos, "'"+tok.String()+"'")
	}
	p.next() // make progress
	return pos
}

func assert(cond bool, msg string) {
	if !cond {
		panic("go/parser internal error: " + msg)
	}
}


// The unresolved object is a sentinel to mark identifiers that have been added
// to the list of unresolved identifiers. The sentinel is only used for verifying
// internal consistency.
var unresolved = new(ast.Object)

// If x is an identifier, tryResolve attempts to resolve x by looking up
// the object it denotes. If no object is found and collectUnresolved is
// set, x is marked as unresolved and collected in the list of unresolved
// identifiers.
//
func (p *parser) tryResolve(x ast.Expr, collectUnresolved bool) {
	// nothing to do if x is not an identifier or the blank identifier
	ident, _ := x.(*ast.Ident)
	if ident == nil {
		return
	}
	assert(ident.Obj == nil, "identifier already declared or resolved")
	if ident.Name == "_" {
		return
	}
	// try to resolve the identifier
	for s := p.topScope; s != nil; s = s.Outer {
		if obj := s.Lookup(ident.Name); obj != nil {
			ident.Obj = obj
			return
		}
	}
	// all local scopes are known, so any unresolved identifier
	// must be found either in the file scope, package scope
	// (perhaps in another file), or universe scope --- collect
	// them so that they can be resolved later
	if collectUnresolved {
		ident.Obj = unresolved
		p.unresolved = append(p.unresolved, ident)
	}
}

func (p *parser) resolve(x ast.Expr) {
	p.tryResolve(x, true)
}


// ----------------------------------------------------------------------------
// Identifiers

func (p *parser) parseIdent() *ast.Ident {
	pos := p.pos
	name := "_"
	if p.tok == token.IDENT {
		name = p.lit
		p.next()
	} else {
		p.expect(token.IDENT) // use expect() error handling
	}
	return &ast.Ident{NamePos: pos, Name: name}
}

// If src != nil, readSource converts src to a []byte if possible;
// otherwise it returns an error. If src == nil, readSource returns
// the result of reading the file specified by filename.
//
func readSource(filename string, src interface{}) ([]byte, error) {
	if src != nil {
		switch s := src.(type) {
			case string:
			return []byte(s), nil
			case []byte:
			return s, nil
			case *bytes.Buffer:
			// is io.Reader, but src is already available in []byte form
			if s != nil {
				return s.Bytes(), nil
			}
			case io.Reader:
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, s); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
		return nil, errors.New("invalid source")
	}
	return ioutil.ReadFile(filename)
}


// ParseFile parses the source code of a single Go source file and returns
// the corresponding ast.File node. The source code may be provided via
// the filename of the source file, or via the src parameter.
//
// If src != nil, ParseFile parses the source from src and the filename is
// only used when recording position information. The type of the argument
// for the src parameter must be string, []byte, or io.Reader.
// If src == nil, ParseFile parses the file specified by filename.
//
// The mode parameter controls the amount of source text parsed and other
// optional parser functionality. Position information is recorded in the
// file set fset.
//
// If the source couldn't be read, the returned AST is nil and the error
// indicates the specific failure. If the source was read but syntax
// errors were found, the result is a partial AST (with ast.Bad* nodes
// representing the fragments of erroneous source code). Multiple errors
// are returned via a scanner.ErrorList which is sorted by file position.
//
func ParseFile(fset *token.FileSet, filename string, src interface{}) *ast.File {
	// get source
	text, err := readSource(filename, src)
	if err != nil {
		return nil
	}

	var p parser

	// parse source
	p.init(fset, filename, text)

	// Don't bother parsing the rest if we had errors scanning the first token.
	// Likely not a Go source file at all.
	if p.errors.Len() != 0 {
		return nil
	}
	//	xxx := p.parseOperand(false)
	return &ast.File{
		Name: &ast.Ident{Name: "p"},
		Decls: []ast.Decl{
			&ast.FuncDecl{
				Name: &ast.Ident{Name: "f"},
				Type: &ast.FuncType{},
			},
		},
	}
}
