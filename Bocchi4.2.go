package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"image/png"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"github.com/go-vgo/robotgo"
	"github.com/kbinani/screenshot"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type TokenType int

const (
	EOF TokenType = iota
	INT
	FLOAT
	PLUS
	MINUS
	MUL
	DIV
	LPAREN
	RPAREN
	ASSIGN
	IDENT
	SEMI
	PUTS
	LOOP
	LBRACE
	RBRACE
	LT
	GT
	LEQ
	GEQ
	EQ
	NEQ
	TYPE
	INT_TYPE
	FLOAT_TYPE
	BOOL_TYPE
	STRING_TYPE
	TRUE
	FALSE
	STRING
	IF
	ELSE
	LIST_TYPE
	LBRACKET
	RBRACKET
	COMMA
	LEN
	DOT
	DEF
	RETURN
	TIME
	NOW
	SLEEP
	FORMAT
	SPLIT
	FILE
	READ
	READ_LINES
	WRITE
	MKDIR
	TRY
	EXPECT
	BREAK
	DICT_TYPE
	COLON
	POW
	SIN
	COS
	TAN
	SQRT
	LOG
	LOG10
	FOR
	IN
	CHANGE
	INPUT
	SET
	QR_SCAN
	IMAGE
	SHOW
	ADD_TEXT
	GO
	AT
	STR_CONVERT
	ENCRYPT
	DECRYPT
	RAND
	DIR
	DELETE
	DEFER
	ADD_METHOD
	REMOVE_METHOD
	REMOVE_AT_METHOD
	OK
	OS
	CMD
	POPUP
	LAMBDA
	ROCK
	MOD
	EMBEDDED_STRING
	ARROW
	REPLACE_METHOD
	STATIC
	USE
	DOUBLE_COLON
	AS
	FLOOR
	CEIL
	ROUND
	PUTSF
	PUTSR
	AND
	OR
	RANGE
	LIST_COMP
	ADMIN_CMD
	ELLIPSIS
	FOR_IN_COUNT
	KEYBOARD
	MOUSE
	PRESS
	RELEASE
	MOVE
	CLICK
	SCROLL
	LEFT
	RIGHT
	MIDDLE
	GET_POS
	WAIT_ALL
	IMAGE_FIND
	IMAGE_CLICK
	IMAGE_WAIT
	IMAGE_WAIT_DISAPPEAR
	IMAGE_EXISTS
	IMAGE_MATCH
	SCREENSHOT
	REGION
	CONFIDENCE
	CLEAR
	TEST
	ASSERT
	RUN_TESTS
	TITLE
	KEY_RECORD
	KEY_STOP_RECORD
	OBJECT_TYPE
	LET
	DOLLAR
	THIS
	EXTENDS
	SUPER
	EXIT
	APPEND
	MEMORY
	// ===== 服务器功能 =====
	SERVER
	ROUTE_GET
	ROUTE_POST
	ROUTE_PUT
	LISTEN
	SERVER_STOP
	SERVER_WAIT
)

// 在 ASTNode 接口实现中添加
type ListGenerator struct {
	Start ASTNode
	End   ASTNode
	Step  ASTNode // 可选
}

func (lg ListGenerator) Eval(ctx *ExecutionContext) interface{} {
	startVal := lg.Start.Eval(ctx)
	endVal := lg.End.Eval(ctx)

	start, ok1 := startVal.(float64)
	end, ok2 := endVal.(float64)

	if !ok1 || !ok2 {
		panic("range参数必须是数字")
	}

	var step float64 = 1
	if lg.Step != nil {
		stepVal := lg.Step.Eval(ctx)
		if s, ok := stepVal.(float64); ok {
			step = s
		}
	}

	var result []interface{}
	if step > 0 {
		for i := start; i < end; i += step {
			result = append(result, i)
		}
	} else {
		for i := start; i > end; i += step {
			result = append(result, i)
		}
	}
	return result
}

// 嵌入标记和长度常量
const (
	EmbedMarker = "CCHI_EMB"
	MarkerSize  = 8
	LengthSize  = 8
	HeaderSize  = MarkerSize + LengthSize
)

type Token struct {
	Type  TokenType
	Value string
}

type Lexer struct {
	text   []rune
	pos    int
	line   int
	column int
}

func NewLexer(text string) *Lexer {
	return &Lexer{
		text:   []rune(text),
		pos:    0,
		line:   1,
		column: 1,
	}
}

func (l *Lexer) skipComment() {
	for l.pos < len(l.text) && l.text[l.pos] != '\n' {
		l.pos++
	}
	l.column = 1
}

type EmbeddedString struct {
	Parts []interface{} // string或Variable
}

func (e EmbeddedString) Eval(ctx *ExecutionContext) interface{} {
	var buf strings.Builder
	for _, part := range e.Parts {
		switch v := part.(type) {
		case string:
			buf.WriteString(v)
		case Variable:
			val := v.Eval(ctx)
			buf.WriteString(fmt.Sprintf("%v", val))
		}
	}
	return buf.String()
}

func (l *Lexer) readString() Token {
	var buf strings.Builder
	l.pos++ // 跳过开始的引号
	l.column++

	inEmbed := false
	escapeNext := false

	for l.pos < len(l.text) {
		ch := l.text[l.pos]

		if escapeNext {
			escapeNext = false
			switch ch {
			case 'n':
				buf.WriteRune('\n')
			case 't':
				buf.WriteRune('\t')
			case '"':
				buf.WriteRune('"')
			case '\\':
				buf.WriteRune('\\')
			case '{':
				buf.WriteRune('{')
			case '}':
				buf.WriteRune('}')
			default:
				buf.WriteRune('\\')
				buf.WriteRune(ch)
			}
			l.pos++
			l.column++
			continue
		}

		if ch == '\\' {
			escapeNext = true
			l.pos++
			l.column++
			continue
		}

		if ch == '{' && !inEmbed {
			inEmbed = true
			l.pos++
			l.column++
			// 添加变量开始标记
			buf.WriteByte(0)
			continue
		}

		if ch == '}' && inEmbed {
			inEmbed = false
			l.pos++
			l.column++
			// 添加变量结束标记
			buf.WriteByte(0)
			continue
		}

		if !inEmbed && ch == '"' {
			break
		}

		// 写入字符
		buf.WriteRune(ch)
		l.pos++
		l.column++
	}

	if inEmbed {
		panic(l.errorMessage("Unclosed embedded variable"))
	}

	if l.pos >= len(l.text) {
		panic(l.errorMessage("Unclosed string literal"))
	}

	l.pos++ // 跳过结束引号
	l.column++

	return Token{Type: STRING, Value: buf.String()}
}

func (l *Lexer) NextToken() Token {
	for l.pos < len(l.text) {
		ch := l.text[l.pos]
		if l.pos+6 < len(l.text) && string(l.text[l.pos:l.pos+7]) == "for..in" {
			l.pos += 7
			l.column += 7
			return Token{FOR_IN_COUNT, "for..in"}
		}

		switch {
		case ch == '?':
			l.pos++
			l.column++
			return Token{EXTENDS, "?"}
		case ch == '$': // 新增：对象成员访问符
			l.pos++
			l.column++
			return Token{DOLLAR, "$"}
		case unicode.IsLetter(ch):
			return l.readIdentifier()
		case ch == '\n':
			l.pos++
			l.line++
			l.column = 1
			continue
		case ch == '%':
			l.pos++
			l.column++
			return Token{MOD, "%"}
		case unicode.IsSpace(ch):
			l.pos++
			l.column++
			continue
		case ch == '/':
			if l.pos+1 < len(l.text) && l.text[l.pos+1] == '/' {
				l.skipComment()
				continue
			}
			l.pos++
			l.column++
			return Token{DIV, "/"}
		case ch == '+':
			l.pos++
			l.column++
			return Token{PLUS, "+"}
		case ch == '-':
			if l.pos+1 < len(l.text) && l.text[l.pos+1] == '>' {
				l.pos += 2
				l.column += 2
				return Token{ARROW, "->"}
			} else {
				l.pos++
				l.column++
				return Token{MINUS, "-"}
			}
		case ch == '*':
			l.pos++
			l.column++
			return Token{MUL, "*"}
		case ch == '^':
			l.pos++
			l.column++
			return Token{POW, "^"}
		case ch == '(':
			l.pos++
			l.column++
			return Token{LPAREN, "("}
		case ch == ')':
			l.pos++
			l.column++
			return Token{RPAREN, ")"}
		case ch == '=':
			if l.pos+1 < len(l.text) && l.text[l.pos+1] == '=' {
				l.pos += 2
				l.column += 2
				return Token{EQ, "=="}
			} else {
				l.pos++
				l.column++
				return Token{ASSIGN, "="}
			}
		case ch == '!':
			if l.pos+1 < len(l.text) && l.text[l.pos+1] == '=' {
				l.pos += 2
				l.column += 2
				return Token{NEQ, "!="}
			} else {
				panic(l.errorMessage(fmt.Sprintf("Unexpected character: %c", ch)))
			}
		case ch == ':':
			if l.pos+1 < len(l.text) && l.text[l.pos+1] == ':' {
				l.pos += 2
				l.column += 2
				return Token{DOUBLE_COLON, "::"}
			} else {
				l.pos++
				l.column++
				return Token{COLON, ":"}
			}
		case ch == '<':
			if l.pos+1 < len(l.text) && l.text[l.pos+1] == '=' {
				l.pos += 2
				l.column += 2
				return Token{LEQ, "<="}
			} else {
				l.pos++
				l.column++
				return Token{LT, "<"}
			}
		case ch == '>':
			if l.pos+1 < len(l.text) && l.text[l.pos+1] == '=' {
				l.pos += 2
				l.column += 2
				return Token{GEQ, ">="}
			} else {
				l.pos++
				l.column++
				return Token{GT, ">"}
			}
		case ch == '{':
			l.pos++
			l.column++
			return Token{LBRACE, "{"}
		case ch == '}':
			l.pos++
			l.column++
			return Token{RBRACE, "}"}
		case ch == ';':
			l.pos++
			l.column++
			return Token{SEMI, ";"}
		case ch == '[':
			l.pos++
			l.column++
			return Token{LBRACKET, "["}
		case ch == ']':
			l.pos++
			l.column++
			return Token{RBRACKET, "]"}
		case ch == ',':
			l.pos++
			l.column++
			return Token{COMMA, ","}
		case ch == '.':
			// 检查是否是 ...（三个连续的点）
			if l.pos+2 < len(l.text) && l.text[l.pos+1] == '.' && l.text[l.pos+2] == '.' {
				//fmt.Println("[DEBUG] 识别到 ELLIPSIS token")
				l.pos += 3
				l.column += 3
				return Token{ELLIPSIS, "..."}
			} else {
				// 单个点
				l.pos++
				l.column++
				return Token{DOT, "."}
			}
		case ch == ':':
			if l.pos+2 < len(l.text) && l.text[l.pos+1] == '.' && l.text[l.pos+2] == '.' {
				l.pos += 3
				l.column += 3
				return Token{ELLIPSIS, "..."}
			} else {
				l.pos++
				l.column++
				return Token{DOT, "."}
			}
		case ch == '@':
			l.pos++
			l.column++
			return Token{AT, "@"}
		case ch == '"':
			return l.readString()
		case unicode.IsDigit(ch):
			return l.readNumber()
		case unicode.IsLetter(ch):
			return l.readIdentifier()
		default:
			panic(l.errorMessage(fmt.Sprintf("Unknown character: %c", ch)))
		}
	}
	return Token{EOF, ""}
}

func (l *Lexer) readNumber() Token {
	start := l.pos
	hasDot := false
	hasExponent := false // 支持科学计数法

	for l.pos < len(l.text) {
		ch := l.text[l.pos]

		if ch == '.' {
			if hasDot || hasExponent {
				panic(l.errorMessage("Invalid number format"))
			}
			hasDot = true
		} else if ch == 'e' || ch == 'E' {
			// 科学计数法
			if hasExponent {
				panic(l.errorMessage("Invalid number format"))
			}
			hasExponent = true
			// 检查指数符号
			l.pos++
			if l.pos < len(l.text) && (l.text[l.pos] == '+' || l.text[l.pos] == '-') {
				l.pos++
			}
			continue
		} else if !unicode.IsDigit(ch) {
			break
		}
		l.pos++
		l.column++
	}

	value := string(l.text[start:l.pos])

	// 验证数字格式
	if hasDot {
		if value == "." {
			panic(l.errorMessage("Invalid number format"))
		}
		// 检查是否有数字在点号前后
		hasDigits := false
		for _, r := range value {
			if unicode.IsDigit(r) {
				hasDigits = true
				break
			}
		}
		if !hasDigits {
			panic(l.errorMessage("Invalid number format"))
		}
		return Token{FLOAT, value}
	}
	return Token{INT, value}
}

type UseStatement struct {
	PackageName string
	Alias       string // 可选别名
}

type NamespacedFunctionCall struct {
	PackageName  string
	FunctionName string
	Args         []ASTNode
}

type FileAppend struct {
	PathExpr    ASTNode
	ContentExpr ASTNode
}

func (fa FileAppend) Eval(ctx *ExecutionContext) interface{} {
	pathVal := fa.PathExpr.Eval(ctx)
	contentVal := fa.ContentExpr.Eval(ctx)

	path, ok1 := pathVal.(string)
	content, ok2 := contentVal.(string)

	if !ok1 || !ok2 {
		panic("file.append 需要字符串参数")
	}

	// 安全检查
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		panic("非法文件路径")
	}

	// 以追加模式打开文件，不存在则创建
	f, err := os.OpenFile(cleanPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("文件追加失败: %v", err))
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		panic(fmt.Sprintf("写入内容失败: %v", err))
	}

	return nil
}

func (l *Lexer) readIdentifier() Token {
	start := l.pos
	for l.pos < len(l.text) {
		ch := l.text[l.pos]
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			break
		}
		l.pos++
		l.column++
	}
	value := string(l.text[start:l.pos])

	switch value {
	case "server":
		return Token{SERVER, value}
	case "get":
		return Token{ROUTE_GET, value}
	case "post":
		return Token{ROUTE_POST, value}
	case "put":
		return Token{ROUTE_PUT, value}
	case "listen":
		return Token{LISTEN, value}
	case "stop":
		return Token{SERVER_STOP, value}
	case "wait":
		return Token{SERVER_WAIT, value}
	case "append":
		return Token{APPEND, value}
	case "exit": // 新增：退出语句
		return Token{EXIT, value}
	case "super":
		return Token{SUPER, value}
	case "this": // 添加this关键字支持
		return Token{THIS, value}
	case "obj": // 新增：对象定义
		return Token{OBJECT_TYPE, value}
	case "let": // 新增：对象成员定义
		return Token{LET, value}
	case "record":
		return Token{KEY_RECORD, value}
	case "stopRecord":
		return Token{KEY_STOP_RECORD, value}
	case "title": // 新增：窗口标题
		return Token{TITLE, value}
	case "test":
		return Token{TEST, value}
	case "assert":
		return Token{ASSERT, value}
	case "runTests":
		return Token{RUN_TESTS, value}
	case "clear":
		return Token{CLEAR, value}
	case "image_find":
		return Token{IMAGE_FIND, value}
	case "image_click":
		return Token{IMAGE_CLICK, value}
	case "image_wait":
		return Token{IMAGE_WAIT, value}
	case "image_wait_disappear":
		return Token{IMAGE_WAIT_DISAPPEAR, value}
	case "image_exists":
		return Token{IMAGE_EXISTS, value}
	case "image_match":
		return Token{IMAGE_MATCH, value}
	case "screenshot":
		return Token{SCREENSHOT, value}
	case "region":
		return Token{REGION, value}
	case "confidence":
		return Token{CONFIDENCE, value}
	case "wait_all":
		return Token{WAIT_ALL, value}
	case "keyboard":
		return Token{KEYBOARD, value}
	case "mouse":
		return Token{MOUSE, value}
	case "press":
		return Token{PRESS, value}
	case "release":
		return Token{RELEASE, value}
	case "move":
		return Token{MOVE, value}
	case "click":
		return Token{CLICK, value}
	case "scroll":
		return Token{SCROLL, value}
	case "left":
		return Token{LEFT, value}
	case "right":
		return Token{RIGHT, value}
	case "middle":
		return Token{MIDDLE, value}
	case "getPos":
		return Token{GET_POS, value}
	case "for..in":
		return Token{FOR_IN_COUNT, value}
	case "admin_cmd":
		return Token{ADMIN_CMD, value}
	case "and":
		return Token{AND, value}
	case "or":
		return Token{OR, value}
	case "putsf":
		return Token{PUTSF, value}
	case "putsr":
		return Token{PUTSR, value}
	case "floor":
		return Token{FLOOR, value}
	case "ceil":
		return Token{CEIL, value}
	case "round":
		return Token{ROUND, value}
	case "use":
		return Token{USE, value}
	case "as":
		return Token{AS, value}
	case "static":
		return Token{STATIC, value}
	case "replace":
		return Token{REPLACE_METHOD, value}
	case "mod":
		return Token{MOD, value}
	case "rock":
		return Token{ROCK, value}
	case "set":
		return Token{SET, value}
	case "puts":
		return Token{PUTS, value}
	case "loop":
		return Token{LOOP, value}
	case "type":
		return Token{TYPE, value}
	case "int":
		return Token{INT_TYPE, value}
	case "float":
		return Token{FLOAT_TYPE, value}
	case "bool":
		return Token{BOOL_TYPE, value}
	case "string":
		return Token{STRING_TYPE, value}
	case "true":
		return Token{TRUE, value}
	case "false":
		return Token{FALSE, value}
	case "if":
		return Token{IF, value}
	case "else":
		return Token{ELSE, value}
	case "list":
		return Token{LIST_TYPE, value}
	case "len":
		return Token{LEN, value}
	case "fn":
		return Token{DEF, value}
	case "return":
		return Token{RETURN, value}
	case "time":
		return Token{TIME, value}
	case "now":
		return Token{NOW, value}
	case "sleep":
		return Token{SLEEP, value}
	case "format":
		return Token{FORMAT, value}
	case "split":
		return Token{SPLIT, value}
	case "file":
		return Token{FILE, value}
	case "read":
		return Token{READ, value}
	case "read_lines":
		return Token{READ_LINES, value}
	case "write":
		return Token{WRITE, value}
	case "mkdir":
		return Token{MKDIR, value}
	case "try":
		return Token{TRY, value}
	case "expect":
		return Token{EXPECT, value}
	case "lambda":
		return Token{LAMBDA, value}
	case "break":
		return Token{BREAK, value}
	case "dict":
		return Token{DICT_TYPE, value}
	case "sin":
		return Token{SIN, value}
	case "cos":
		return Token{COS, value}
	case "tan":
		return Token{TAN, value}
	case "sqrt":
		return Token{SQRT, value}
	case "log":
		return Token{LOG, value}
	case "log10":
		return Token{LOG10, value}
	case "pow":
		return Token{POW, value}
	case "for":
		return Token{FOR, value}
	case "in":
		return Token{IN, value}
	case "input":
		return Token{INPUT, value}
	case "ok":
		return Token{OK, value}
	case "scan":
		return Token{QR_SCAN, value}
	case "image":
		return Token{IMAGE, value}
	case "addText":
		return Token{ADD_TEXT, value}
	case "show":
		return Token{SHOW, value}
	case "go":
		return Token{GO, value}
	case "str":
		return Token{STR_CONVERT, value}
	case "encrypt":
		return Token{ENCRYPT, value}
	case "decrypt":
		return Token{DECRYPT, value}
	case "rand":
		return Token{RAND, value}
	case "dir":
		return Token{DIR, value}
	case "delete":
		return Token{DELETE, value}
	case "defer":
		return Token{DEFER, value}
	case "add":
		return Token{ADD_METHOD, value}
	case "remove":
		return Token{REMOVE_METHOD, value}
	case "removeAt":
		return Token{REMOVE_AT_METHOD, value}
	case "os":
		return Token{OS, value}
	case "cmd":
		return Token{CMD, value}
	case "popup":
		return Token{POPUP, value}
	case "memory": // 新增
		return Token{MEMORY, value}
	default:
		return Token{IDENT, value}
	}
}

type StringReplace struct {
	StrExpr ASTNode
	OldExpr ASTNode
	NewExpr ASTNode
}

// 在AST节点定义部分添加
type ClearStmt struct{}

func (c ClearStmt) Eval(ctx *ExecutionContext) interface{} {
	// 拆分调试模式下跳过清屏，避免冲掉已打印的执行状态
	suppress := false
	if ctx != nil {
		suppress = ctx.SuppressClear
		// 子上下文（函数调用 / go 块）可能没有直接继承该字段，
		// 通过 globalEnv 兜底生效
		if !suppress && ctx.globalEnv != nil {
			suppress = ctx.globalEnv.SuppressClear
		}
	}
	if suppress {
		fmt.Println("   [拆分调试] 已跳过 clear()（保留执行输出）")
		return nil
	}

	clearScreen()
	return nil
}

// 等待所有go块结束的AST节点
type WaitAllStmt struct{}

func (w WaitAllStmt) Eval(ctx *ExecutionContext) interface{} {
	if ctx.wg != nil {
		// 等待所有goroutine完成
		ctx.wg.Wait()
	}
	return "所有goroutine已完成"
}

type ForCountLoop struct {
	VarName   string
	CountExpr ASTNode
	Body      []ASTNode
}

func (f ForCountLoop) Eval(ctx *ExecutionContext) interface{} {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(BreakError); ok {
				return
			}
			panic(r)
		}
	}()

	countVal := f.CountExpr.Eval(ctx)
	var count int

	switch v := countVal.(type) {
	case float64:
		if v != math.Trunc(v) || v < 0 {
			panic("循环次数必须是正整数")
		}
		count = int(v)
	case int:
		count = v
	default:
		panic("循环次数必须是数字")
	}

	for i := 0; i < count; i++ {
		// 设置循环变量
		ctx.env[f.VarName] = VarInfo{
			Type:  IntType,
			Value: float64(i),
		}

		for _, stmt := range f.Body {
			stmt.Eval(ctx)
		}
	}

	// 循环结束后删除循环变量
	delete(ctx.env, f.VarName)
	return nil
}

// 在 parser 中添加解析逻辑
func (p *Parser) parseForCountLoop() ASTNode {
	p.eat(FOR_IN_COUNT) // 吃掉 "for..in"

	varName := p.curr.Value
	p.eat(IDENT)

	countExpr := p.parseExpr()

	body := p.parseBlock()

	return ForCountLoop{
		VarName:   varName,
		CountExpr: countExpr,
		Body:      body,
	}
}

func (sr StringReplace) Eval(ctx *ExecutionContext) interface{} {
	strVal := sr.StrExpr.Eval(ctx)
	oldVal := sr.OldExpr.Eval(ctx)
	newVal := sr.NewExpr.Eval(ctx)

	str, ok1 := strVal.(string)
	old, ok2 := oldVal.(string)
	new, ok3 := newVal.(string)

	if !ok1 || !ok2 || !ok3 {
		panic("replace() requires string arguments")
	}

	return strings.ReplaceAll(str, old, new)
}

func (p *Parser) parseRockFunction() ASTNode {
	p.eat(ROCK)
	p.eat(LPAREN)
	p.eat(RPAREN)
	body := p.parseBlock()
	return RockFunction{Body: body}
}

func (p *Parser) parsePopupStmt() ASTNode {
	p.eat(POPUP)
	p.eat(LPAREN)
	expr := p.parseExpr()
	p.eat(RPAREN)
	p.eat(SEMI)
	return PopupStmt{Expr: expr}
}

type OsCmdExpr struct {
	Command ASTNode
}

type ListRemove struct {
	List  ASTNode
	Value ASTNode
}

type ModStmt struct {
	Path string
}

func (m ModStmt) Eval(ctx *ExecutionContext) interface{} {
	// 更严格的文件路径验证
	path := filepath.Clean(m.Path)

	// 检查路径是否安全
	if !isSafePath(path) {
		panic(fmt.Sprintf("非法模块路径: %s", path))
	}

	// 检查是否已加载
	ctx.mu.Lock()
	if _, loaded := ctx.loadedModules[path]; loaded {
		ctx.mu.Unlock()
		return nil
	}
	ctx.loadedModules[path] = true
	ctx.mu.Unlock()

	// 读取文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("无法读取模块文件: %s, 错误: %v", path, err))
	}

	// 限制模块大小 (例如10MB)
	if len(content) > 10*1024*1024 {
		panic(fmt.Sprintf("模块文件过大: %s (%d bytes)", path, len(content)))
	}

	executeModule(string(content), ctx.globalEnv)
	return nil
}

func isSafePath(path string) bool {
	// 绝对路径检查
	if filepath.IsAbs(path) {
		return false
	}

	// 目录遍历攻击检查
	if strings.Contains(path, "..") {
		return false
	}

	// 防止绝对路径别名
	if strings.Contains(path, ":/") || strings.Contains(path, ":\\") {
		return false
	}

	// 检查文件扩展名
	ext := filepath.Ext(path)
	if ext != ".bocchi" && ext != ".tuka" && ext != "" {
		return false
	}

	return true
}

func (p *Parser) parseModStmt() ASTNode {
	p.eat(MOD)
	pathToken := p.curr
	if pathToken.Type != STRING {
		panic("模块路径必须是字符串")
	}
	p.eat(STRING)
	p.eat(SEMI)
	return ModStmt{Path: pathToken.Value}
}

type ListRemoveAt struct {
	List  ASTNode
	Index ASTNode
}

type DeferStmt struct {
	Level int
	Stmt  ASTNode
}

type DeferItem struct {
	Stmt ASTNode
	Env  map[string]VarInfo
}

func (l *Lexer) errorMessage(msg string) string {
	return fmt.Sprintf("[%d:%d] %s", l.line, l.column, msg)
}

type ValueType int

const (
	IntType ValueType = iota
	FloatType
	BoolType
	StringType
	ListType
	FunctionType
	TimeType
	DictType
)

type VarInfo struct {
	Type  ValueType
	Value interface{}
}

// 在 Param 结构体中添加默认值支持
type Param struct {
	Name    string
	Default ASTNode // 如果为nil，表示没有默认值
}

type FunctionValue struct {
	Name        string // 新增：函数名，用于递归调用
	Params      []string
	Body        []ASTNode
	Env         map[string]VarInfo
	Defaults    map[string]ASTNode
	HasVarargs  bool
	VarargsName string
}

type LambdaExpr struct {
	Params []string
	Body   []ASTNode
}

func (l LambdaExpr) Eval(ctx *ExecutionContext) interface{} {
	capturedEnv := deepCopyEnv(ctx.env)
	return FunctionValue{
		Params: l.Params,
		Body:   l.Body,
		Env:    capturedEnv,
	}
}

func (p *Parser) parseLambdaExpr() ASTNode {
	p.eat(LAMBDA)
	p.eat(LPAREN)

	var params []string
	for p.curr.Type != RPAREN {
		if p.curr.Type == IDENT {
			params = append(params, p.curr.Value)
			p.eat(IDENT)
		}
		if p.curr.Type == COMMA {
			p.eat(COMMA)
		}
	}
	p.eat(RPAREN)

	// 保存当前作用域，为lambda体创建新作用域
	savedScopes := p.scopes
	p.scopes = append(p.scopes, make(map[string]VarInfo))

	// 将参数添加到新作用域
	for _, param := range params {
		p.scopes[len(p.scopes)-1][param] = VarInfo{
			Type:  StringType, // 使用StringType作为默认类型
			Value: "",         // 使用空字符串作为默认值
		}
	}

	body := p.parseBlock()

	// 恢复原来的作用域
	p.scopes = savedScopes

	return LambdaExpr{Params: params, Body: body}
}

type ReturnValue struct {
	Value interface{}
}

type BreakError struct{}

var staticVariables = make(map[string]VarInfo)
var staticMutex sync.Mutex

type ExecutionContext struct {
	env             map[string]VarInfo
	deferStack      map[int][]DeferItem
	wg              *sync.WaitGroup
	mu              sync.Mutex
	globalEnv       *ExecutionContext
	globalMu        *sync.Mutex
	loadedModules   map[string]bool
	profiler        *Profiler
	goroutineCount  int64
	goroutineErrors []string
	Args            []string
	SuppressClear   bool // 新增：拆分调试模式下为 true，跳过 clear() 以免冲掉调试输出
}

type OkStmt struct {
	VarName string
	Expr    ASTNode
}

func (o OkStmt) Eval(ctx *ExecutionContext) interface{} {
	value := o.Expr.Eval(ctx)

	if ctx.globalEnv == nil {
		panic("Global environment not available")
	}

	// 使用全局互斥锁确保线程安全
	ctx.globalMu.Lock()
	defer ctx.globalMu.Unlock()

	// 检查变量是否已存在并获取其类型信息
	if existingInfo, exists := ctx.globalEnv.env[o.VarName]; exists {
		// 如果变量已存在，进行类型检查
		switch existingInfo.Type {
		case IntType:
			if f, ok := value.(float64); ok {
				if f != math.Trunc(f) {
					panic("Type mismatch: expected integer")
				}
				ctx.globalEnv.env[o.VarName] = VarInfo{IntType, f}
			} else {
				panic("Type mismatch: expected number")
			}
		case FloatType:
			if _, ok := value.(float64); !ok {
				panic("Type mismatch: expected float")
			}
			ctx.globalEnv.env[o.VarName] = VarInfo{FloatType, value}
		case BoolType:
			if _, ok := value.(bool); !ok {
				panic("Type mismatch: expected boolean")
			}
			ctx.globalEnv.env[o.VarName] = VarInfo{BoolType, value}
		case StringType:
			if _, ok := value.(string); !ok {
				panic("Type mismatch: expected string")
			}
			ctx.globalEnv.env[o.VarName] = VarInfo{StringType, value}
		default:
			// 对于其他类型，允许动态类型转换
			ctx.globalEnv.env[o.VarName] = VarInfo{Type: getValueType(value), Value: value}
		}
	} else {
		// 如果变量不存在，创建新变量
		ctx.globalEnv.env[o.VarName] = VarInfo{Type: getValueType(value), Value: value}
	}
	return nil
}

func (p *Parser) parseOkStmt() ASTNode {
	p.eat(OK)
	p.eat(LPAREN)
	varName := p.curr.Value
	p.eat(IDENT)
	p.eat(RPAREN)
	p.eat(ASSIGN)
	expr := p.parseExpr()
	p.eat(SEMI)
	return OkStmt{VarName: varName, Expr: expr}
}

func (t TempTypeConversion) Eval(ctx *ExecutionContext) interface{} {
	val := t.Expr.Eval(ctx)
	return convertType(val, t.TargetType)
}

func (d DeferStmt) Eval(ctx *ExecutionContext) interface{} {
	envCopy := deepCopyEnv(ctx.env)

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if ctx.deferStack == nil {
		ctx.deferStack = make(map[int][]DeferItem)
	}

	ctx.deferStack[d.Level] = append(ctx.deferStack[d.Level], DeferItem{
		Stmt: d.Stmt,
		Env:  envCopy,
	})
	return nil
}

func (p *Parser) parseDeferStmt() ASTNode {
	p.eat(DEFER)
	level := 0
	for p.curr.Type == LT {
		level++
		p.eat(LT)
	}
	stmt := p.parseStatement()
	return DeferStmt{Level: level, Stmt: stmt}
}

func (a AtGoBlock) Eval(ctx *ExecutionContext) interface{} {
	ctx.mu.Lock()
	// 深度复制环境，确保goroutine独立
	newEnv := deepCopyEnv(ctx.env)

	// 添加必要的goroutine标识
	goID := atomic.AddInt64(&ctx.goroutineCount, 1)
	ctx.mu.Unlock()

	ctx.wg.Add(1)
	go func(id int64, envCopy map[string]VarInfo) {
		defer func() {
			if r := recover(); r != nil {
				// 更好的goroutine panic处理
				ctx.mu.Lock()
				ctx.goroutineErrors = append(ctx.goroutineErrors,
					fmt.Sprintf("goroutine %d panic: %v", id, r))
				ctx.mu.Unlock()
				debug.PrintStack()
			}
			ctx.wg.Done()
		}()

		// 创建独立的执行上下文
		childCtx := &ExecutionContext{
			env:             envCopy,
			wg:              ctx.wg,
			mu:              sync.Mutex{},
			globalEnv:       ctx.globalEnv,
			globalMu:        ctx.globalMu,
			deferStack:      make(map[int][]DeferItem),
			goroutineCount:  0,
			goroutineErrors: []string{},
		}

		for _, stmt := range a.Block {
			stmt.Eval(childCtx)
		}
	}(goID, newEnv)

	return nil
}

func (a AtPutStmt) Eval(ctx *ExecutionContext) interface{} {
	var parts []string
	for _, expr := range a.Exprs {
		val := expr.Eval(ctx)
		parts = append(parts, fmt.Sprint(val))
	}
	fmt.Println(strings.Join(parts, " "))
	return nil
}

func (fd FileDelete) Eval(ctx *ExecutionContext) interface{} {
	path := filepath.Clean(fd.PathExpr.Eval(ctx).(string))
	if strings.Contains(path, "..") {
		panic("非法文件路径")
	}
	if err := os.Remove(path); err != nil {
		panic(fmt.Sprintf("删除文件失败: %v", err))
	}
	return "文件已删除: " + path
}

func (dd DirDelete) Eval(ctx *ExecutionContext) interface{} {
	path := filepath.Clean(dd.PathExpr.Eval(ctx).(string))
	if strings.Contains(path, "..") {
		panic("非法目录路径")
	}
	if err := os.RemoveAll(path); err != nil {
		panic(fmt.Sprintf("删除目录失败: %v", err))
	}
	return "目录已删除: " + path
}

func (q QRScanExpr) Eval(ctx *ExecutionContext) interface{} {
	pathVal := q.PathExpr.Eval(ctx).(string)
	result, err := scanQRCode(pathVal)
	if err != nil {
		panic(err.Error())
	}
	return result
}

func (a AutoDeclaration) Eval(ctx *ExecutionContext) interface{} {
	val := a.Expr.Eval(ctx)
	varType := getValueType(val)

	// 调试：打印正在定义的变量
	// fmt.Printf("[DEBUG] 定义变量 %s = %v (类型: %v)\n", a.Name, val, varType)

	ctx.env[a.Name] = VarInfo{
		Type:  varType,
		Value: val,
	}
	return nil
}

func (p *Parser) parseLogicalOr() ASTNode {
	node := p.parseLogicalAnd()

	for p.curr.Type == OR {
		op := p.curr.Type
		p.eat(OR)
		right := p.parseLogicalAnd()
		node = BinOp{
			left:  node,
			op:    op,
			right: right,
		}
	}
	return node
}

func (p *Parser) parseLogicalAnd() ASTNode {
	node := p.parseComparison()

	for p.curr.Type == AND {
		op := p.curr.Type
		p.eat(AND)
		right := p.parseComparison()
		node = BinOp{
			left:  node,
			op:    op,
			right: right,
		}
	}
	return node
}

type StaticDeclaration struct {
	VarType TokenType
	Name    string
	Expr    ASTNode
}

func (s StaticDeclaration) Eval(ctx *ExecutionContext) interface{} {
	staticMutex.Lock()
	defer staticMutex.Unlock()

	// 如果静态变量已经存在，不重新初始化
	if _, exists := staticVariables[s.Name]; exists {
		return nil
	}

	val := s.Expr.Eval(ctx)
	var vt ValueType

	switch s.VarType {
	case INT_TYPE:
		vt = IntType
		if f, ok := val.(float64); ok {
			if f != math.Trunc(f) {
				panic("Cannot assign float to int variable")
			}
		} else {
			panic("Cannot assign non-number to int variable")
		}
	case FLOAT_TYPE:
		vt = FloatType
		if _, ok := val.(float64); !ok {
			panic("Cannot assign non-number to float variable")
		}
	case BOOL_TYPE:
		vt = BoolType
		if _, ok := val.(bool); !ok {
			panic("Cannot assign non-boolean to bool variable")
		}
	case STRING_TYPE:
		vt = StringType
		if _, ok := val.(string); !ok {
			panic("Cannot assign non-string to string variable")
		}
	case LIST_TYPE:
		vt = ListType
		if _, ok := val.([]interface{}); !ok {
			panic("Cannot assign non-list to list variable")
		}
	case DICT_TYPE:
		vt = DictType
		if _, ok := val.(map[string]interface{}); !ok {
			panic("Cannot assign non-dict to dict variable")
		}
	default:
		panic("Unknown type")
	}

	staticVariables[s.Name] = VarInfo{vt, val}
	return nil
}

func (i InputExpr) Eval(ctx *ExecutionContext) interface{} {
	var prompt string
	if i.Prompt != nil {
		prompt = i.Prompt.Eval(ctx).(string)
		fmt.Print(prompt)
	}

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func (b BlockStmt) Eval(ctx *ExecutionContext) interface{} {
	for _, stmt := range b.stmts {
		stmt.Eval(ctx)
	}
	return nil
}

func (b BreakStmt) Eval(ctx *ExecutionContext) interface{} {
	panic(BreakError{})
}

func (tc TypeConversion) Eval(ctx *ExecutionContext) interface{} {
	varInfo, exists := ctx.env[tc.VarName]
	if !exists {
		panic(fmt.Sprintf("未定义变量: %s", tc.VarName))
	}

	newValue := convertType(varInfo.Value, tc.TargetType)
	ctx.env[tc.VarName] = VarInfo{
		Type:  getTypeFromToken(tc.TargetType),
		Value: newValue,
	}
	return nil
}

func (n Number) Eval(ctx *ExecutionContext) interface{} {
	// 对于特别大的数字，我们可以考虑返回big.Float
	// 但为了保持兼容性，暂时仍然返回float64
	// 在输出时通过formatLargeNumber处理显示问题
	return n.value
}

func (b Boolean) Eval(ctx *ExecutionContext) interface{} {
	return b.value
}

func (s String) Eval(ctx *ExecutionContext) interface{} {
	return s.value
}

func (v Variable) Eval(ctx *ExecutionContext) interface{} {
	// 首先检查静态变量
	staticMutex.Lock()
	if val, ok := staticVariables[v.name]; ok {
		staticMutex.Unlock()
		return val.Value
	}
	staticMutex.Unlock()

	// 然后尝试从环境变量中获取值
	if val, ok := ctx.env[v.name]; ok {
		return val.Value
	}

	// 如果当前环境中没有，尝试从全局环境中获取
	if ctx.globalEnv != nil {
		ctx.globalEnv.mu.Lock()
		defer ctx.globalEnv.mu.Unlock()
		if val, ok := ctx.globalEnv.env[v.name]; ok {
			return val.Value
		}
	}

	panic(fmt.Sprintf("Undefined variable: %s", v.name))
}

func (d DictLiteral) Eval(ctx *ExecutionContext) interface{} {
	dict := make(map[string]interface{})
	for key, valueNode := range d.Elements {
		dict[key] = valueNode.Eval(ctx)
	}
	return dict
}

func (la ListAccess) Eval(ctx *ExecutionContext) interface{} {
	containerVal := la.List.Eval(ctx)
	indexVal := la.Index.Eval(ctx)

	switch container := containerVal.(type) {
	case []interface{}:
		index, ok := indexVal.(float64)
		if !ok || index != math.Trunc(index) {
			panic("Invalid list index")
		}
		i := int(index)
		if i < 0 || i >= len(container) {
			panic("List index out of range")
		}
		return container[i]
	case map[string]interface{}:
		key, ok := indexVal.(string)
		if !ok {
			panic("Dictionary key must be string")
		}
		return container[key]
	default:
		panic(fmt.Sprintf("Not a list or dict: %T", containerVal))
	}
}

func (m MathFunction) Eval(ctx *ExecutionContext) interface{} {
	switch m.Func {
	case FLOOR:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("floor函数需要数字参数")
		}
		return math.Floor(num)
	case CEIL:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("ceil函数需要数字参数")
		}
		return math.Ceil(num)
	case ROUND:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("round函数需要数字参数")
		}
		return math.Round(num)
	case SIN:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("Math function requires numeric argument")
		}
		return math.Sin(num)
	case COS:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("Math function requires numeric argument")
		}
		return math.Cos(num)
	case TAN:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("Math function requires numeric argument")
		}
		return math.Tan(num)
	case SQRT:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("sqrt需要数字参数")
		}
		if num < 0 {
			panic("sqrt的参数不能为负数")
		}
		result := math.Sqrt(num)
		if math.IsInf(result, 0) {
			return formatLargeNumber(result)
		}
		return result
	case LOG:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("Math function requires numeric argument")
		}
		return math.Log(num)
	case LOG10:
		arg := m.Args[0].Eval(ctx)
		num, ok := arg.(float64)
		if !ok {
			panic("Math function requires numeric argument")
		}
		return math.Log10(num)
	case POW:
		base := m.Args[0].Eval(ctx)
		exp := m.Args[1].Eval(ctx)
		baseNum, ok1 := base.(float64)
		expNum, ok2 := exp.(float64)
		if !ok1 || !ok2 {
			panic("pow requires numeric arguments")
		}
		result := math.Pow(baseNum, expNum)
		if math.IsInf(result, 0) {
			return formatLargeNumber(result)
		}
		return result
	default:
		panic("Unknown math function")
	}
}

func (a Assignment) Eval(ctx *ExecutionContext) interface{} {
	// 添加 recover 机制来捕获 panic 并提供更好的错误信息
	defer func() {
		if r := recover(); r != nil {
			// 根据左值类型提供更具体的错误信息
			var leftDesc string
			switch left := a.Left.(type) {
			case Variable:
				leftDesc = fmt.Sprintf("变量 '%s'", left.name)
			case ListAccess:
				leftDesc = "列表元素"
			default:
				leftDesc = fmt.Sprintf("赋值目标 (%T)", a.Left)
			}
			panic(fmt.Sprintf("赋值操作错误 %s: %v", leftDesc, r))
		}
	}()

	// 评估右值表达式
	val := a.Expr.Eval(ctx)

	// 处理不同类型的左值
	switch left := a.Left.(type) {
	case Variable:
		return a.evalVariableAssignment(left, val, ctx)

	case ListAccess:
		return a.evalListAccessAssignment(left, val, ctx)

	default:
		panic(fmt.Sprintf("无效的赋值目标类型: %T", a.Left))
	}
}

// 处理变量赋值
func (a Assignment) evalVariableAssignment(left Variable, val interface{}, ctx *ExecutionContext) interface{} {
	varName := left.name

	// 调试：打印变量赋值
	// fmt.Printf("[DEBUG] 赋值给变量 %s = %v\n", varName, val)

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// 检查变量是否在当前环境中声明
	info, exists := ctx.env[varName]
	if !exists {
		// 如果当前环境中没有，直接创建新变量
		// 这在方法内部应该是允许的
		ctx.env[varName] = VarInfo{
			Type:  getValueType(val),
			Value: val,
		}
		return val
	}

	// 根据变量类型进行赋值
	switch info.Type {
	case IntType:
		if f, ok := val.(float64); ok {
			if f != math.Trunc(f) {
				panic("类型不匹配: 需要整数")
			}
			ctx.env[varName] = VarInfo{IntType, f}
		} else {
			panic("类型不匹配: 需要数字")
		}
	case FloatType:
		if _, ok := val.(float64); !ok {
			panic("类型不匹配: 需要浮点数")
		}
		ctx.env[varName] = VarInfo{FloatType, val}
	case BoolType:
		if _, ok := val.(bool); !ok {
			panic("类型不匹配: 需要布尔值")
		}
		ctx.env[varName] = VarInfo{BoolType, val}
	case StringType:
		if _, ok := val.(string); !ok {
			panic("类型不匹配: 需要字符串")
		}
		ctx.env[varName] = VarInfo{StringType, val}
	case ListType:
		if _, ok := val.([]interface{}); !ok {
			panic("类型不匹配: 需要列表")
		}
		ctx.env[varName] = VarInfo{ListType, val}
	case DictType:
		if _, ok := val.(map[string]interface{}); !ok {
			panic("类型不匹配: 需要字典")
		}
		ctx.env[varName] = VarInfo{DictType, val}
	case FunctionType:
		ctx.env[varName] = VarInfo{FunctionType, val}
	case TimeType:
		if _, ok := val.(time.Time); !ok {
			panic("类型不匹配: 需要时间")
		}
		ctx.env[varName] = VarInfo{TimeType, val}
	default:
		// 对于其他类型，允许动态类型转换
		ctx.env[varName] = VarInfo{Type: getValueType(val), Value: val}
	}
	return val
}

// 处理列表/字典访问赋值
func (a Assignment) evalListAccessAssignment(left ListAccess, val interface{}, ctx *ExecutionContext) interface{} {
	// 对列表访问操作加锁
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// 评估容器和索引
	containerVal := left.List.Eval(ctx)
	indexVal := left.Index.Eval(ctx)

	switch container := containerVal.(type) {
	case []interface{}:
		return a.assignToList(container, indexVal, val, left, ctx)

	case map[string]interface{}:
		return a.assignToDict(container, indexVal, val, left, ctx)

	default:
		panic(fmt.Sprintf("无法赋值给非列表或字典类型: %T", containerVal))
	}
}

// 赋值给列表元素
func (a Assignment) assignToList(container []interface{}, indexVal interface{}, val interface{}, left ListAccess, ctx *ExecutionContext) interface{} {
	index, ok := indexVal.(float64)
	if !ok || index != math.Trunc(index) {
		panic("无效的列表索引")
	}
	i := int(index)
	if i < 0 || i >= len(container) {
		panic(fmt.Sprintf("列表索引越界: %d (列表长度: %d)", i, len(container)))
	}
	container[i] = val

	// 如果左值是一个变量，需要更新环境中的列表引用
	if varNode, ok := left.List.(Variable); ok {
		ctx.env[varNode.name] = VarInfo{
			Type:  ListType,
			Value: container,
		}
	}

	return val
}

// 赋值给字典元素
func (a Assignment) assignToDict(container map[string]interface{}, indexVal interface{}, val interface{}, left ListAccess, ctx *ExecutionContext) interface{} {
	key, ok := indexVal.(string)
	if !ok {
		panic("字典键必须是字符串")
	}
	container[key] = val

	// 如果左值是一个变量，需要更新环境中的字典引用
	if varNode, ok := left.List.(Variable); ok {
		ctx.env[varNode.name] = VarInfo{
			Type:  DictType,
			Value: container,
		}
	}

	return val
}

func (b BinOp) Eval(ctx *ExecutionContext) interface{} {
	leftVal := b.left.Eval(ctx)
	rightVal := b.right.Eval(ctx)

	switch b.op {
	case AND:
		leftBool := isTruthy(leftVal)
		if !leftBool {
			return false
		}
		rightBool := isTruthy(rightVal)
		return leftBool && rightBool

	case OR:
		leftBool := isTruthy(leftVal)
		if leftBool {
			return true
		}
		rightBool := isTruthy(rightVal)
		return leftBool || rightBool
	case PLUS:
		switch l := leftVal.(type) {
		case float64:
			switch r := rightVal.(type) {
			case float64:
				result := l + r
				// 检查是否溢出
				if math.IsInf(result, 0) {
					// 返回big.Float来处理大数
					bf1 := big.NewFloat(l)
					bf2 := big.NewFloat(r)
					sum := new(big.Float).Add(bf1, bf2)
					return sum
				}
				return result
			}
		case *big.Float:
			switch r := rightVal.(type) {
			case float64:
				bf2 := big.NewFloat(r)
				return new(big.Float).Add(l, bf2)
			case *big.Float:
				return new(big.Float).Add(l, r)
			}
		case string:
			switch r := rightVal.(type) {
			case string:
				return l + r
			}
		}
		panic("Invalid operands for +")

	case MUL:
		switch l := leftVal.(type) {
		case float64:
			switch r := rightVal.(type) {
			case float64:
				result := l * r
				if math.IsInf(result, 0) {
					bf1 := big.NewFloat(l)
					bf2 := big.NewFloat(r)
					product := new(big.Float).Mul(bf1, bf2)
					return product
				}
				return result
			}
		case *big.Float:
			switch r := rightVal.(type) {
			case float64:
				bf2 := big.NewFloat(r)
				return new(big.Float).Mul(l, bf2)
			case *big.Float:
				return new(big.Float).Mul(l, r)
			}
		}
		panic("Invalid operands for *")
	case MOD:
		l, ok1 := leftVal.(float64)
		r, ok2 := rightVal.(float64)
		if !ok1 || !ok2 {
			panic("Invalid operands for %")
		}
		if r == 0 {
			panic("Modulo by zero")
		}
		return math.Mod(l, r)

	case POW:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				result := math.Pow(l, r)
				if math.IsInf(result, 0) {
					// 对于大数幂运算，返回字符串表示
					return formatLargeNumber(result)
				}
				return result
			}
		}
		panic("Invalid operands for ^")
	case LT:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l < r
			}
		}
		panic("Invalid operands for <")
	case GT:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l > r
			}
		}
		panic("Invalid operands for >")
	case LEQ:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l <= r
			}
		}
		panic("Invalid operands for <=")
	case GEQ:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l >= r
			}
		}
		panic("Invalid operands for >=")
	case EQ:
		return leftVal == rightVal
	case NEQ:
		return leftVal != rightVal
	default:
		return b.evalOriginal(ctx)
	}
}

func (b BinOp) evalOriginal(ctx *ExecutionContext) interface{} {
	leftVal := b.left.Eval(ctx)
	rightVal := b.right.Eval(ctx)

	switch b.op {
	case AND:
		leftBool := isTruthy(leftVal)
		if !leftBool {
			return false
		}
		rightBool := isTruthy(rightVal)
		return leftBool && rightBool

	case OR:
		leftBool := isTruthy(leftVal)
		if leftBool {
			return true
		}
		rightBool := isTruthy(rightVal)
		return leftBool || rightBool
	case PLUS:
		switch l := leftVal.(type) {
		case float64:
			switch r := rightVal.(type) {
			case float64:
				return l + r
			}
		case string:
			switch r := rightVal.(type) {
			case string:
				return l + r
			}
		}
		panic("Invalid operands for +")
	case MINUS:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l - r
			}
		}
		panic("Invalid operands for -")
	case MUL:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l * r
			}
		}
		panic("Invalid operands for *")
	case MOD:
		l, ok1 := leftVal.(float64)
		r, ok2 := rightVal.(float64)
		if !ok1 || !ok2 {
			panic("Invalid operands for %")
		}
		if r == 0 {
			panic("Modulo by zero")
		}
		return math.Mod(l, r)
	case DIV:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				if r == 0 {
					panic("Division by zero")
				}
				return l / r
			}
		}
		panic("Invalid operands for /")
	case POW:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return math.Pow(l, r)
			}
		}
		panic("Invalid operands for ^")
	case LT:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l < r
			}
		}
		panic("Invalid operands for <")
	case GT:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l > r
			}
		}
		panic("Invalid operands for >")
	case LEQ:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l <= r
			}
		}
		panic("Invalid operands for <=")
	case GEQ:
		if l, ok := leftVal.(float64); ok {
			if r, ok := rightVal.(float64); ok {
				return l >= r
			}
		}
		panic("Invalid operands for >=")
	case EQ:
		return leftVal == rightVal
	case NEQ:
		return leftVal != rightVal
	default:
		panic("Unknown operator")
	}
}

func (p PrintStmt) Eval(ctx *ExecutionContext) interface{} {
	var output strings.Builder
	for i, expr := range p.Exprs {
		val := expr.Eval(ctx)

		// 使用新的大数格式化函数
		var str string
		if p.Prec >= 0 {
			// 如果指定了精度，使用原来的逻辑
			switch v := val.(type) {
			case float64:
				str = strconv.FormatFloat(v, 'f', p.Prec, 64)
			default:
				str = fmt.Sprintf("%v", val)
			}
		} else {
			// 使用新的大数格式化
			str = formatLargeNumber(val)
		}

		if i > 0 {
			output.WriteString(" ")
		}
		output.WriteString(str)
	}

	fmt.Println(output.String())
	return nil
}

func (w WhileLoop) Eval(ctx *ExecutionContext) interface{} {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(BreakError); ok {
				return
			}
			panic(r)
		}
	}()

	for {
		cond := w.condition.Eval(ctx)
		switch c := cond.(type) {
		case bool:
			if !c {
				return nil
			}
		default:
			panic("Loop condition must be boolean")
		}
		for _, stmt := range w.body {
			stmt.Eval(ctx)
		}
	}
}

func (f ForLoop) Eval(ctx *ExecutionContext) interface{} {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(BreakError); ok {
				return
			}
			panic(r)
		}
	}()

	if f.Init != nil {
		f.Init.Eval(ctx)
	}

	for {
		condVal := f.Condition.Eval(ctx)
		cond, ok := condVal.(bool)
		if !ok || !cond {
			break
		}

		for _, stmt := range f.Body {
			stmt.Eval(ctx)
		}

		if f.Update != nil {
			f.Update.Eval(ctx)
		}
	}
	return nil
}

func (f ForRange) Eval(ctx *ExecutionContext) interface{} {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(BreakError); ok {
				return
			}
			panic(r)
		}
	}()

	iterableVal := f.Iterable.Eval(ctx)

	switch iterable := iterableVal.(type) {
	case []interface{}:
		for _, item := range iterable {
			ctx.env[f.VarName] = VarInfo{
				Type:  getValueType(item),
				Value: item,
			}
			for _, stmt := range f.Body {
				stmt.Eval(ctx)
			}
		}
	case map[string]interface{}:
		for key, value := range iterable {
			ctx.env[f.VarName] = VarInfo{
				Type:  StringType,
				Value: key + ":" + fmt.Sprint(value),
			}
			for _, stmt := range f.Body {
				stmt.Eval(ctx)
			}
		}
	default:
		panic("Cannot iterate over non-iterable type")
	}
	return nil
}

func (i IfStmt) Eval(ctx *ExecutionContext) interface{} {
	condVal := i.Condition.Eval(ctx)
	cond, ok := condVal.(bool)
	if !ok {
		panic("Condition must be boolean")
	}

	if cond {
		for _, stmt := range i.Then {
			stmt.Eval(ctx)
		}
	} else if len(i.Else) > 0 {
		for _, stmt := range i.Else {
			stmt.Eval(ctx)
		}
	}
	return nil
}

type TempTypeConversion struct {
	Expr       ASTNode
	TargetType TokenType
}

func convertType(val interface{}, target TokenType) interface{} {
	switch target {
	case INT_TYPE:
		switch v := val.(type) {
		case float64:
			return int(v)
		case string:
			i, err := strconv.Atoi(v)
			if err != nil {
				panic(fmt.Sprintf("无法将字符串 '%s' 转换为整数", v))
			}
			return i
		case bool:
			if v {
				return 1
			}
			return 0
		default:
			panic(fmt.Sprintf("无法转换 %T 为 int", val))
		}
	case FLOAT_TYPE:
		switch v := val.(type) {
		case int:
			return float64(v)
		case float64:
			return v
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				panic(fmt.Sprintf("无法将字符串 '%s' 转换为浮点数", v))
			}
			return f
		default:
			panic(fmt.Sprintf("无法转换 %T 为 float", val))
		}
	case STRING_TYPE:
		return fmt.Sprintf("%v", val)
	default:
		panic("不支持的临时转换类型")
	}
}

type ASTNode interface {
	Eval(ctx *ExecutionContext) interface{}
}

type GoBlock struct {
	Block []ASTNode
}

type AtGoBlock struct {
	Block []ASTNode
}

type ListAdd struct {
	List  ASTNode
	Value ASTNode
}

const (
	saltSize = 16
	keySize  = 32
	iter     = 10000
)

type RandomExpr struct {
	Args []ASTNode
}

type FileDelete struct {
	PathExpr ASTNode
}

type AtPutStmt struct {
	Exprs []ASTNode
}

type DirDelete struct {
	PathExpr ASTNode
}
type ImageAddTextExpr struct {
	PathExpr  ASTNode
	TextExpr  ASTNode
	SizeExpr  ASTNode
	XExpr     ASTNode
	YExpr     ASTNode
	ColorExpr ASTNode
}

func addTextToImage(originalPath, text string, size, x, y int, colorStr string) (string, error) {
	ext := filepath.Ext(originalPath)
	base := strings.TrimSuffix(originalPath, ext)
	newPath := base + "_text" + ext

	fontDirs := []string{
		"C:\\Windows\\Fonts",
		"/usr/share/fonts",
		"/Library/Fonts",
	}
	fontFiles := []string{"arial.ttf", "msyh.ttc", "simhei.ttf", "simsun.ttc", "Arial_Unicode.ttf"}
	var fontPath string
	for _, dir := range fontDirs {
		for _, font := range fontFiles {
			candidate := filepath.Join(dir, font)
			if _, err := os.Stat(candidate); err == nil {
				fontPath = candidate
				break
			}
		}
		if fontPath != "" {
			break
		}
	}
	if fontPath == "" {
		return "", fmt.Errorf("未找到支持的字体文件")
	}

	img, err := gg.LoadImage(originalPath)
	if err != nil {
		return "", fmt.Errorf("图片加载失败: %v", err)
	}

	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	dc := gg.NewContext(width, height)
	dc.DrawImage(img, 0, 0)

	if err := dc.LoadFontFace(fontPath, float64(size)); err != nil {
		return "", fmt.Errorf("字体加载失败: %v", err)
	}

	var r, g, b int
	if strings.Contains(colorStr, ",") {
		parts := strings.Split(colorStr, ",")
		if len(parts) != 3 {
			return "", errors.New("颜色格式错误，应为 R,G,B")
		}
		var err error
		r, err = strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return "", errors.New("红色值无效")
		}
		g, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return "", errors.New("绿色值无效")
		}
		b, err = strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			return "", errors.New("蓝色值无效")
		}
	} else {
		return "", errors.New("颜色格式错误")
	}

	dc.SetRGB(float64(r)/255, float64(g)/255, float64(b)/255)

	maxWidth := float64(width) - float64(x)*2
	dc.DrawStringWrapped(text, float64(x), float64(y), 0, 0, maxWidth, 1.5, gg.AlignLeft)

	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		err = dc.SavePNG(newPath)
	case ".png":
		err = dc.SavePNG(newPath)
	default:
		return "", errors.New("不支持的图片格式")
	}

	if err != nil {
		return "", fmt.Errorf("保存失败: %v", err)
	}
	return "处理完成: " + newPath, nil
}

type QRScanExpr struct {
	PathExpr ASTNode
}

func scanQRCode(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("无法打开文件: %v", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("图片解码失败: %v", err)
	}

	bmp, _ := gozxing.NewBinaryBitmapFromImage(img)
	qrReader := qrcode.NewQRCodeReader()

	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return "", fmt.Errorf("未找到二维码：%v", err)
	}

	return result.GetText(), nil
}

type AutoDeclaration struct {
	Name string
	Expr ASTNode
}

type InputExpr struct {
	Prompt ASTNode
}

type BlockStmt struct {
	stmts []ASTNode
}

type BreakStmt struct{}

type TypeConversion struct {
	VarName    string
	TargetType TokenType
}

func getTypeFromToken(t TokenType) ValueType {
	switch t {
	case INT_TYPE:
		return IntType
	case FLOAT_TYPE:
		return FloatType
	case STRING_TYPE:
		return StringType
	default:
		panic("不支持的转换类型")
	}
}

type Number struct {
	value float64
}

type Boolean struct {
	value bool
}

type String struct {
	value string
}

type Variable struct {
	name string
}

type DictLiteral struct {
	Elements map[string]ASTNode
}

type ListAccess struct {
	List  ASTNode
	Index ASTNode
}

type MathFunction struct {
	Func TokenType
	Args []ASTNode
}

type Assignment struct {
	Left ASTNode
	Expr ASTNode
}

type BinOp struct {
	left  ASTNode
	op    TokenType
	right ASTNode
}

type PrintStmt struct {
	Exprs []ASTNode
	Prec  int
}

type WhileLoop struct {
	condition ASTNode
	body      []ASTNode
}

type ForLoop struct {
	Init      ASTNode
	Condition ASTNode
	Update    ASTNode
	Body      []ASTNode
}

type ForRange struct {
	VarName  string
	Iterable ASTNode
	Body     []ASTNode
}

type IfStmt struct {
	Condition ASTNode
	Then      []ASTNode
	Else      []ASTNode
}

type TypeDeclaration struct {
	VarType TokenType
	Name    string
	Expr    ASTNode
}

func (p *Parser) parseAtGoBlock() ASTNode {
	p.eat(AT)
	p.eat(GO)
	return AtGoBlock{Block: p.parseBlock()}
}

func (t TypeDeclaration) Eval(ctx *ExecutionContext) interface{} {
	val := t.Expr.Eval(ctx)

	var vt ValueType
	switch t.VarType {
	case INT_TYPE:
		vt = IntType
		if f, ok := val.(float64); ok {
			if f != math.Trunc(f) {
				panic("Cannot assign float to int variable")
			}
		} else {
			panic("Cannot assign non-number to int variable")
		}
	case FLOAT_TYPE:
		vt = FloatType
		if _, ok := val.(float64); !ok {
			panic("Cannot assign non-number to float variable")
		}
	case BOOL_TYPE:
		vt = BoolType
		if _, ok := val.(bool); !ok {
			panic("Cannot assign non-boolean to bool variable")
		}
	case STRING_TYPE:
		vt = StringType
		if _, ok := val.(string); !ok {
			panic("Cannot assign non-string to string variable")
		}
	case LIST_TYPE:
		vt = ListType
		if _, ok := val.([]interface{}); !ok {
			panic("Cannot assign non-list to list variable")
		}
	case DICT_TYPE:
		vt = DictType
		if _, ok := val.(map[string]interface{}); !ok {
			panic("Cannot assign non-dict to dict variable")
		}
	default:
		panic("Unknown type")
	}

	ctx.env[t.Name] = VarInfo{vt, val}
	return val
}

type ListLiteral struct {
	Elements []ASTNode
}

func (l ListLiteral) Eval(ctx *ExecutionContext) interface{} {
	var elements []interface{}
	for _, elem := range l.Elements {
		evaluated := elem.Eval(ctx)

		if nestedList, ok := evaluated.([]interface{}); ok {
			elements = append(elements, nestedList)
		} else {
			elements = append(elements, evaluated)
		}
	}
	return elements
}

type ListAppend struct {
	List  ASTNode
	Value ASTNode
}

func (la ListAppend) Eval(ctx *ExecutionContext) interface{} {
	listVal := la.List.Eval(ctx)
	value := la.Value.Eval(ctx)

	list, ok := listVal.([]interface{})
	if !ok {
		panic("Not a list")
	}

	for name, info := range ctx.env {
		if info.Value == listVal {
			newList := append(list, value)
			ctx.env[name] = VarInfo{
				Type:  ListType,
				Value: newList,
			}
			return newList
		}
	}
	panic("List not found in environment")
}

type ListLength struct {
	List ASTNode
}

type StrConvert struct {
	Expr ASTNode
}

func (s StrConvert) Eval(ctx *ExecutionContext) interface{} {
	return fmt.Sprint(s.Expr.Eval(ctx))
}

func (ll ListLength) Eval(ctx *ExecutionContext) interface{} {
	listVal := ll.List.Eval(ctx)
	list, ok := listVal.([]interface{})
	if !ok {
		panic("Not a list")
	}
	return float64(len(list))
}

type StringSplit struct {
	StrExpr ASTNode
	SepExpr ASTNode
}

func (ss StringSplit) Eval(ctx *ExecutionContext) interface{} {
	strVal := ss.StrExpr.Eval(ctx)
	sepVal := ss.SepExpr.Eval(ctx)

	str, ok1 := strVal.(string)
	sep, ok2 := sepVal.(string)

	if !ok1 || !ok2 {
		panic("split() requires string arguments")
	}

	parts := strings.Split(str, sep)
	result := make([]interface{}, len(parts))
	for i, p := range parts {
		result[i] = p
	}
	return result
}

type FileRead struct {
	PathExpr ASTNode
	AsLines  bool
}

func (fr FileRead) Eval(ctx *ExecutionContext) interface{} {
	pathVal := fr.PathExpr.Eval(ctx)
	path, ok := pathVal.(string)
	if !ok {
		panic("File path must be a string")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("File read error: %v", err))
	}

	if fr.AsLines {
		lines := strings.Split(string(content), "\n")
		result := make([]interface{}, len(lines))
		for i, line := range lines {
			result[i] = line
		}
		return result
	}
	return string(content)
}

type FileWrite struct {
	PathExpr    ASTNode
	ContentExpr ASTNode
}

func (fw FileWrite) Eval(ctx *ExecutionContext) interface{} {
	pathVal := fw.PathExpr.Eval(ctx)
	path, ok1 := pathVal.(string)
	contentVal := fw.ContentExpr.Eval(ctx)
	content, ok2 := contentVal.(string)

	if !ok1 || !ok2 {
		panic("file.write requires string arguments")
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		panic(fmt.Sprintf("File write error: %v", err))
	}
	return nil
}

type FileMkdir struct {
	PathExpr ASTNode
}

func (fm FileMkdir) Eval(ctx *ExecutionContext) interface{} {
	pathVal := fm.PathExpr.Eval(ctx)
	path, ok := pathVal.(string)
	if !ok {
		panic("mkdir path must be a string")
	}

	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(fmt.Sprintf("Mkdir error: %v", err))
	}
	return nil
}

type FunctionDef struct {
	Name        string
	Params      []string
	Body        []ASTNode
	Defaults    map[string]ASTNode
	HasVarargs  bool
	VarargsName string
}

func (f FunctionDef) Eval(ctx *ExecutionContext) interface{} {
	capturedEnv := deepCopyEnv(ctx.env)

	fnValue := FunctionValue{
		Params:      f.Params,
		Body:        f.Body,
		Env:         capturedEnv,
		Defaults:    f.Defaults,
		HasVarargs:  f.HasVarargs,
		VarargsName: f.VarargsName,
	}

	ctx.env[f.Name] = VarInfo{
		Type:  FunctionType,
		Value: fnValue,
	}

	return nil
}

type FunctionCall struct {
	Function   ASTNode
	Args       []ASTNode
	IsTailCall bool
}

func (fc FunctionCall) Eval(ctx *ExecutionContext) interface{} {
	// 如果是尾调用优化，使用尾调用路径
	if fc.IsTailCall {
		return fc.evalTailCall(ctx)
	}

	// 尝试优化执行
	return fc.evalOptimized(ctx)
}

func (fc FunctionCall) evalTailCall(ctx *ExecutionContext) interface{} {
	funcVal := fc.Function.Eval(ctx)
	fv, ok := funcVal.(FunctionValue)
	if !ok {
		panic("只能调用函数")
	}

	if len(fc.Args) != len(fv.Params) {
		panic("参数数量不匹配")
	}

	// 对于尾调用，我们重用当前环境而不是创建新环境
	// 这避免了调用栈的无限增长
	for i, param := range fv.Params {
		argVal := fc.Args[i].Eval(ctx)
		ctx.env[param] = VarInfo{
			Type:  getValueType(argVal),
			Value: argVal,
		}
	}

	// 直接执行函数体，不创建新的调用上下文
	// 这会使得当前函数调用的栈帧被重用
	for _, stmt := range fv.Body {
		stmt.Eval(ctx)
	}

	return nil // 尾调用的返回值由函数体中的return语句处理
}

func (p *Parser) parseFunctionCall(function ASTNode) ASTNode {
	p.eat(LPAREN)
	var args []ASTNode
	for p.curr.Type != RPAREN {
		arg := p.parseExpr()
		args = append(args, arg)
		if p.curr.Type == COMMA {
			p.eat(COMMA)
		}
	}
	p.eat(RPAREN)

	return FunctionCall{
		Function:   function,
		Args:       args,
		IsTailCall: false,
	}
}

type MemoKey struct {
	FunctionName string
	ArgsHash     string
}

type MemoCache struct {
	cache map[MemoKey]interface{}
	mutex sync.RWMutex
}

var globalMemoCache = &MemoCache{
	cache: make(map[MemoKey]interface{}),
}

func (m *MemoCache) Get(key MemoKey) (interface{}, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	val, exists := m.cache[key]
	return val, exists
}

func (m *MemoCache) Set(key MemoKey, value interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache[key] = value
}

func computeArgsHash(args []ASTNode, ctx *ExecutionContext) string {
	hash := sha256.New()
	for _, arg := range args {
		val := arg.Eval(ctx)
		fmt.Fprintf(hash, "%v", val)
	}
	return string(hash.Sum(nil))
}

// 在FunctionCall中添加缓存支持
func (fc FunctionCall) evalNormalCall(ctx *ExecutionContext) interface{} {
	// 尝试从缓存获取（仅对命名函数）
	if varNode, ok := fc.Function.(Variable); ok {
		key := MemoKey{
			FunctionName: varNode.name,
			ArgsHash:     computeArgsHash(fc.Args, ctx),
		}

		if cached, exists := globalMemoCache.Get(key); exists {
			return cached
		}

		// 执行函数并缓存结果
		result := fc.evalWithoutCache(ctx)
		globalMemoCache.Set(key, result)
		return result
	}

	return fc.evalWithoutCache(ctx)
}

// 修改 evalWithoutCache 方法中的环境设置部分
func (fc FunctionCall) evalWithoutCache(ctx *ExecutionContext) interface{} {
	funcVal := fc.Function.Eval(ctx)
	fv, ok := funcVal.(FunctionValue)
	if !ok {
		panic("只能调用函数")
	}

	// 计算实际传入的参数
	providedArgs := make([]interface{}, len(fc.Args))
	for i, arg := range fc.Args {
		providedArgs[i] = arg.Eval(ctx)
	}

	// 处理参数绑定
	actualArgs := fv.bindArguments(providedArgs, ctx)

	newEnv := deepCopyEnv(fv.Env)
	for i, param := range fv.Params {
		if i < len(actualArgs) {
			argVal := actualArgs[i]
			newEnv[param] = VarInfo{
				Type:  getValueType(argVal),
				Value: argVal,
			}
		}
	}

	var result interface{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ret, ok := r.(ReturnValue); ok {
					result = ret.Value
				} else {
					panic(r)
				}
			}
		}()

		funcCtx := &ExecutionContext{
			env:        newEnv,
			deferStack: make(map[int][]DeferItem),
			wg:         ctx.wg,
			globalEnv:  ctx.globalEnv,
		}

		for _, stmt := range fv.Body {
			stmt.Eval(funcCtx)
		}
	}()

	return result
}

func (fv FunctionValue) bindArguments(providedArgs []interface{}, ctx *ExecutionContext) []interface{} {
	expectedCount := len(fv.Params)
	providedCount := len(providedArgs)

	// 处理可变参数
	if fv.HasVarargs {
		minArgs := len(fv.Params) - 1
		if providedCount < minArgs {
			panic(fmt.Sprintf("函数需要至少 %d 个参数，提供了 %d 个", minArgs, providedCount))
		}

		result := make([]interface{}, len(fv.Params))

		// 复制固定参数
		for i := 0; i < minArgs; i++ {
			if i < providedCount {
				result[i] = providedArgs[i]
			} else if defaultVal, hasDefault := fv.Defaults[fv.Params[i]]; hasDefault {
				// 使用默认值
				result[i] = defaultVal.Eval(ctx)
			} else {
				panic(fmt.Sprintf("参数 %s 没有提供默认值", fv.Params[i]))
			}
		}

		// 处理可变参数
		varArgs := make([]interface{}, 0)
		for i := minArgs; i < providedCount; i++ {
			varArgs = append(varArgs, providedArgs[i])
		}
		result[minArgs] = varArgs

		return result
	} else {
		// 普通函数
		if providedCount > expectedCount {
			panic(fmt.Sprintf("函数需要 %d 个参数，提供了 %d 个", expectedCount, providedCount))
		}

		result := make([]interface{}, expectedCount)

		for i := 0; i < expectedCount; i++ {
			if i < providedCount {
				result[i] = providedArgs[i]
			} else if defaultVal, hasDefault := fv.Defaults[fv.Params[i]]; hasDefault {
				// 使用默认值
				result[i] = defaultVal.Eval(ctx)
			} else {
				panic(fmt.Sprintf("参数 %s 没有提供默认值", fv.Params[i]))
			}
		}

		return result
	}
}

func executeFunctionFast(fv FunctionValue, args []interface{}, ctx *ExecutionContext) interface{} {
	var result interface{}

	func() {
		defer func() {
			if r := recover(); r != nil {
				if ret, ok := r.(ReturnValue); ok {
					result = ret.Value
				} else {
					panic(r)
				}
			}
		}()

		// 修复：从闭包环境开始，而不是空环境
		newEnv := deepCopyEnv(fv.Env)

		// 绑定参数（参数会覆盖闭包中同名的变量）
		for i, param := range fv.Params {
			if i < len(args) {
				newEnv[param] = VarInfo{
					Type:  getValueType(args[i]),
					Value: args[i],
				}
			}
		}

		funcCtx := &ExecutionContext{
			env:        newEnv,
			deferStack: make(map[int][]DeferItem),
			wg:         ctx.wg,
			globalEnv:  ctx.globalEnv,
			globalMu:   ctx.globalMu,
		}

		for _, stmt := range fv.Body {
			stmt.Eval(funcCtx)
		}
	}()

	return result
}

func (fc FunctionCall) evalOptimized(ctx *ExecutionContext) interface{} {
	// 对于简单函数调用，使用快速路径
	funcVal := fc.Function.Eval(ctx)

	switch fv := funcVal.(type) {
	case FunctionValue:
		// 评估参数
		argValues := make([]interface{}, len(fc.Args))
		for i, arg := range fc.Args {
			argValues[i] = arg.Eval(ctx)
		}

		// 对于没有闭包捕获的简单函数，使用快速执行
		if len(fv.Env) == 0 && len(fv.Body) <= 5 && !fv.HasVarargs {
			return executeFunctionFast(fv, argValues, ctx)
		}

		// 否则使用正常执行路径
		return fc.evalNormalCall(ctx)

	default:
		panic("只能调用函数")
	}
}

type ReturnStmt struct {
	Expr ASTNode
}

func (rs ReturnStmt) Eval(ctx *ExecutionContext) interface{} {
	val := rs.Expr.Eval(ctx)
	panic(ReturnValue{val})
}

type TimeNow struct{}

func (t TimeNow) Eval(ctx *ExecutionContext) interface{} {
	return time.Now()
}

type TimeSleep struct {
	Duration ASTNode
}

func (t TimeSleep) Eval(ctx *ExecutionContext) interface{} {
	durVal := t.Duration.Eval(ctx)

	var dur float64
	switch v := durVal.(type) {
	case float64:
		dur = v
	case int:
		dur = float64(v)
	default:
		panic("sleep duration must be a number")
	}

	if dur < 0 {
		panic("sleep duration cannot be negative")
	}

	// 支持毫秒级精度
	time.Sleep(time.Duration(dur * float64(time.Second)))
	return nil
}

type TimeFormat struct {
	Time   ASTNode
	Layout ASTNode
}

func (t TimeFormat) Eval(ctx *ExecutionContext) interface{} {
	tm := t.Time.Eval(ctx).(time.Time)
	layout := t.Layout.Eval(ctx).(string)
	return tm.Format(layout)
}

type TryExpect struct {
	TryBlock    []ASTNode
	ExpectVar   string
	ExpectBlock []ASTNode
}

func (t TryExpect) Eval(ctx *ExecutionContext) interface{} {
	var caught interface{}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			// 关键：让 return / break 穿透，不当异常处理
			switch r.(type) {
			case ReturnValue, BreakError:
				panic(r)
			}
			caught = r
		}()
		for _, stmt := range t.TryBlock {
			stmt.Eval(ctx)
		}
	}()

	if caught == nil {
		return nil
	}

	// 构建完整的 expect 子上下文
	expectCtx := &ExecutionContext{
		env:           deepCopyEnv(ctx.env), // 保留深拷贝语义
		deferStack:    make(map[int][]DeferItem),
		wg:            ctx.wg,
		globalEnv:     ctx.globalEnv, // ← 关键：补上全局环境
		globalMu:      ctx.globalMu,
		loadedModules: ctx.loadedModules,
		profiler:      ctx.profiler,
	}
	expectCtx.mu = sync.Mutex{}
	if ctx.globalEnv == nil {
		expectCtx.globalEnv = expectCtx // 兜底，避免 nil 崩
	}
	expectCtx.env[t.ExpectVar] = VarInfo{
		Type:  StringType,
		Value: fmt.Sprintf("%v", caught),
	}

	// expect 内部再抛错时，同样不吞 return/break
	func() {
		defer func() {
			if r2 := recover(); r2 != nil {
				switch r2.(type) {
				case ReturnValue, BreakError:
					panic(r2)
				default:
					panic(r2) // 交由外层 try 或主流程处理
				}
			}
		}()
		for _, stmt := range t.ExpectBlock {
			stmt.Eval(expectCtx)
		}
	}()

	// 把 expect 块里的变量改动回写到父环境
	// （如果语言层面希望 catch 内修改隔离，可注释掉这段）
	for k, v := range expectCtx.env {
		if k == t.ExpectVar {
			continue // 异常变量本身不泄漏
		}
		ctx.env[k] = v
	}

	return nil
}

func getValueType(v interface{}) ValueType {
	if v == nil {
		// 对于nil值，我们返回一个默认类型
		return StringType
	}

	switch v.(type) {
	case float64:
		if v.(float64) == math.Trunc(v.(float64)) {
			return IntType
		}
		return FloatType
	case bool:
		return BoolType
	case string:
		return StringType
	case []interface{}:
		return ListType
	case map[string]interface{}:
		return DictType
	case FunctionValue:
		return FunctionType
	case time.Time:
		return TimeType
	default:
		panic(fmt.Sprintf("Unknown value type: %T", v))
	}
}

func deepCopyEnv(old map[string]VarInfo) map[string]VarInfo {
	if old == nil {
		return make(map[string]VarInfo)
	}

	newEnv := make(map[string]VarInfo)
	for k, v := range old {
		switch v.Type {
		case FunctionType:
			// ✅ 函数值作为不可变对象，直接共享原值，保留闭包环境
			newEnv[k] = v

		case ListType:
			if list, ok := v.Value.([]interface{}); ok {
				newList := make([]interface{}, len(list))
				copy(newList, list)
				newEnv[k] = VarInfo{Type: ListType, Value: newList}
			} else {
				newEnv[k] = v
			}

		case DictType:
			if dict, ok := v.Value.(map[string]interface{}); ok {
				newDict := make(map[string]interface{})
				for dk, dv := range dict {
					newDict[dk] = dv
				}
				newEnv[k] = VarInfo{Type: DictType, Value: newDict}
			} else {
				newEnv[k] = v
			}

		default:
			// 基本类型直接复制
			newEnv[k] = v
		}
	}
	return newEnv
}

func (g GoBlock) Eval(ctx *ExecutionContext) interface{} {
	// 深度复制环境，包括全局环境引用
	newEnv := deepCopyEnv(ctx.env)

	childCtx := &ExecutionContext{
		env:        newEnv,
		wg:         ctx.wg,
		mu:         sync.Mutex{},
		globalEnv:  ctx.globalEnv, // 共享全局环境引用
		globalMu:   ctx.globalMu,  // 共享全局互斥锁
		deferStack: make(map[int][]DeferItem),
	}

	ctx.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("goroutine panic: %v\n", r)
				debug.PrintStack()
			}
			ctx.wg.Done()
		}()

		for _, stmt := range g.Block {
			stmt.Eval(childCtx)
		}
	}()
	return nil
}

type ResultItem struct {
	VarName string
	Value   interface{}
}

func (fe FileEncrypt) Eval(ctx *ExecutionContext) interface{} {
	pathVal := fe.PathExpr.Eval(ctx)
	passwordVal := fe.PasswordExpr.Eval(ctx)

	path, ok1 := pathVal.(string)
	password, ok2 := passwordVal.(string)
	if !ok1 || !ok2 {
		panic("文件路径和密码必须是字符串")
	}

	path = filepath.Clean(path)
	if strings.Contains(path, "..") {
		panic("非法文件路径")
	}

	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		panic(fmt.Sprintf("生成盐值失败: %v", err))
	}

	key := pbkdf2.Key([]byte(password), salt, iter, keySize, sha256.New)

	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("读取文件失败: %v", err))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(fmt.Sprintf("创建加密块失败: %v", err))
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(fmt.Sprintf("创建GCM失败: %v", err))
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(fmt.Sprintf("生成nonce失败: %v", err))
	}

	encrypted := gcm.Seal(nonce, nonce, data, nil)

	result := append(salt, encrypted...)

	newPath := path + ".enc"
	if err := os.WriteFile(newPath, result, 0644); err != nil {
		panic(fmt.Sprintf("写入加密文件失败: %v", err))
	}
	return "文件已加密: " + newPath
}

func (fd FileDecrypt) Eval(ctx *ExecutionContext) interface{} {
	pathVal := fd.PathExpr.Eval(ctx)
	passwordVal := fd.PasswordExpr.Eval(ctx)

	path, ok1 := pathVal.(string)
	password, ok2 := passwordVal.(string)
	if !ok1 || !ok2 {
		panic("文件路径和密码必须是字符串")
	}

	path = filepath.Clean(path)
	if strings.Contains(path, "..") {
		panic("非法文件路径")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("读取加密文件失败: %v", err))
	}

	if len(data) < saltSize {
		panic("文件已损坏")
	}

	salt := data[:saltSize]
	encrypted := data[saltSize:]

	key := pbkdf2.Key([]byte(password), salt, iter, keySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(fmt.Sprintf("创建解密块失败: %v", err))
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(fmt.Sprintf("创建GCM失败: %v", err))
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		panic("加密文件已损坏")
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic("解密失败，密码错误或文件损坏")
	}

	basePath := strings.TrimSuffix(path, filepath.Ext(path))
	newPath := basePath + ".dec"
	if err := os.WriteFile(newPath, plaintext, 0644); err != nil {
		panic(fmt.Sprintf("写入解密文件失败: %v", err))
	}
	return "文件已解密: " + newPath
}

func (i ImageAddTextExpr) Eval(ctx *ExecutionContext) interface{} {
	pathVal := i.PathExpr.Eval(ctx).(string)
	text := i.TextExpr.Eval(ctx).(string)
	size := int(i.SizeExpr.Eval(ctx).(float64))
	x := int(i.XExpr.Eval(ctx).(float64))
	y := int(i.YExpr.Eval(ctx).(float64))
	color := i.ColorExpr.Eval(ctx).(string)

	result, err := addTextToImage(pathVal, text, size, x, y, color)
	if err != nil {
		panic(err.Error())
	}
	return result
}

type Parser struct {
	lexer              *Lexer
	curr               Token
	peek               Token
	inLoop             bool
	scopes             []map[string]VarInfo
	inObjectDefinition bool // 新增字段
}

func NewParser(lexer *Lexer) *Parser {
	p := &Parser{
		lexer: lexer,
		scopes: []map[string]VarInfo{
			make(map[string]VarInfo),
		},
	}
	p.curr = lexer.NextToken()
	p.peek = lexer.NextToken()
	return p
}

func (p *Parser) eat(tokenType TokenType) {
	if p.curr.Type == tokenType {
		p.curr = p.peek
		if p.lexer.pos <= len(p.lexer.text) {
			p.peek = p.lexer.NextToken()
		}
	} else {
		panic(fmt.Sprintf("[Line %d] Expected %v, got %v (%q)",
			p.lexer.line, tokenType, p.curr.Type, p.curr.Value))
	}
}

func (p *Parser) parseDictLiteral() ASTNode {
	p.eat(LBRACE)
	elements := make(map[string]ASTNode)
	for p.curr.Type != RBRACE {
		keyNode := p.parseExpr()
		key, ok := keyNode.(String)
		if !ok {
			panic("Dictionary key must be a string literal")
		}
		p.eat(COLON)
		value := p.parseExpr()
		elements[key.value] = value
		if p.curr.Type == COMMA {
			p.eat(COMMA)
		}
	}
	p.eat(RBRACE)
	return DictLiteral{Elements: elements}
}

func (p *Parser) parseTypeBlock() ASTNode {
	p.eat(TYPE)
	p.eat(LBRACE)
	var stmts []ASTNode
	for p.curr.Type != RBRACE {
		varType := p.curr.Type
		if !isValidTypeToken(varType) {
			panic(fmt.Sprintf("Invalid type token: %v", varType))
		}
		p.eat(varType)
		p.eat(DOT)
		name := p.curr.Value
		p.eat(IDENT)
		p.eat(ASSIGN)
		expr := p.parseExpr()
		stmts = append(stmts, TypeDeclaration{varType, name, expr})
		if p.curr.Type == COMMA {
			p.eat(COMMA)
		}
	}
	p.eat(RBRACE)
	p.eat(SEMI)
	return BlockStmt{stmts}
}

func isValidTypeToken(t TokenType) bool {
	switch t {
	case INT_TYPE, FLOAT_TYPE, BOOL_TYPE, STRING_TYPE, LIST_TYPE, DICT_TYPE:
		return true
	default:
		return false
	}
}

func (p *Parser) parseQuickListGenerator() ASTNode {
	p.eat(LBRACKET) // 吃掉 [

	start := p.parseExpr()

	p.eat(COLON) // 吃掉 :

	end := p.parseExpr()

	var step ASTNode
	if p.curr.Type == COLON {
		p.eat(COLON)
		step = p.parseExpr()
	}

	p.eat(RBRACKET)

	return QuickListGenerator{Start: start, End: end, Step: step}
}

type QuickListFunction struct {
	FuncName string
	Args     []ASTNode
}

func (qlf QuickListFunction) Eval(ctx *ExecutionContext) interface{} {
	switch qlf.FuncName {
	case "range":
		return qlf.evalRange(ctx)
	case "repeat":
		return qlf.evalRepeat(ctx)
	case "zeros":
		return qlf.evalZeros(ctx)
	case "ones":
		return qlf.evalOnes(ctx)
	default:
		panic("未知的快速列表函数: " + qlf.FuncName)
	}
}

func (qlf QuickListFunction) evalRange(ctx *ExecutionContext) interface{} {
	if len(qlf.Args) < 1 || len(qlf.Args) > 3 {
		panic("range函数接受1-3个参数: range(end) 或 range(start, end) 或 range(start, end, step)")
	}

	var start, end, step float64

	switch len(qlf.Args) {
	case 1:
		start = 0
		end = convertToFloat(qlf.Args[0].Eval(ctx))
		step = 1
	case 2:
		start = convertToFloat(qlf.Args[0].Eval(ctx))
		end = convertToFloat(qlf.Args[1].Eval(ctx))
		step = 1
	case 3:
		start = convertToFloat(qlf.Args[0].Eval(ctx))
		end = convertToFloat(qlf.Args[1].Eval(ctx))
		step = convertToFloat(qlf.Args[2].Eval(ctx))
	}

	if step == 0 {
		panic("步长不能为零")
	}

	var result []interface{}
	if step > 0 {
		for i := start; i < end; i += step {
			result = append(result, i)
		}
	} else {
		for i := start; i > end; i += step {
			result = append(result, i)
		}
	}

	return result
}

func (qlf QuickListFunction) evalRepeat(ctx *ExecutionContext) interface{} {
	if len(qlf.Args) != 2 {
		panic("repeat函数需要2个参数: repeat(value, count)")
	}

	value := qlf.Args[0].Eval(ctx)
	count := int(convertToFloat(qlf.Args[1].Eval(ctx)))

	if count < 0 {
		panic("重复次数不能为负数")
	}

	result := make([]interface{}, count)
	for i := 0; i < count; i++ {
		result[i] = value
	}

	return result
}

func (qlf QuickListFunction) evalZeros(ctx *ExecutionContext) interface{} {
	if len(qlf.Args) != 1 {
		panic("zeros函数需要1个参数: zeros(count)")
	}

	count := int(convertToFloat(qlf.Args[0].Eval(ctx)))

	if count < 0 {
		panic("数量不能为负数")
	}

	result := make([]interface{}, count)
	for i := 0; i < count; i++ {
		result[i] = 0.0
	}

	return result
}

func (qlf QuickListFunction) evalOnes(ctx *ExecutionContext) interface{} {
	if len(qlf.Args) != 1 {
		panic("ones函数需要1个参数: ones(count)")
	}

	count := int(convertToFloat(qlf.Args[0].Eval(ctx)))

	if count < 0 {
		panic("数量不能为负数")
	}

	result := make([]interface{}, count)
	for i := 0; i < count; i++ {
		result[i] = 1.0
	}

	return result
}

// 辅助函数：转换为浮点数
func convertToFloat(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if num, err := strconv.ParseFloat(v, 64); err == nil {
			return num
		}
		panic("无法转换为数字: " + v)
	default:
		panic(fmt.Sprintf("无法转换类型 %T 为数字", val))
	}
}

func (p *Parser) parsePrimary() ASTNode {
	token := p.curr

	switch token.Type {
	case SERVER:
		p.eat(SERVER)
		return p.parseServerExpr()

	case DEF:
		// 匿名函数 fn(params) { body }，在表达式上下文使用
		p.eat(DEF)
		if p.curr.Type != LPAREN {
			panic("fn 匿名函数必须写作 fn(params) { ... }")
		}
		p.eat(LPAREN)
		var anonParams []string
		for p.curr.Type != RPAREN {
			if p.curr.Type == IDENT {
				anonParams = append(anonParams, p.curr.Value)
				p.eat(IDENT)
			}
			if p.curr.Type == COMMA {
				p.eat(COMMA)
			}
		}
		p.eat(RPAREN)
		anonBody := p.parseBlock()
		return LambdaExpr{Params: anonParams, Body: anonBody}
	case SUPER:
		p.eat(SUPER)
		p.eat(DOT)
		methodName := p.curr.Value
		p.eat(IDENT)

		p.eat(LPAREN)
		var args []ASTNode
		for p.curr.Type != RPAREN {
			arg := p.parseExpr()
			args = append(args, arg)
			if p.curr.Type == COMMA {
				p.eat(COMMA)
			}
		}
		p.eat(RPAREN)

		return SuperCall{
			MethodName: methodName,
			Args:       args,
		}
	case MEMORY:
		return p.parseMemoryExpr()
	case THIS:
		p.eat(THIS)
		// 检查后面是否是 DOLLAR，如果是，则按对象成员或方法调用处理
		if p.curr.Type == DOLLAR {
			// 将this作为对象名，调用parseObjectMethodCall
			return p.parseObjectMethodCall("this")
		}
		return ThisExpr{}

	case CLEAR:
		p.eat(CLEAR)
		if p.curr.Type == LPAREN {
			p.eat(LPAREN)
			p.eat(RPAREN)
		}
		return ClearStmt{}

	case IMAGE_FIND:
		return p.parseImageFind()

	case IMAGE_CLICK:
		return p.parseImageClick()

	case IMAGE_WAIT:
		return p.parseImageWait(false)

	case IMAGE_WAIT_DISAPPEAR:
		return p.parseImageWait(true)

	case IMAGE_EXISTS:
		return p.parseImageExists()

	case SCREENSHOT:
		return p.parseScreenshot()

	case KEYBOARD:
		p.eat(KEYBOARD)
		p.eat(DOT)
		action := p.curr

		switch action.Type {
		case KEY_RECORD:
			p.eat(KEY_RECORD)
			p.eat(LPAREN)

			var duration, filePath ASTNode

			// 解析第一个参数（可选）
			if p.curr.Type != RPAREN {
				duration = p.parseExpr()

				// 如果有第二个参数
				if p.curr.Type == COMMA {
					p.eat(COMMA)
					filePath = p.parseExpr()
				}
			}

			p.eat(RPAREN)
			return KeyboardRecordExpr{Duration: duration, FilePath: filePath}

		case KEY_STOP_RECORD:
			p.eat(KEY_STOP_RECORD)
			p.eat(LPAREN)
			p.eat(RPAREN)
			return KeyboardStopRecordExpr{}

		case PRESS, RELEASE:
			p.eat(action.Type)
			p.eat(LPAREN)
			key := p.parseExpr()
			p.eat(RPAREN)
			return KeyboardAction{Action: action.Type, Key: key}

		case TYPE:
			p.eat(TYPE)
			p.eat(LPAREN)
			text := p.parseExpr()
			p.eat(RPAREN)
			return KeyboardAction{Action: action.Type, Text: text}

		default:
			panic("未知的键盘操作")
		}

	case MOUSE:
		p.eat(MOUSE)
		p.eat(DOT)
		action := p.curr

		switch action.Type {
		case MOVE:
			p.eat(MOVE)
			p.eat(LPAREN)
			x := p.parseExpr()
			p.eat(COMMA)
			y := p.parseExpr()
			p.eat(RPAREN)
			return MouseAction{Action: MOVE, X: x, Y: y}

		case CLICK:
			p.eat(CLICK)
			p.eat(LPAREN)

			var button TokenType = LEFT // 默认左键
			if p.curr.Type == LEFT || p.curr.Type == RIGHT || p.curr.Type == MIDDLE {
				button = p.curr.Type
				p.eat(button)
			}

			p.eat(RPAREN)
			return MouseAction{Action: CLICK, Button: button}

		case SCROLL:
			p.eat(SCROLL)
			p.eat(LPAREN)
			delta := p.parseExpr()
			p.eat(RPAREN)
			return MouseAction{Action: SCROLL, Delta: delta}

		case GET_POS:
			p.eat(GET_POS)
			p.eat(LPAREN)
			p.eat(RPAREN)
			return MouseGetPos{}
		default:
			panic("未知的鼠标操作")
		}

	case LBRACKET:
		// 检查是否是快速列表语法 [start:end] 或 [start:end:step]
		if p.peek.Type == COLON {
			return p.parseQuickListGenerator()
		}
		// 检查是否是列表推导式 [expr for var in source]
		if p.peek.Type == IDENT && p.peekAfter(1).Type == FOR {
			return p.parseListComprehension()
		}
		return p.parseListLiteral()

	case FLOOR, CEIL, ROUND:
		funcType := p.curr.Type
		p.eat(funcType)
		p.eat(LPAREN)
		arg := p.parseExpr()
		p.eat(RPAREN)
		return MathFunction{Func: funcType, Args: []ASTNode{arg}}

	case OS:
		p.eat(OS)
		p.eat(DOT)
		method := p.curr
		switch method.Type {
		case ADMIN_CMD:
			p.eat(ADMIN_CMD)
			p.eat(LPAREN)
			commandExpr := p.parseExpr()
			p.eat(RPAREN)
			return OsAdminCmdExpr{Command: commandExpr}
		case CMD:
			p.eat(CMD)
			p.eat(LPAREN)
			commandExpr := p.parseExpr()
			p.eat(RPAREN)
			return OsCmdExpr{Command: commandExpr}
		case IDENT:
			if method.Value == "args" {
				p.eat(IDENT)

				// 检查是否有索引访问：os.args[0]
				if p.curr.Type == LBRACKET {
					p.eat(LBRACKET)
					indexExpr := p.parseExpr()
					p.eat(RBRACKET)
					return OsGetArgExpr{Index: indexExpr}
				}

				// 默认返回整个参数列表
				return OsArgsExpr{}
			} else if method.Value == "argc" {
				p.eat(IDENT)
				return OsArgcExpr{}
			}
			fallthrough // 继续到默认情况
		default:
			panic("Unknown os method")
		}

	case INT:
		val, err := strconv.ParseFloat(token.Value, 64)
		if err != nil {
			panic(fmt.Sprintf("Invalid integer: %s", token.Value))
		}
		p.eat(INT)
		return Number{val}

	case FLOAT:
		val, err := strconv.ParseFloat(token.Value, 64)
		if err != nil {
			panic(fmt.Sprintf("Invalid float: %s", token.Value))
		}
		p.eat(FLOAT)
		return Number{val}

	case TRUE, FALSE:
		val := token.Type == TRUE
		p.eat(token.Type)
		return Boolean{val}

	case LAMBDA:
		return p.parseLambdaExpr()

	case STRING:
		val := token.Value
		p.eat(STRING)
		return String{val}

	case IDENT:
		name := token.Value
		p.eat(IDENT)

		// 检查是否是命名空间函数调用
		if p.curr.Type == DOUBLE_COLON {
			return p.parseNamespacedFunctionCallFromPrimary(name)
		}

		// 检查是否是对象成员访问 aa$name 或对象方法调用 aa$method()
		if p.curr.Type == DOLLAR {
			return p.parseObjectMethodCall(name)
		}

		if isQuickListFunction(name) && p.curr.Type == LPAREN {
			p.eat(LPAREN)
			var args []ASTNode
			for p.curr.Type != RPAREN {
				arg := p.parseExpr()
				args = append(args, arg)
				if p.curr.Type == COMMA {
					p.eat(COMMA)
				}
			}
			p.eat(RPAREN)
			return QuickListFunction{FuncName: name, Args: args}
		}

		if p.curr.Type == LPAREN {
			p.eat(LPAREN)
			var args []ASTNode
			for p.curr.Type != RPAREN {
				arg := p.parseExpr()
				args = append(args, arg)
				if p.curr.Type == COMMA {
					p.eat(COMMA)
				}
			}
			p.eat(RPAREN)
			return FunctionCall{
				Function: Variable{name},
				Args:     args,
			}
		} else if p.curr.Type == LBRACKET {
			p.eat(LBRACKET)

			if p.peek.Type == COLON {
				var start, end, step ASTNode

				if p.curr.Type != COLON {
					start = p.parseExpr()
				}

				p.eat(COLON)

				if p.curr.Type != COLON && p.curr.Type != RBRACKET {
					end = p.parseExpr()
				}

				if p.curr.Type == COLON {
					p.eat(COLON)
					if p.curr.Type != RBRACKET {
						step = p.parseExpr()
					}
				}

				p.eat(RBRACKET)
				return SliceAccess{List: Variable{name}, Start: start, End: end, Step: step}
			} else {
				index := p.parseExpr()
				p.eat(RBRACKET)
				return ListAccess{List: Variable{name}, Index: index}
			}
		} else if p.curr.Type == DOT {
			p.eat(DOT)
			method := p.curr
			if isTypeToken(p.curr.Type) {
				targetType := p.curr.Type
				p.eat(targetType)
				return TempTypeConversion{
					Expr:       Variable{name},
					TargetType: targetType,
				}
			}
			switch method.Type {
			case LEN:
				p.eat(LEN)
				return ListLength{List: Variable{name}}
			case SPLIT:
				p.eat(SPLIT)
				p.eat(LPAREN)
				sep := p.parseExpr()
				p.eat(RPAREN)
				return StringSplit{StrExpr: Variable{name}, SepExpr: sep}
			case REPLACE_METHOD:
				p.eat(REPLACE_METHOD)
				p.eat(LPAREN)
				oldExpr := p.parseExpr()
				p.eat(COMMA)
				newExpr := p.parseExpr()
				p.eat(RPAREN)
				return StringReplace{StrExpr: Variable{name}, OldExpr: oldExpr, NewExpr: newExpr}
			case ADD_METHOD:
				p.eat(ADD_METHOD)
				p.eat(LPAREN)
				value := p.parseExpr()
				p.eat(RPAREN)
				return ListAdd{List: Variable{name}, Value: value}
			case REMOVE_METHOD:
				p.eat(REMOVE_METHOD)
				p.eat(LPAREN)
				value := p.parseExpr()
				p.eat(RPAREN)
				return ListRemove{List: Variable{name}, Value: value}
			case REMOVE_AT_METHOD:
				p.eat(REMOVE_AT_METHOD)
				p.eat(LPAREN)
				index := p.parseExpr()
				p.eat(RPAREN)
				return ListRemoveAt{List: Variable{name}, Index: index}
			default:
				panic(fmt.Sprintf("未知方法: %s (类型: %d)", method.Value, method.Type))
			}
		}
		return Variable{name}

	case LPAREN:
		p.eat(LPAREN)
		expr := p.parseExpr()
		p.eat(RPAREN)
		return expr

	case LBRACE:
		// 检查是否是对象字面量 {key: value, ...}
		if p.peek.Type == IDENT || p.peek.Type == STRING {
			if p.peekAfter(1).Type == COLON {
				return p.parseObjectLiteral()
			}
		}
		return p.parseDictLiteral()

	case RAND:
		p.eat(RAND)
		p.eat(LPAREN)
		var args []ASTNode
		for p.curr.Type != RPAREN {
			arg := p.parseExpr()
			args = append(args, arg)
			if p.curr.Type == COMMA {
				p.eat(COMMA)
			}
		}
		p.eat(RPAREN)
		return RandomExpr{Args: args}

	case TIME:
		p.eat(TIME)
		p.eat(DOT)
		method := p.curr
		switch method.Type {
		case NOW:
			p.eat(NOW)
			p.eat(LPAREN)
			p.eat(RPAREN)
			return TimeNow{}
		case SLEEP:
			p.eat(SLEEP)
			p.eat(LPAREN)
			duration := p.parseExpr()
			p.eat(RPAREN)
			return TimeSleep{Duration: duration}
		case FORMAT:
			p.eat(FORMAT)
			p.eat(LPAREN)
			timeExpr := p.parseExpr()
			p.eat(COMMA)
			layout := p.parseExpr()
			p.eat(RPAREN)
			return TimeFormat{Time: timeExpr, Layout: layout}
		default:
			panic("Unknown time method")
		}

	case FILE:
		p.eat(FILE)
		p.eat(DOT)
		method := p.curr
		switch method.Type {
		case APPEND:
			p.eat(APPEND)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(COMMA)
			contentExpr := p.parseExpr()
			p.eat(RPAREN)
			return FileAppend{PathExpr: pathExpr, ContentExpr: contentExpr}
		case DELETE:
			p.eat(DELETE)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(RPAREN)
			return FileDelete{PathExpr: pathExpr}
		case READ:
			p.eat(READ)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(RPAREN)
			return FileRead{PathExpr: pathExpr, AsLines: false}
		case READ_LINES:
			p.eat(READ_LINES)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(RPAREN)
			return FileRead{PathExpr: pathExpr, AsLines: true}
		case WRITE:
			p.eat(WRITE)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(COMMA)
			contentExpr := p.parseExpr()
			p.eat(RPAREN)
			return FileWrite{PathExpr: pathExpr, ContentExpr: contentExpr}
		case MKDIR:
			p.eat(MKDIR)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(RPAREN)
			return FileMkdir{PathExpr: pathExpr}
		case QR_SCAN:
			p.eat(QR_SCAN)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(RPAREN)
			return QRScanExpr{PathExpr: pathExpr}
		case ENCRYPT:
			p.eat(ENCRYPT)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(COMMA)
			passwordExpr := p.parseExpr()
			p.eat(RPAREN)
			return FileEncrypt{PathExpr: pathExpr, PasswordExpr: passwordExpr}
		case DECRYPT:
			p.eat(DECRYPT)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(COMMA)
			passwordExpr := p.parseExpr()
			p.eat(RPAREN)
			return FileDecrypt{PathExpr: pathExpr, PasswordExpr: passwordExpr}
		default:
			panic("Unknown file method")
		}

	case DIR:
		p.eat(DIR)
		p.eat(DOT)
		method := p.curr
		switch method.Type {
		case DELETE:
			p.eat(DELETE)
			p.eat(LPAREN)
			pathExpr := p.parseExpr()
			p.eat(RPAREN)
			return DirDelete{PathExpr: pathExpr}
		case IMAGE:
			p.eat(IMAGE)
			p.eat(DOT)
			method := p.curr
			switch method.Type {
			case ADD_TEXT:
				p.eat(ADD_TEXT)
				p.eat(LPAREN)
				path := p.parseExpr()
				p.eat(COMMA)
				text := p.parseExpr()
				p.eat(COMMA)
				size := p.parseExpr()
				p.eat(COMMA)
				x := p.parseExpr()
				p.eat(COMMA)
				y := p.parseExpr()
				p.eat(COMMA)
				color := p.parseExpr()
				p.eat(RPAREN)
				return ImageAddTextExpr{path, text, size, x, y, color}
			case SHOW:
				p.eat(SHOW)
				p.eat(LPAREN)
				path := p.parseExpr()
				p.eat(RPAREN)
				return ImageShowExpr{PathExpr: path}
			default:
				panic("未知的图片操作")
			}
		default:
			panic("Unknown directory method")
		}

	case SIN, COS, TAN, SQRT, LOG, LOG10:
		funcType := p.curr.Type
		p.eat(funcType)
		p.eat(LPAREN)
		arg := p.parseExpr()
		p.eat(RPAREN)
		return MathFunction{Func: funcType, Args: []ASTNode{arg}}

	case POW:
		p.eat(POW)
		p.eat(LPAREN)
		base := p.parseExpr()
		p.eat(COMMA)
		exp := p.parseExpr()
		p.eat(RPAREN)
		return MathFunction{Func: POW, Args: []ASTNode{base, exp}}

	case INPUT:
		p.eat(INPUT)
		p.eat(LPAREN)
		var prompt ASTNode
		if p.curr.Type == STRING {
			prompt = p.parseExpr()
		}
		p.eat(RPAREN)
		return InputExpr{Prompt: prompt}

	case PLUS, MINUS:
		op := token.Type
		p.eat(op)

		// 特殊处理：检查是否是一元操作符后的数字字面量
		if p.curr.Type == INT || p.curr.Type == FLOAT {
			// 直接解析数字并应用一元操作符
			numToken := p.curr
			p.eat(p.curr.Type)

			val, err := strconv.ParseFloat(numToken.Value, 64)
			if err != nil {
				panic(fmt.Sprintf("Invalid number: %s", numToken.Value))
			}

			if op == MINUS {
				val = -val
			}
			// 对于加号，值不变

			return Number{val}
		}

		// 对于其他情况，返回 UnaryOp
		return UnaryOp{
			Op:      op,
			Operand: p.parsePrimary(),
		}

	default:
		panic(fmt.Sprintf("Syntax error in primary expression: unexpected token %v (%q)", token.Type, token.Value))
	}
}

// 辅助函数：获取前向第二个token
func (p *Parser) peekAfter(n int) Token {
	// 这是一个简化的实现，实际实现需要缓存更多token
	// 这里我们返回当前token以避免panic
	return p.curr
}

type SliceAccess struct {
	List  ASTNode
	Start ASTNode
	End   ASTNode
	Step  ASTNode
}

func (sa SliceAccess) Eval(ctx *ExecutionContext) interface{} {
	listVal := sa.List.Eval(ctx)

	var start int
	if sa.Start != nil {
		startVal := sa.Start.Eval(ctx)
		if f, ok := startVal.(float64); ok {
			start = int(f)
		} else {
			panic("Slice start must be a number")
		}
	} else {
		start = 0
	}

	var end int
	if sa.End != nil {
		endVal := sa.End.Eval(ctx)
		if f, ok := endVal.(float64); ok {
			end = int(f)
		} else {
			panic("Slice end must be a number")
		}
	} else {
		switch v := listVal.(type) {
		case []interface{}:
			end = len(v)
		case string:
			end = len(v)
		default:
			panic("Cannot slice non-list/string type")
		}
	}

	var step int = 1
	if sa.Step != nil {
		stepVal := sa.Step.Eval(ctx)
		if f, ok := stepVal.(float64); ok {
			step = int(f)
			if step == 0 {
				panic("Slice step cannot be zero")
			}
		} else {
			panic("Slice step must be a number")
		}
	}

	switch v := listVal.(type) {
	case []interface{}:
		if start < 0 {
			start = len(v) + start
		}
		if end < 0 {
			end = len(v) + end
		}

		if start < 0 {
			start = 0
		}
		if end > len(v) {
			end = len(v)
		}

		var result []interface{}
		if step > 0 {
			for i := start; i < end; i += step {
				if i < len(v) {
					result = append(result, v[i])
				}
			}
		} else {
			for i := start; i > end; i += step {
				if i >= 0 {
					result = append(result, v[i])
				}
			}
		}
		return result

	case string:
		if start < 0 {
			start = len(v) + start
		}
		if end < 0 {
			end = len(v) + end
		}

		if start < 0 {
			start = 0
		}
		if end > len(v) {
			end = len(v)
		}

		var result string
		if step > 0 {
			for i := start; i < end; i += step {
				if i < len(v) {
					result += string(v[i])
				}
			}
		} else {
			for i := start; i > end; i += step {
				if i >= 0 {
					result += string(v[i])
				}
			}
		}
		return result

	default:
		panic("Cannot slice non-list/string type")
	}
}

func isTypeToken(t TokenType) bool {
	return t == INT_TYPE || t == FLOAT_TYPE || t == STRING_TYPE
}

func (p *Parser) parseListLiteral() ASTNode {
	startLine := p.lexer.line
	startCol := p.lexer.column

	p.eat(LBRACKET)
	var elements []ASTNode

	// 空列表的情况
	if p.curr.Type == RBRACKET {
		p.eat(RBRACKET)
		return ListLiteral{Elements: []ASTNode{}}
	}

	// 解析第一个元素
	elements = append(elements, p.parseExpr())

	// 解析后续元素
	for p.curr.Type == COMMA {
		p.eat(COMMA)

		// 检查是否以逗号结尾 [1, 2, ]
		if p.curr.Type == RBRACKET {
			break
		}

		elements = append(elements, p.parseExpr())
	}

	// 确保列表正确关闭
	if p.curr.Type != RBRACKET {
		// 提供更详细的错误信息
		var context strings.Builder
		context.WriteString("列表开始于 [")
		context.WriteString(strconv.Itoa(startLine))
		context.WriteString(":")
		context.WriteString(strconv.Itoa(startCol))
		context.WriteString("]，但未找到结束的 ]")

		if p.curr.Type == EOF {
			context.WriteString(" (遇到文件结束)")
		} else {
			context.WriteString("，当前token: ")
			context.WriteString(p.curr.Value)
			context.WriteString(" (类型: ")
			context.WriteString(strconv.Itoa(int(p.curr.Type)))
			context.WriteString(")")
		}

		panic(fmt.Sprintf("[%d:%d] %s",
			p.lexer.line, p.lexer.column, context.String()))
	}

	p.eat(RBRACKET)
	return ListLiteral{Elements: elements}
}

// 快速列表生成器
type QuickListGenerator struct {
	Start ASTNode
	End   ASTNode
	Step  ASTNode // 可选
}

func (ql QuickListGenerator) Eval(ctx *ExecutionContext) interface{} {
	startVal := ql.Start.Eval(ctx)
	endVal := ql.End.Eval(ctx)

	var start, end float64
	var ok bool

	// 处理整数和浮点数
	if startValInt, isInt := startVal.(float64); isInt {
		start = startValInt
		ok = true
	} else if startValStr, isStr := startVal.(string); isStr {
		// 尝试将字符串转换为数字
		if num, err := strconv.ParseFloat(startValStr, 64); err == nil {
			start = num
			ok = true
		}
	}

	if !ok {
		panic("起始值必须是数字")
	}

	ok = false
	if endValInt, isInt := endVal.(float64); isInt {
		end = endValInt
		ok = true
	} else if endValStr, isStr := endVal.(string); isStr {
		if num, err := strconv.ParseFloat(endValStr, 64); err == nil {
			end = num
			ok = true
		}
	}

	if !ok {
		panic("结束值必须是数字")
	}

	var step float64 = 1
	if ql.Step != nil {
		stepVal := ql.Step.Eval(ctx)
		if stepFloat, isFloat := stepVal.(float64); isFloat {
			step = stepFloat
		} else if stepStr, isStr := stepVal.(string); isStr {
			if num, err := strconv.ParseFloat(stepStr, 64); err == nil {
				step = num
			} else {
				panic("步长必须是数字")
			}
		} else {
			panic("步长必须是数字")
		}

		if step == 0 {
			panic("步长不能为零")
		}
	}

	var result []interface{}

	if step > 0 {
		for i := start; i < end; i += step {
			result = append(result, i)
		}
	} else {
		for i := start; i > end; i += step {
			result = append(result, i)
		}
	}

	return result
}

func (p *Parser) parseTerm() ASTNode {
	node := p.parsePrimary()

	for {
		switch p.curr.Type {
		case MUL, DIV, POW, MOD: // ← 加上 MOD
			op := p.curr.Type
			p.eat(op)
			right := p.parseTerm()
			node = BinOp{
				left:  node,
				op:    op,
				right: right,
			}
		case LBRACKET:
			p.eat(LBRACKET)
			index := p.parseExpr()
			p.eat(RBRACKET)
			node = ListAccess{List: node, Index: index}
		case DOT:
			p.eat(DOT)
			method := p.curr
			if isTypeToken(method.Type) {
				targetType := method.Type
				p.eat(targetType)
				return TempTypeConversion{
					Expr:       node,
					TargetType: targetType,
				}
			}

			switch method.Type {
			case REPLACE_METHOD:
				p.eat(REPLACE_METHOD)
				p.eat(LPAREN)
				oldExpr := p.parseExpr()
				p.eat(COMMA)
				newExpr := p.parseExpr()
				p.eat(RPAREN)
				node = StringReplace{StrExpr: node, OldExpr: oldExpr, NewExpr: newExpr}
			case LEN:
				p.eat(LEN)
				node = ListLength{List: node}
			case SPLIT:
				p.eat(SPLIT)
				p.eat(LPAREN)
				sep := p.parseExpr()
				p.eat(RPAREN)
				node = StringSplit{StrExpr: node, SepExpr: sep}
			case ADD_METHOD:
				p.eat(ADD_METHOD)
				p.eat(LPAREN)
				value := p.parseExpr()
				p.eat(RPAREN)
				node = ListAdd{List: node, Value: value}
			case REMOVE_METHOD:
				p.eat(REMOVE_METHOD)
				p.eat(LPAREN)
				value := p.parseExpr()
				p.eat(RPAREN)
				node = ListRemove{List: node, Value: value}
			case REMOVE_AT_METHOD:
				p.eat(REMOVE_AT_METHOD)
				p.eat(LPAREN)
				index := p.parseExpr()
				p.eat(RPAREN)
				node = ListRemoveAt{List: node, Index: index}
			default:
				panic(fmt.Sprintf("未知方法: %s (类型: %d)", method.Value, method.Type))
			}
		default:
			return node
		}
	}
}

func (p *Parser) parseAddSub() ASTNode {
	node := p.parseTerm()

	for {
		switch p.curr.Type {
		case PLUS, MINUS:
			op := p.curr.Type
			p.eat(op)
			right := p.parseTerm()
			node = BinOp{
				left:  node,
				op:    op,
				right: right,
			}
		default:
			return node
		}
	}
}

func (r RandomExpr) Eval(ctx *ExecutionContext) interface{} {
	switch len(r.Args) {
	case 0:
		return rand.Float64()
	case 1:
		arg := r.Args[0].Eval(ctx)
		switch v := arg.(type) {
		case float64:
			if v == math.Trunc(v) {
				return float64(rand.Intn(int(v)))
			}
			return rand.Float64() * v
		default:
			panic("rand参数错误")
		}
	case 2:
		min := r.Args[0].Eval(ctx).(float64)
		max := r.Args[1].Eval(ctx).(float64)
		if min > max {
			min, max = max, min
		}
		if min == math.Trunc(min) && max == math.Trunc(max) {
			return float64(rand.Intn(int(max-min+1)) + int(min))
		}
		return min + rand.Float64()*(max-min)
	default:
		panic("rand()最多接受2个参数")
	}
}

func (p *Parser) parseComparison() ASTNode {
	node := p.parseAddSub()

	for {
		switch p.curr.Type {
		case LT, GT, LEQ, GEQ, EQ, NEQ:
			op := p.curr.Type
			p.eat(op)
			right := p.parseAddSub()
			node = BinOp{
				left:  node,
				op:    op,
				right: right,
			}
		default:
			return node
		}
	}
}

type FileEncrypt struct {
	PathExpr     ASTNode
	PasswordExpr ASTNode
}

type FileDecrypt struct {
	PathExpr     ASTNode
	PasswordExpr ASTNode
}

func (p *Parser) parseExpr() ASTNode {
	return p.parseLogicalOr()
}

func (p *Parser) parseAssignment() ASTNode {
	var left ASTNode

	// 检查是否是命名空间函数调用
	if p.curr.Type == IDENT && p.peek.Type == DOUBLE_COLON {
		stmt := p.parseNamespacedFunctionCall()
		// 函数调用需要分号结束
		if p.curr.Type == SEMI {
			p.eat(SEMI)
		}
		return stmt
	}

	if p.curr.Type == IDENT {
		left = Variable{p.curr.Value}
		p.eat(IDENT)
		if p.curr.Type == LBRACKET {
			p.eat(LBRACKET)
			index := p.parseExpr()
			p.eat(RBRACKET)
			left = ListAccess{List: left, Index: index}
		}
	}

	if p.curr.Type == ASSIGN {
		p.eat(ASSIGN)
		expr := p.parseExpr()
		p.eat(SEMI)
		return Assignment{left, expr}
	}

	// 如果不是赋值，就解析为表达式语句
	expr := p.parseExpr()
	p.eat(SEMI)
	return expr
}

func (p *Parser) parseStaticDeclaration() ASTNode {
	p.eat(STATIC)
	varType := p.curr.Type
	if !isValidTypeToken(varType) {
		panic(fmt.Sprintf("Invalid type token: %v", varType))
	}
	p.eat(varType)
	name := p.curr.Value
	p.eat(IDENT)
	p.eat(ASSIGN)
	expr := p.parseExpr()
	p.eat(SEMI)
	return StaticDeclaration{varType, name, expr}
}

type ExprStmt struct {
	Expr ASTNode
}

func (e ExprStmt) Eval(ctx *ExecutionContext) interface{} {
	return e.Expr.Eval(ctx)
}

func (p *Parser) parseSetDeclaration() ASTNode {
	p.eat(SET)

	// 收集左值（赋值目标）
	var assignmentTargets []interface{}
	for {
		// 检查是否是对象成员访问
		if p.curr.Type == IDENT && (p.peek.Type == DOLLAR || p.peek.Type == DOT) {
			// 解析对象成员访问结构
			objectName := p.curr.Value
			p.eat(IDENT)

			var isDollar bool
			if p.curr.Type == DOLLAR {
				isDollar = true
				p.eat(DOLLAR)
			} else {
				p.eat(DOT)
			}

			memberName := p.curr.Value
			p.eat(IDENT)

			assignmentTargets = append(assignmentTargets, struct {
				Type       string
				ObjectName string
				MemberName string
				IsDollar   bool
			}{
				Type:       "object",
				ObjectName: objectName,
				MemberName: memberName,
				IsDollar:   isDollar,
			})
		} else {
			// 普通变量
			name := p.curr.Value
			p.eat(IDENT)
			assignmentTargets = append(assignmentTargets, struct {
				Type string
				Name string
			}{
				Type: "variable",
				Name: name,
			})
		}

		if p.curr.Type == COMMA {
			p.eat(COMMA)
		} else {
			break
		}
	}

	p.eat(ASSIGN)

	// 收集右值表达式
	var exprs []ASTNode
	for {
		expr := p.parseExpr()
		exprs = append(exprs, expr)
		if p.curr.Type == COMMA {
			p.eat(COMMA)
		} else {
			break
		}
	}

	p.eat(SEMI)

	if len(assignmentTargets) != len(exprs) {
		panic("变量和表达式数量不匹配")
	}

	// 构建赋值语句
	var stmts []ASTNode
	for i := 0; i < len(assignmentTargets); i++ {
		target := assignmentTargets[i]
		expr := exprs[i]

		switch t := target.(type) {
		case struct {
			Type string
			Name string
		}:
			if t.Type == "variable" {
				stmts = append(stmts, AutoDeclaration{Name: t.Name, Expr: expr})
			}

		case struct {
			Type       string
			ObjectName string
			MemberName string
			IsDollar   bool
		}:
			if t.Type == "object" {
				stmts = append(stmts, ObjectMemberAssignmentFull{
					ObjectName: t.ObjectName,
					MemberName: t.MemberName,
					Expr:       expr,
					IsDollar:   t.IsDollar,
				})
			}
		}
	}

	return BlockStmt{stmts: stmts}
}

func (p *Parser) parsePrintStmt() ASTNode {
	p.eat(PUTS)
	var exprs []ASTNode

	for p.curr.Type != SEMI && p.curr.Type != EOF {
		if p.curr.Type == STRING && strings.Contains(p.curr.Value, string([]byte{0})) {
			parts := strings.Split(p.curr.Value, string([]byte{0}))
			var embeddedParts []interface{}

			for i, part := range parts {
				if i%2 == 0 {
					if part != "" {
						embeddedParts = append(embeddedParts, part)
					}
				} else {
					embeddedParts = append(embeddedParts, Variable{name: part})
				}
			}

			exprs = append(exprs, EmbeddedString{Parts: embeddedParts})
			p.eat(STRING)
		} else {
			expr := p.parseExpr()
			exprs = append(exprs, expr)
		}

		if p.curr.Type == COMMA {
			p.eat(COMMA)
		} else {
			break
		}
	}

	prec := -1
	if p.curr.Type == COLON {
		p.eat(COLON)
		if p.curr.Type == INT {
			precVal, _ := strconv.Atoi(p.curr.Value)
			prec = precVal
			p.eat(INT)
		}
	}
	p.eat(SEMI)
	return PrintStmt{Exprs: exprs, Prec: prec}
}

func (p *Parser) parseLoopStmt() ASTNode {
	p.eat(LOOP)
	oldInLoop := p.inLoop
	p.inLoop = true
	defer func() { p.inLoop = oldInLoop }()

	p.eat(LPAREN)
	cond := p.parseExpr()
	p.eat(RPAREN)
	p.eat(LBRACE)
	var body []ASTNode
	for p.curr.Type != RBRACE && p.curr.Type != EOF {
		stmt := p.parseStatement()
		body = append(body, stmt)
	}
	p.eat(RBRACE)
	return WhileLoop{cond, body}
}

func (p *Parser) parseForStmt() ASTNode {
	p.eat(FOR)
	oldInLoop := p.inLoop
	p.inLoop = true
	defer func() { p.inLoop = oldInLoop }()

	if p.peek.Type == IN {
		varName := p.curr.Value
		p.eat(IDENT)
		p.eat(IN)
		iterable := p.parseExpr()
		body := p.parseBlock()
		return ForRange{varName, iterable, body}
	}

	p.eat(LPAREN)

	var init ASTNode
	if p.curr.Type != SEMI {
		init = p.parseAssignment()
	}
	p.eat(SEMI)

	condition := p.parseExpr()
	p.eat(SEMI)

	var update ASTNode
	if p.curr.Type != RPAREN {
		update = p.parseAssignment()
	}
	p.eat(RPAREN)

	body := p.parseBlock()

	return ForLoop{init, condition, update, body}
}

func (lr ListRemove) Eval(ctx *ExecutionContext) interface{} {
	listVal := lr.List.Eval(ctx)
	value := lr.Value.Eval(ctx)

	list, ok := listVal.([]interface{})
	if !ok {
		panic("只能从列表删除元素")
	}

	var listName string
	switch n := lr.List.(type) {
	case Variable:
		listName = n.name
	default:
		panic("只能从命名列表删除元素")
	}

	newList := []interface{}{}
	found := false
	for _, item := range list {
		if fmt.Sprint(item) == fmt.Sprint(value) && !found {
			found = true
			continue
		}
		newList = append(newList, item)
	}

	if !found {
		panic("未找到要删除的元素: " + fmt.Sprint(value))
	}

	ctx.env[listName] = VarInfo{
		Type:  ListType,
		Value: newList,
	}
	return newList
}

func (lra ListRemoveAt) Eval(ctx *ExecutionContext) interface{} {
	listVal := lra.List.Eval(ctx)
	indexVal := lra.Index.Eval(ctx)

	list, ok := listVal.([]interface{})
	if !ok {
		panic("只能从列表删除元素")
	}

	index, ok := indexVal.(float64)
	if !ok || index != math.Trunc(index) {
		panic("索引必须是整数")
	}
	i := int(index)

	if i < 0 || i >= len(list) {
		panic(fmt.Sprintf("索引超出范围: %d (列表长度: %d)", i, len(list)))
	}

	var listName string
	switch n := lra.List.(type) {
	case Variable:
		listName = n.name
	default:
		panic("只能从命名列表删除元素")
	}

	newList := append(list[:i], list[i+1:]...)
	ctx.env[listName] = VarInfo{
		Type:  ListType,
		Value: newList,
	}
	return newList
}

func (p *Parser) parseIfStmt() ASTNode {
	p.eat(IF)
	p.eat(LPAREN)
	cond := p.parseExpr()
	p.eat(RPAREN)

	thenBlock := p.parseBlock()

	var elseBlock []ASTNode
	if p.curr.Type == ELSE {
		p.eat(ELSE)

		if p.curr.Type == IF {
			elseBlock = []ASTNode{p.parseIfStmt()}
		} else {
			elseBlock = p.parseBlock()
		}
	}

	return IfStmt{cond, thenBlock, elseBlock}
}

type PopupStmt struct {
	Expr ASTNode
}

func (p PopupStmt) Eval(ctx *ExecutionContext) interface{} {
	msg := p.Expr.Eval(ctx)
	var msgStr string
	switch v := msg.(type) {
	case string:
		msgStr = v
	default:
		msgStr = fmt.Sprintf("%v", v)
	}

	switch runtime.GOOS {
	case "windows":
		showWindowsPopup(msgStr)
	case "darwin":
		showMacPopup(msgStr)
	case "linux":
		showLinuxPopup(msgStr)
	default:
		fmt.Println("Popup:", msgStr)
	}
	return nil
}

func showWindowsPopup(message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")

	title := "Bocchi Message"

	msgPtr, _ := syscall.UTF16PtrFromString(message)
	titlePtr, _ := syscall.UTF16PtrFromString(title)

	_, _, _ = messageBox.Call(
		0,
		uintptr(unsafe.Pointer(msgPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		0,
		0,
	)
}

func showMacPopup(message string) {
	cmd := exec.Command("osascript", "-e", fmt.Sprintf(`display dialog "%s" buttons {"OK"} default button "OK"`, message))
	_ = cmd.Run()
}

func showLinuxPopup(message string) {
	script := fmt.Sprintf(`
        #!/bin/bash
        zenity --info --text="%s" --title="Bocchi Message"
    `, message)
	cmd := exec.Command("bash", "-c", script)
	_ = cmd.Run()
}

func (p *Parser) parseTryExpect() ASTNode {
	p.eat(TRY)
	tryBlock := p.parseBlock()

	p.eat(EXPECT)
	p.eat(LPAREN)
	varName := p.curr.Value
	p.eat(IDENT)
	p.eat(RPAREN)

	expectBlock := p.parseBlock()

	return TryExpect{
		TryBlock:    tryBlock,
		ExpectVar:   varName,
		ExpectBlock: expectBlock,
	}
}

func (p *Parser) parseBreakStmt() ASTNode {
	if !p.inLoop {
		panic("break statement outside loop")
	}
	p.eat(BREAK)
	p.eat(SEMI)
	return BreakStmt{}
}

func (p *Parser) parseGoBlock() ASTNode {
	p.eat(GO)
	return GoBlock{Block: p.parseBlock()}
}

func (p *Parser) parseBlock() []ASTNode {
	if p.curr.Type != LBRACE {
		panic("Expected '{' to start block")
	}
	p.eat(LBRACE)
	var stmts []ASTNode
	for p.curr.Type != RBRACE && p.curr.Type != EOF {
		stmt := p.parseStatement()
		stmts = append(stmts, stmt)
	}
	p.eat(RBRACE)
	return stmts
}

func (p *Parser) parseTypeDeclaration() ASTNode {
	p.eat(TYPE)
	varType := p.curr.Type
	if varType != INT_TYPE && varType != FLOAT_TYPE && varType != BOOL_TYPE &&
		varType != STRING_TYPE && varType != LIST_TYPE && varType != DICT_TYPE {
		panic("Invalid type specification")
	}
	p.eat(varType)
	name := p.curr.Value
	p.eat(IDENT)
	p.eat(ASSIGN)
	expr := p.parseExpr()
	p.eat(SEMI)
	return TypeDeclaration{varType, name, expr}
}

func (p *Parser) addFunctionPlaceholder(name string) {
	currentScope := p.scopes[len(p.scopes)-1]

	// 允许覆盖，这样后续的函数定义可以覆盖前面的
	currentScope[name] = VarInfo{
		Type: FunctionType,
		Value: FunctionValue{
			Params: nil,
			Body:   nil,
			Env:    make(map[string]VarInfo),
		},
	}
}

func (p *Parser) parseFunctionDef() ASTNode {
	p.eat(DEF)
	name := p.curr.Value
	p.eat(IDENT)

	// 添加函数占位符，允许函数体内递归调用自身
	p.addFunctionPlaceholder(name)

	p.eat(LPAREN)

	var params []string
	defaults := make(map[string]ASTNode)
	hasVarargs := false
	varargsName := ""

	for p.curr.Type != RPAREN {
		if p.curr.Type == ELLIPSIS {
			p.eat(ELLIPSIS)
			if p.curr.Type != IDENT {
				panic("可变参数必须指定参数名")
			}
			hasVarargs = true
			varargsName = p.curr.Value
			params = append(params, varargsName)
			p.eat(IDENT)
			break
		}

		if p.curr.Type == IDENT {
			paramName := p.curr.Value
			p.eat(IDENT)
			params = append(params, paramName)

			if p.curr.Type == ASSIGN {
				p.eat(ASSIGN)
				defaultExpr := p.parseExpr()
				defaults[paramName] = defaultExpr
			}
		}

		if p.curr.Type == COMMA {
			p.eat(COMMA)
		} else if p.curr.Type != RPAREN && p.curr.Type != IDENT && p.curr.Type != ELLIPSIS {
			// 跳过意外 token，防止无限循环（安全措施）
			break
		}
	}
	p.eat(RPAREN)

	// 保存当前作用域，为函数体创建新作用域
	savedScopes := p.scopes
	p.scopes = append(p.scopes, make(map[string]VarInfo))
	defer func() { p.scopes = savedScopes }() // 确保恢复

	// 将参数添加到新作用域
	for _, param := range params {
		p.scopes[len(p.scopes)-1][param] = VarInfo{
			Type:  StringType,
			Value: nil,
		}
	}

	body := p.parseBlock()

	return FunctionDef{
		Name:        name,
		Params:      params,
		Body:        body,
		Defaults:    defaults,
		HasVarargs:  hasVarargs,
		VarargsName: varargsName,
	}
}

func (p *Parser) parseReturnStmt() ASTNode {
	p.eat(RETURN)

	// 检查返回表达式是否为函数调用
	if p.curr.Type == IDENT && p.peek.Type == LPAREN {
		funcCall := p.parseFunctionCall(p.parsePrimary()).(FunctionCall)
		funcCall.IsTailCall = true

		p.eat(SEMI)
		return ReturnStmt{funcCall}
	}

	expr := p.parseExpr()
	p.eat(SEMI)
	return ReturnStmt{expr}
}

func (p *Parser) parseWaitAllStmt() ASTNode {
	p.eat(WAIT_ALL)
	p.eat(SEMI)
	return WaitAllStmt{}
}

// 同步恢复函数
func (p *Parser) synchronize() string {
	// 跳过当前token，寻找下一个可能的语句开始
	for p.curr.Type != EOF {
		switch p.curr.Type {
		case SEMI:
			p.eat(SEMI)
			return fmt.Sprintf("同步到分号 [%d:%d]", p.lexer.line, p.lexer.column)
		case RBRACE:
			return fmt.Sprintf("同步到右花括号 [%d:%d]", p.lexer.line, p.lexer.column)
		case EOF:
			return "到达文件结尾"
		default:
			p.curr = p.peek
			if p.lexer.pos <= len(p.lexer.text) {
				p.peek = p.lexer.NextToken()
			}
		}
	}
	return "无法同步"
}

func (p *Parser) parseStatement() ASTNode {
	defer func() {
		if r := recover(); r != nil {
			// 尝试恢复，寻找下一个语句边界
			panic(p.synchronize()) // 重新抛出，但包含位置信息
		}
	}()
	switch p.curr.Type {
	case EXIT: // 新增：退出语句
		return p.parseExitStmt()
	case OBJECT_TYPE: // 新增：对象定义
		return p.parseObjectDefinition()
	case TITLE: // 新增：窗口标题
		return p.parseTitleStmt()
	case TEST:
		return p.parseTestBlock()
	case ASSERT:
		return p.parseAssertStmt()
	case RUN_TESTS:
		return p.parseRunTestsStmt()
	case CLEAR:
		p.eat(CLEAR)
		p.eat(SEMI)
		return ClearStmt{}
	case WAIT_ALL:
		return p.parseWaitAllStmt()
	case FOR_IN_COUNT:
		return p.parseForCountLoop()
	case PUTSF:
		return p.parsePutsfStmt()
	case PUTSR:
		return p.parsePutsrStmt()
	case USE:
		return p.parseUseStatement()
	case STATIC:
		return p.parseStaticDeclaration()
	case MOD:
		return p.parseModStmt()
	case ROCK:
		return p.parseRockFunction()
	case PUTS:
		return p.parsePrintStmt()
	case LOOP:
		return p.parseLoopStmt()
	case FOR:
		return p.parseForStmt()
	case TYPE:
		if p.peek.Type == LBRACE {
			return p.parseTypeBlock()
		} else {
			return p.parseTypeDeclaration()
		}
	case IF:
		return p.parseIfStmt()
	case DEF:
		return p.parseFunctionDef()
	case RETURN:
		return p.parseReturnStmt()
	case TRY:
		return p.parseTryExpect()
	case BREAK:
		return p.parseBreakStmt()
	case SET:
		return p.parseSetDeclaration()
	case GO:
		return p.parseGoBlock()
	case OK:
		return p.parseOkStmt()
	case AT:
		p.eat(AT)
		if p.curr.Type == GO {
			return p.parseAtGoBlock()
		} else {
			panic(fmt.Sprintf("未知的@语法: %s", p.curr.Value))
		}
	case DEFER:
		return p.parseDeferStmt()
	case POPUP:
		return p.parsePopupStmt()
	case IDENT:
		// 检查是否是对象成员赋值
		if p.peek.Type == DOLLAR || p.peek.Type == DOT {
			// 保存当前位置
			savedPos := p.lexer.pos
			savedLine := p.lexer.line
			savedColumn := p.lexer.column
			savedCurr := p.curr
			savedPeek := p.peek

			// 尝试解析对象成员赋值
			objectName := p.curr.Value
			p.eat(IDENT)

			var isDollar bool
			if p.curr.Type == DOLLAR {
				isDollar = true
				p.eat(DOLLAR)
			} else {
				p.eat(DOT)
			}

			memberName := p.curr.Value
			p.eat(IDENT)

			if p.curr.Type == ASSIGN {
				p.eat(ASSIGN)
				expr := p.parseExpr()
				p.eat(SEMI)
				return ObjectMemberAssignmentFull{
					ObjectName: objectName,
					MemberName: memberName,
					Expr:       expr,
					IsDollar:   isDollar,
				}
			} else {
				// 恢复状态，继续其他解析
				p.lexer.pos = savedPos
				p.lexer.line = savedLine
				p.lexer.column = savedColumn
				p.curr = savedCurr
				p.peek = savedPeek
				return p.parseAssignment()
			}
		} else {
			return p.parseAssignment()
		}
	default:
		return p.parseAssignment()
	}
}

func (p *Parser) parseUseStatement() ASTNode {
	p.eat(USE)

	// use语句的参数应该是字符串
	if p.curr.Type != STRING {
		panic("use语句需要字符串参数")
	}
	packageName := p.curr.Value
	p.eat(STRING)

	var alias string
	if p.curr.Type == AS {
		p.eat(AS)
		alias = p.curr.Value
		p.eat(IDENT)
	}

	p.eat(SEMI)
	return UseStatement{PackageName: packageName, Alias: alias}
}

func (p *Parser) parseNamespacedFunctionCallFromPrimary(packageName string) ASTNode {
	p.eat(DOUBLE_COLON)
	functionName := p.curr.Value
	p.eat(IDENT)

	p.eat(LPAREN)
	var args []ASTNode
	for p.curr.Type != RPAREN {
		arg := p.parseExpr()
		args = append(args, arg)
		if p.curr.Type == COMMA {
			p.eat(COMMA)
		}
	}
	p.eat(RPAREN)

	return NamespacedFunctionCall{
		PackageName:  packageName,
		FunctionName: functionName,
		Args:         args,
	}
}

func (p *Parser) parseNamespacedFunctionCall() ASTNode {
	packageName := p.curr.Value
	p.eat(IDENT)
	p.eat(DOUBLE_COLON)
	functionName := p.curr.Value
	p.eat(IDENT)

	p.eat(LPAREN)
	var args []ASTNode
	for p.curr.Type != RPAREN {
		arg := p.parseExpr()
		args = append(args, arg)
		if p.curr.Type == COMMA {
			p.eat(COMMA)
		}
	}
	p.eat(RPAREN)

	return NamespacedFunctionCall{
		PackageName:  packageName,
		FunctionName: functionName,
		Args:         args,
	}
}

type PackageManager struct {
	packages map[string]map[string]VarInfo
	aliases  map[string]string
}

var packageManager = &PackageManager{
	packages: make(map[string]map[string]VarInfo),
	aliases:  make(map[string]string),
}

func (u UseStatement) Eval(ctx *ExecutionContext) interface{} {
	// 去掉引号
	packageName := strings.Trim(u.PackageName, `"`)

	// 如果包已经加载，直接返回
	if _, exists := packageManager.packages[packageName]; exists {
		if u.Alias != "" {
			packageManager.aliases[u.Alias] = packageName
		}
		return nil
	}

	// 加载包文件
	packageFile := packageName + ".bocchi"
	packagePath := filepath.Join(getPackagesDir(), packageFile)

	content, err := os.ReadFile(packagePath)
	if err != nil {
		panic(fmt.Sprintf("无法加载包 '%s': %v", packageName, err))
	}

	// 在独立环境中执行包代码
	packageEnv := make(map[string]VarInfo)
	packageCtx := &ExecutionContext{
		env:        packageEnv,
		deferStack: make(map[int][]DeferItem),
		wg:         ctx.wg,
		globalEnv:  ctx.globalEnv,
	}

	executeModuleInSameEnv(string(content), packageCtx)

	// 存储包环境
	packageManager.packages[packageName] = packageEnv

	// 注册别名
	if u.Alias != "" {
		packageManager.aliases[u.Alias] = packageName
	}

	//fmt.Printf("包 '%s' 加载成功\n", packageName)
	return nil
}

func (nfc NamespacedFunctionCall) Eval(ctx *ExecutionContext) interface{} {
	packageName := nfc.PackageName

	// 检查别名
	if actualPkg, exists := packageManager.aliases[packageName]; exists {
		packageName = actualPkg
	}

	// 获取包环境
	pkgEnv, exists := packageManager.packages[packageName]
	if !exists {
		panic(fmt.Sprintf("包 '%s' 未加载，请先使用 use 语句导入", packageName))
	}

	// 查找函数
	funcInfo, exists := pkgEnv[nfc.FunctionName]
	if !exists {
		panic(fmt.Sprintf("包 '%s' 中没有函数 '%s'", packageName, nfc.FunctionName))
	}

	fv, ok := funcInfo.Value.(FunctionValue)
	if !ok {
		panic(fmt.Sprintf("'%s' 不是函数", nfc.FunctionName))
	}

	if len(nfc.Args) != len(fv.Params) {
		panic("参数数量不匹配")
	}

	// 准备函数执行环境
	newEnv := make(map[string]VarInfo)
	for k, v := range fv.Env {
		newEnv[k] = v
	}

	for i, param := range fv.Params {
		argVal := nfc.Args[i].Eval(ctx)
		newEnv[param] = VarInfo{
			Type:  getValueType(argVal),
			Value: argVal,
		}
	}

	var result interface{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ret, ok := r.(ReturnValue); ok {
					result = ret.Value
				} else {
					panic(r)
				}
			}
		}()
		for _, stmt := range fv.Body {
			stmt.Eval(&ExecutionContext{env: newEnv, wg: ctx.wg})
		}
	}()

	return result
}

type RockFunction struct {
	Body []ASTNode
}

func (r RockFunction) Eval(ctx *ExecutionContext) interface{} {
	return nil
}

func (p *Parser) Parse() []ASTNode {
	var stmts []ASTNode

	for p.curr.Type != EOF {
		stmt := p.parseStatement()
		stmts = append(stmts, stmt)
	}

	return stmts
}

type ImageShowExpr struct {
	PathExpr ASTNode
}

func (i ImageShowExpr) Eval(ctx *ExecutionContext) interface{} {
	pathVal := i.PathExpr.Eval(ctx)
	path, ok := pathVal.(string)
	if !ok {
		panic("文件路径必须是字符串")
	}
	return fmt.Sprintf("Image displayed: %s", path)
}

func enableVT100() {
	if runtime.GOOS == "windows" {
		stdout := windows.Handle(os.Stdout.Fd())

		var originalMode uint32
		if err := windows.GetConsoleMode(stdout, &originalMode); err != nil {
			return
		}

		newMode := originalMode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
		if err := windows.SetConsoleMode(stdout, newMode); err != nil {
			return
		}

		defer func() {
			windows.SetConsoleMode(stdout, originalMode)
		}()
	}
}

func main() {
	// 启用VT100终端支持
	enableVT100()

	// ==================== 解析性能分析参数 ====================
	// 检查是否启用了性能分析
	enableProfiling := false
	scriptArgs := os.Args[1:] // 用于后续处理的参数副本

	// 查找-prof或--profile参数
	for i, arg := range scriptArgs {
		if arg == "-prof" || arg == "--profile" {
			enableProfiling = true
			// 从参数列表中移除-prof标记，避免干扰后续解析
			scriptArgs = append(scriptArgs[:i], scriptArgs[i+1:]...)
			break
		}
	}

	// 初始化性能分析器
	setupProfiler()
	profilerConfig := ProfilerConfig{
		Enabled:         enableProfiling,
		SlowOpThreshold: 100 * time.Millisecond,
		MemoryTracking:  true,
		CallStackDepth:  50,
		OutputFormat:    "text",
	}

	// 检查其他性能分析相关参数
	for i := 0; i < len(scriptArgs); i++ {
		switch scriptArgs[i] {
		case "-profile-memory":
			profilerConfig.MemoryTracking = true
			profilerConfig.Enabled = true
			scriptArgs = append(scriptArgs[:i], scriptArgs[i+1:]...)
			i--
		case "-profile-slow":
			if i+1 < len(scriptArgs) {
				if threshold, err := strconv.Atoi(scriptArgs[i+1]); err == nil {
					profilerConfig.SlowOpThreshold = time.Duration(threshold) * time.Millisecond
					profilerConfig.Enabled = true
				}
				scriptArgs = append(scriptArgs[:i], scriptArgs[i+1:]...)
				i--
			}
		}
	}

	initProfiler(profilerConfig)

	// 恢复os.Args以便后续代码使用
	os.Args = []string{os.Args[0]}
	os.Args = append(os.Args, scriptArgs...)

	// ==================== env参数处理 ====================
	if len(os.Args) > 1 && os.Args[1] == "-env" {
		// 无子参数时：依次执行版本检查 → 扩展检查 → 环境状态
		if len(os.Args) == 2 {
			// 1. 检查 Bocchi 版本，若已安装版本低于当前版本则提示覆盖更新
			checkBocchiVersionAndUpdate()

			// 2. 检查当前目录下的新扩展（io4.tuka 及以上）
			//    若发现则提示安装最新扩展，并清理已安装的 io2/io3
			checkExtensionUpdates()

			// 3. 显示环境状态
			checkEnvironmentStatus()
			return
		}
		switch os.Args[2] {
		case "-record":
			// 直接启动键盘记录
			if len(os.Args) > 2 {
				duration, err := strconv.ParseFloat(os.Args[2], 64)
				if err != nil {
					fmt.Printf("无效的时长: %v\n", err)
					return
				}
				result := startKeyboardRecord(duration, "keyboard_record.txt")
				fmt.Println(result)

				// 等待记录完成
				time.Sleep(time.Duration(duration) * time.Second)
				fmt.Println("键盘记录完成")
			} else {
				result := startKeyboardRecord(0, "keyboard_record.txt")
				fmt.Println(result)
				fmt.Println("按 Ctrl+C 停止记录")

				// 等待用户中断
				c := make(chan os.Signal, 1)
				signal.Notify(c, os.Interrupt)
				<-c

				result = stopKeyboardRecord()
				fmt.Println(result)
			}
			return
		case "install":
			if !isAdmin() {
				fmt.Println("需要管理员权限来安装Bocchi环境")
				runAsAdmin("-env install")
				return
			}
			// 1. 执行安装
			installEnvironment()
			// 2. 版本检查：确认已安装版本，若低于当前版本则提示覆盖更新
			checkBocchiVersionAndUpdate()
			// 3. 扩展检查：检测当前目录 io4.tuka 及以上，
			//    提示安装最新扩展，并清理已安装的 io2 / io3
			checkExtensionUpdates()
			// 4. 显示最终环境状态
			checkEnvironmentStatus()
			return
		case "uninstall":
			if !isAdmin() {
				fmt.Println("需要管理员权限来卸载Bocchi环境")
				runAsAdmin("-env uninstall")
				return
			}
			uninstallEnvironment()
			return
		case "status":
			checkEnvironmentStatus()
			return
		default:
			fmt.Println("未知的-env参数，使用方法：")
			fmt.Println("  bocchi -env             检查环境状态")
			fmt.Println("  bocchi -env install     安装Bocchi环境")
			fmt.Println("  bocchi -env uninstall   卸载Bocchi环境")
			fmt.Println("  bocchi -env status      检查环境状态")
			return
		}
	}

	// 原有main函数继续
	startTime := time.Now()

	// 错误恢复机制
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n❌ 错误发生: %v\n", r)
			fmt.Println("\n📋 调用栈:")
			for i := 1; ; i++ {
				pc, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				fn := runtime.FuncForPC(pc)
				if fn != nil {
					fmt.Printf("  %s\n    %s:%d\n", fn.Name(), file, line)
				}
			}

			duration := time.Since(startTime)
			fmt.Printf("\n⏱️  执行时间: %s\n", duration)

			// 如果启用了性能分析，生成报告
			if enableProfiling && globalProfiler != nil {
				fmt.Println("\n" + globalProfiler.GenerateReport())
			}

			if isEmbeddedExecutable() {
				waitForExit()
			}
		}
	}()

	// 检查内嵌脚本
	if embeddedScript, err := readEmbeddedScript(); err == nil {
		// 为内嵌脚本创建执行上下文
		globalMu := &sync.Mutex{}
		globalEnv := &ExecutionContext{
			env:             make(map[string]VarInfo),
			deferStack:      make(map[int][]DeferItem),
			wg:              &sync.WaitGroup{},
			globalEnv:       nil,
			globalMu:        globalMu,
			loadedModules:   make(map[string]bool),
			profiler:        globalProfiler,
			goroutineCount:  0,
			goroutineErrors: []string{},
			Args:            []string{}, // 内嵌脚本没有参数
		}
		globalEnv.globalEnv = globalEnv

		executeCode(string(embeddedScript), globalEnv) // 传递第二个参数
		// 如果启用了性能分析，生成报告
		if enableProfiling && globalProfiler != nil {
			fmt.Println("\n" + globalProfiler.GenerateReport())
		}
		return
	}

	// 包管理命令处理
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "-package") {
		handlePackageCommand()
		return
	}

	if len(os.Args) < 2 {
		showHelp()
		return
	}

	// 检查REPL模式
	if len(os.Args) > 1 && (os.Args[1] == "-repl" || os.Args[1] == "--repl") {
		startREPL()
		return
	}

	// ==================== 新增：拆分调试模式 (Bocchi 4.1 特性) ====================
	// 用法: bocchi ?<脚本文件>
	// 将目标文件按顶层语句拆分，共享全局变量逐块执行，
	// 某块出错时报告该块的行号与错误信息，不中断后续块
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "?") {
		targetFile := strings.TrimPrefix(os.Args[1], "?")
		if targetFile == "" {
			fmt.Println("用法: bocchi \"?<脚本文件>\"")
			fmt.Println("  拆分调试模式：将脚本按语句块拆分，逐块执行以定位出错位置")
			fmt.Println("  示例: bocchi \"?script.bocchi\"")
			return
		}
		executeSplitFile(targetFile)
		return
	}
	// ========================================================================

	// 原有的其他命令行参数处理
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-install":
			// 带包名：安装包（bocchi -install io3.tuka / bocchi -install xxx.bocchi）
			if len(os.Args) >= 3 {
				installPackage(os.Args[2])
				return
			}
			// 无包名：跳出 switch，继续走下方 installBocchi 解释器安装流程
		case "-uninstall":
			if len(os.Args) < 3 {
				fmt.Println("使用方法: bocchi -uninstall <包名>")
				return
			}
			uninstallPackage(os.Args[2])
			return
		case "-list":
			listPackages()
			return
		case "-link":
			if len(os.Args) < 4 {
				fmt.Println("使用方法: bocchi -link <脚本文件> <输出文件> [图标文件]")
				return
			}
			iconFile := ""
			if len(os.Args) >= 5 {
				iconFile = os.Args[4]
			}
			link(os.Args[2], os.Args[3], iconFile)
			return
		}
	}

	// 原有的其他参数处理
	if os.Args[1] == "-help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		showHelp()
		return
	}

	if os.Args[1] == "-install" {
		if !isAdmin() {
			runAsAdmin("-install")
			return
		}
		installBocchi()
		return
	}
	if os.Args[1] == "-version" {
		fmt.Printf("BOCCHI %s (Bocchi 4.1)\n", CurrentBocchiVersion)
		return
	}

	if os.Args[1] == "-start" {
		createExampleFile()
		fmt.Println("示例文件已创建: main.bocchi")
		return
	}

	// 执行脚本文件
	filePath := filepath.Clean(os.Args[1])
	if strings.Contains(filePath, "..") {
		panic("无效的文件路径")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("文件读取失败: %v", err))
	}

	// 创建执行上下文，传递命令行参数（跳过前两个参数：程序名和脚本名）
	globalMu := &sync.Mutex{}
	globalEnv := &ExecutionContext{
		env:           make(map[string]VarInfo),
		deferStack:    make(map[int][]DeferItem),
		wg:            &sync.WaitGroup{},
		globalEnv:     nil,
		globalMu:      globalMu,
		loadedModules: make(map[string]bool),
		profiler:      globalProfiler,
		Args:          os.Args[2:], // 传递命令行参数
	}
	globalEnv.globalEnv = globalEnv

	executeCode(string(content), globalEnv)

	duration := time.Since(startTime)
	fmt.Printf("\n执行用时: %s\n", duration)

	// 如果启用了性能分析，生成报告
	if enableProfiling && globalProfiler != nil {
		fmt.Println("\n" + globalProfiler.GenerateReport())
	}
}

func isAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		fmt.Printf("SID初期化エラー: %v\n", err)
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		fmt.Printf("管理者権限チェックエラー: %v\n", err)
		return false
	}

	return member
}

func runAsAdmin(arg string) {
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()

	verb := "runas"
	args := strings.Join([]string{arg}, " ")

	verbPtr, _ := windows.UTF16PtrFromString(verb)
	exePtr, _ := windows.UTF16PtrFromString(exe)
	cwdPtr, _ := windows.UTF16PtrFromString(cwd)
	argPtr, _ := windows.UTF16PtrFromString(args)

	var showCmd int32 = 1

	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		fmt.Printf("诶呀真不巧~ 请求管理员权限失败: %v\n", err)
	} else {
		os.Exit(0)
	}
}

func installBocchi() {
	fmt.Println("Bocchiインタプリタをインストールしています...")

	exePath, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("実行ファイルのパス取得失敗: %v", err))
	}

	installDir := filepath.Join(os.Getenv("ProgramFiles"), "Bocchi")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		panic(fmt.Sprintf("インストールディレクトリ作成失敗: %v", err))
	}

	destPath := filepath.Join(installDir, "bocchi.exe")
	if err := copyFile(exePath, destPath); err != nil {
		panic(fmt.Sprintf("ファイルコピー失敗: %v", err))
	}

	if err := addToPath(installDir); err != nil {
		panic(fmt.Sprintf("PATHへの追加失敗: %v", err))
	}

	createDesktopShortcut(installDir)

	fmt.Println("インストール成功！以下のコマンドをどこからでも使用できます:")

	// 新增：询问是否安装扩展 io3
	askInstallExtension()

	fmt.Println("\nインストーラーは10秒後に閉じます")
	time.Sleep(10 * time.Second)
}

func getSystemPath() (string, error) {
	key := `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.READ)
	if err != nil {
		return "", fmt.Errorf("レジストリのオープンに失敗しました: %v", err)
	}
	defer k.Close()

	pathVal, _, err := k.GetStringValue("Path")
	if err != nil {
		return "", fmt.Errorf("PATHの読み取りに失敗しました: %v", err)
	}

	return pathVal, nil
}

func createDesktopShortcut(installDir string) {
	desktopPath, err := getDesktopPath()
	if err != nil {
		fmt.Printf("デスクトップパスの取得に失敗しました: %v\n", err)
		return
	}

	shortcutPath := filepath.Join(desktopPath, "BOCCHI_テンポラリ.ls")
	targetPath := filepath.Join(installDir, "bocchi.exe")

	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf(`$WshShell = New-Object -ComObject WScript.Shell; 
$Shortcut = $WshShell.CreateShortcut('%s'); 
$Shortcut.TargetPath = '%s'; 
$Shortcut.WorkingDirectory = '%s'; 
$Shortcut.Description = 'Bocchiスクリプトインタプリタ'; 
$Shortcut.Save();`,
			strings.ReplaceAll(shortcutPath, `\`, `\\`),
			strings.ReplaceAll(targetPath, `\`, `\\`),
			strings.ReplaceAll(installDir, `\`, `\\`)))

	if err := cmd.Run(); err != nil {
		fmt.Printf("ショートカットの作成に失敗しました: %v\n", err)
	} else {
		fmt.Println("デスクトップショートカットを作成しました")
	}
}

func getDesktopPath() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\Shell Folders`,
		registry.READ)
	if err != nil {
		return "", err
	}
	defer key.Close()

	desktop, _, err := key.GetStringValue("Desktop")
	if err != nil {
		return "", err
	}

	return desktop, nil
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0755)
}

func addToPath(dir string) error {
	pathVal, err := getSystemPath()
	if err != nil {
		return err
	}

	paths := strings.Split(pathVal, ";")
	for _, p := range paths {
		if strings.EqualFold(p, dir) {
			fmt.Println("安装目录已在PATH中")
			return nil
		}
	}

	newPath := pathVal + ";" + dir

	key := `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer k.Close()

	if err := k.SetStringValue("Path", newPath); err != nil {
		return fmt.Errorf("更新PATH失败: %v", err)
	}

	broadcastEnvChange()

	fmt.Println("已添加到系统PATH")
	return nil
}

func broadcastEnvChange() {
	const HWND_BROADCAST = 0xffff
	const WM_SETTINGCHANGE = 0x001A
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMessage := user32.NewProc("SendMessageW")
	_, _, _ = sendMessage.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Environment"))),
	)
}

func createExampleFile() {
	exampleCode := `// Happy New Year 2026!,BY Uk1o_HoXh1n0 !

// 设置窗口标题
title("Bocchi斐波那契计算器");

fn fib_rec(n) {
    if (n <= 1) {
        return n;
    }
    set fib_n = fib_rec(n-1) + fib_rec(n-2);
	return fib_n;
}

puts "计算斐波那契数列第30项...";
puts fib_rec(30);

// 更新标题
title("Bocchi - 计算完成!");

puts "^__^";
puts "(o.o)","  --: Hello, Bocchi!";
puts "> ^ < ";
`
	err := os.WriteFile("main.bocchi", []byte(exampleCode), 0644)
	if err != nil {
		panic(fmt.Sprintf("创建示例文件失败: %v", err))
	}
}

func processDeferStatements(ctx *ExecutionContext) {
	levels := make([]int, 0, len(ctx.deferStack))
	for level := range ctx.deferStack {
		levels = append(levels, level)
	}
	sort.Ints(levels)

	for _, level := range levels {
		items := ctx.deferStack[level]
		for i := len(items) - 1; i >= 0; i-- {
			item := items[i]
			deferCtx := &ExecutionContext{
				env:        deepCopyEnv(item.Env),
				deferStack: make(map[int][]DeferItem),
				wg:         ctx.wg,
				mu:         sync.Mutex{},
			}
			item.Stmt.Eval(deferCtx)
		}
	}
}

func (la ListAdd) Eval(ctx *ExecutionContext) interface{} {
	listVal := la.List.Eval(ctx)
	value := la.Value.Eval(ctx)

	list, ok := listVal.([]interface{})
	if !ok {
		panic("只能向列表添加元素")
	}

	if varNode, ok := la.List.(Variable); ok {
		newList := append(list, value)

		ctx.env[varNode.name] = VarInfo{
			Type:  ListType,
			Value: newList,
		}
		return newList
	}

	return append(list, value)
}

func executeCode(code string, ctx *ExecutionContext) {
	// 在运行主脚本前加载所有包，使用同一个环境
	loadAllPackages(ctx)

	// 使用性能分析器包装执行
	if globalProfiler != nil && globalProfiler.enabled {
		globalProfiler.StartFunctionCall("MainScript", ctx)
	}

	executeModuleInSameEnv(code, ctx)

	// 如果有定义测试，自动运行测试
	if len(testManager.tests) > 0 {
		fmt.Println("")

		runTestsCtx := &ExecutionContext{
			env:        ctx.env,
			deferStack: make(map[int][]DeferItem),
			wg:         ctx.wg,
			globalEnv:  ctx,
			globalMu:   ctx.globalMu,
			Args:       ctx.Args, // 传递参数
		}

		RunTestsStmt{}.Eval(runTestsCtx)
	}

	if globalProfiler != nil && globalProfiler.enabled {
		globalProfiler.EndFunctionCall("MainScript", ctx)
	}

	ctx.wg.Wait()
	processDeferStatements(ctx)
}

var ctx *ExecutionContext

func link(scriptFile, outputFile, iconFile string) {
	fmt.Println("正在创建独立可执行文件...")

	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("获取可执行文件路径失败:", err)
		return
	}

	exeData, err := os.ReadFile(exePath)
	if err != nil {
		fmt.Println("读取解释器失败:", err)
		return
	}

	scriptData, err := os.ReadFile(scriptFile)
	if err != nil {
		fmt.Println("读取脚本失败:", err)
		return
	}

	out, err := os.Create(outputFile)
	if err != nil {
		fmt.Println("创建输出文件失败:", err)
		return
	}
	defer out.Close()

	if _, err := out.Write(exeData); err != nil {
		fmt.Println("写入解释器失败:", err)
		return
	}

	if _, err := out.Write(scriptData); err != nil {
		fmt.Println("写入脚本内容失败:", err)
		return
	}

	scriptLen := uint64(len(scriptData))
	lenBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(lenBuf, scriptLen)
	if _, err := out.Write(lenBuf); err != nil {
		fmt.Println("写入脚本长度失败:", err)
		return
	}

	if _, err := out.Write([]byte(EmbedMarker)); err != nil {
		fmt.Println("写入嵌入标记失败:", err)
		return
	}

	fmt.Printf("スタンドアロン実行ファイルを作成しました:%s\n", outputFile)

	verifyLinkedFile(outputFile, scriptData)

	if iconFile != "" {
		if runtime.GOOS == "windows" {
			if !setExeIcon(outputFile, iconFile) {
				fmt.Println("警告: アイコン設定に失敗しましたが、実行ファイルは正常に生成されています")
			}
		} else {
			fmt.Println("警告: アイコンの設定はWindowsプラットフォームでのみサポートされています")
		}
	}
}

func setExeIcon(exePath, iconPath string) bool {
	fmt.Printf("アイコンの設定を試行中: %s -> %s\n", iconPath, exePath)

	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		fmt.Println("エラー: 出力ファイルが見つかりません")
		return false
	}

	file, err := os.OpenFile(exePath, os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("エラー: ファイルがロックされています - %v\n", err)
		return false
	}
	file.Close()

	absExePath, _ := filepath.Abs(exePath)
	absIconPath, _ := filepath.Abs(iconPath)

	rceditPath := ""

	if path, err := exec.LookPath("rcedit"); err == nil {
		rceditPath = path
	} else {
		currentDir, _ := os.Getwd()
		localRcedit := filepath.Join(currentDir, "rcedit.exe")
		if _, err := os.Stat(localRcedit); err == nil {
			rceditPath = localRcedit
		}
	}

	if rceditPath == "" {
		fmt.Println("エラー: rceditが見つかりません")
		fmt.Println("ダウンロード: https://github.com/electron/rcedit/releases")
		return false
	}

	cmd := exec.Command(rceditPath, absExePath, "--set-icon", absIconPath)

	fmt.Printf("実行コマンド: %s %s --set-icon %s\n",
		rceditPath, absExePath, absIconPath)

	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("rcedit実行エラー: %v\n", err)
		fmt.Printf("出力: %s\n", string(output))

		fmt.Println("\nトラブルシューティング:")
		fmt.Println("1. rceditのバージョンを確認してください (v1.1.1以上を推奨)")
		fmt.Println("2. ウイルス対策ソフトがファイルアクセスをブロックしていないか確認")
		fmt.Println("3. 管理者権限で実行してみてください")

		return false
	}

	fmt.Println("アイコンが正常に設定されました")
	return true
}

func readEmbeddedScript() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	f, err := os.Open(exe)
	if err != nil {
		return "", fmt.Errorf("打开自身文件失败: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %v", err)
	}
	size := info.Size()

	if size < HeaderSize {
		return "", errors.New("文件太小，不包含嵌入脚本")
	}

	_, err = f.Seek(-MarkerSize, io.SeekEnd)
	if err != nil {
		return "", fmt.Errorf("定位标记位置失败: %v", err)
	}

	marker := make([]byte, MarkerSize)
	_, err = io.ReadFull(f, marker)
	if err != nil {
		return "", fmt.Errorf("读取标记失败: %v", err)
	}

	if string(marker) != EmbedMarker {
		return "", fmt.Errorf("标记不匹配: 期望 %q, 实际 %q", EmbedMarker, marker)
	}

	_, err = f.Seek(-MarkerSize-LengthSize, io.SeekEnd)
	if err != nil {
		return "", fmt.Errorf("定位长度位置失败: %v", err)
	}

	lenBuf := make([]byte, LengthSize)
	_, err = io.ReadFull(f, lenBuf)
	if err != nil {
		return "", fmt.Errorf("读取脚本长度失败: %v", err)
	}
	scriptLen := binary.BigEndian.Uint64(lenBuf)

	maxPossibleLen := uint64(size) - HeaderSize
	if scriptLen > maxPossibleLen || scriptLen == 0 {
		return "", fmt.Errorf("无效的脚本长度: %d (文件大小: %d, 最大可能长度: %d)",
			scriptLen, size, maxPossibleLen)
	}

	scriptStart := size - int64(HeaderSize+scriptLen)
	if scriptStart < 0 {
		return "", fmt.Errorf("脚本位置无效: %d (文件大小: %d, 脚本长度: %d)",
			scriptStart, size, scriptLen)
	}

	_, err = f.Seek(scriptStart, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("定位脚本位置失败: %v", err)
	}

	scriptData := make([]byte, scriptLen)
	_, err = io.ReadFull(f, scriptData)
	if err != nil {
		return "", fmt.Errorf("读取脚本内容失败: %v", err)
	}

	return string(scriptData), nil
}

func verifyLinkedFile(file string, expectedScript []byte) {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("验证失败：读取文件错误: %v\n", err)
		return
	}

	totalSize := len(data)
	if totalSize < 16 {
		fmt.Printf("验证失败：文件太小（%d < 16）\n", totalSize)
		return
	}

	markerStart := totalSize - MarkerSize
	actualMarker := string(data[markerStart:totalSize])

	if actualMarker != EmbedMarker {
		fmt.Printf("验证失败：标记不正确，期望 %q, 实际 %q\n", EmbedMarker, actualMarker)
		return
	}

	lenStart := markerStart - LengthSize
	scriptLen := binary.BigEndian.Uint64(data[lenStart:markerStart])

	scriptStart := totalSize - HeaderSize - int(scriptLen)
	if scriptStart < 0 {
		fmt.Printf("验证失败：脚本起始位置无效 (%d)\n", scriptStart)
		return
	}

	scriptEnd := scriptStart + int(scriptLen)
	actualScript := data[scriptStart:scriptEnd]

	if !bytes.Equal(actualScript, expectedScript) {
		fmt.Println("验证失败：脚本内容不匹配")
		fmt.Printf("期望长度: %d, 实际长度: %d\n", len(expectedScript), len(actualScript))

		diffLimit := 100
		if len(expectedScript) < diffLimit {
			diffLimit = len(expectedScript)
		}
		if len(actualScript) < diffLimit {
			diffLimit = len(actualScript)
		}

		fmt.Println("期望内容 (前100字节):", string(expectedScript[:diffLimit]))
		fmt.Println("实际内容 (前100字节):", string(actualScript[:diffLimit]))
		return
	}

	fmt.Println("検証成功：埋め込みスクリプトは正しいです")
}

const (
	OP_HALT = iota
	OP_CONST
	OP_ADD
	OP_SUB
	OP_MUL
	OP_DIV
	OP_PRINT
	OP_SET_GLOBAL
	OP_GET_GLOBAL
	OP_SET_LOCAL
	OP_GET_LOCAL
	OP_JUMP_IF_FALSE
	OP_JUMP
	OP_LOOP
	OP_CALL
	OP_RETURN
	OP_LIST
	OP_LIST_APPEND
	OP_DICT
	OP_INDEX
	OP_LEN
	OP_EQ
	OP_NEQ
	OP_LT
	OP_GT
	OP_LEQ
	OP_GEQ
	OP_NEG
	OP_NOT
	OP_POP
	OP_ROCK
	OP_AND
	OP_OR
	OP_SET_INDEX
	OP_GET_ATTR
	OP_SET_ATTR
	OP_CALL_METHOD
	OP_MAKE_CLOSURE
	OP_CLOSURE
	OP_POP_JUMP_IF_FALSE
	OP_JUMP_FORWARD
	OP_BUILD_LIST
	OP_BUILD_MAP
	OP_STORE_SUBSCR
	OP_LOAD_SUBSCR
	OP_UNPACK_SEQUENCE
	OP_FOR_ITER
	OP_GET_ITER
	OP_SETUP_EXCEPT
	OP_POP_BLOCK
	OP_GO
	OP_MAKE_FUNCTION
	NEG
	NOT
	OP_LIST_SET
	OP_LIST_GET
	OP_BUILD_DICT
	OP_DICT_GET
	OP_DICT_SET
)

type MemberAccess struct {
	Object ASTNode
	Member string
}

type MethodCall struct {
	Object ASTNode
	Method string
	Args   []ASTNode
}

func (u UnaryOp) Eval(ctx *ExecutionContext) interface{} {
	// 评估操作数
	operandVal := u.Operand.Eval(ctx)

	switch u.Op {
	case PLUS:
		// 一元加号，通常不改变值，但需要确保操作数是数字
		switch v := operandVal.(type) {
		case float64:
			return v
		case *big.Float:
			return v
		case int:
			return float64(v)
		case string:
			// 尝试转换为数字
			if num, err := strconv.ParseFloat(v, 64); err == nil {
				return num
			}
			panic("一元加号操作符需要数字操作数")
		default:
			panic("一元加号操作符需要数字操作数")
		}

	case MINUS:
		// 一元减号，取负值
		switch v := operandVal.(type) {
		case float64:
			return -v
		case *big.Float:
			// 大数的负数
			result := new(big.Float).Neg(v)
			return result
		case int:
			return float64(-v)
		case string:
			// 尝试转换为数字并取负
			if num, err := strconv.ParseFloat(v, 64); err == nil {
				return -num
			}
			panic("一元减号操作符需要数字操作数")
		default:
			panic("一元减号操作符需要数字操作数")
		}

	default:
		panic(fmt.Sprintf("未知的一元操作符: %v", u.Op))
	}
}

type UnaryOp struct {
	Op      TokenType
	Operand ASTNode
}

type BytecodeCompiler struct {
	code      []byte
	constants []interface{}
	globals   map[string]int
	locals    []map[string]int
	loops     []int
}

func NewBytecodeCompiler() *BytecodeCompiler {
	return &BytecodeCompiler{
		globals: make(map[string]int),
		locals:  []map[string]int{make(map[string]int)},
	}
}

func executeModule(code string, globalEnv *ExecutionContext) {
	lexer := NewLexer(code)
	parser := NewParser(lexer)
	stmts := parser.Parse()

	// 使用传入的全局环境执行模块代码
	for _, stmt := range stmts {
		stmt.Eval(globalEnv)
	}
}

func showHelp() {
	// 清除屏幕（可选）
	clearScreen()

	// 横幅和基础信息（版本升级: 1.13.4 → 1.14.0，Bocchi 4.0.3 → 4.1）
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                        BOCCHI 脚本语言                        ║")
	fmt.Println("║                   版本 1.14.1 (Bocchi 4.1.4)                  ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("📌 核心使用方式:")
	fmt.Println("  bocchi <脚本文件>          执行 Bocchi 脚本")
	fmt.Println("  bocchi -repl              启动交互式 REPL 环境")
	fmt.Println("  bocchi -help              显示此帮助信息")
	fmt.Println("  bocchi -version           显示版本信息")
	fmt.Println()

	// ==================== 4.1 新特性说明 ====================
	fmt.Println("🔍 拆分调试模式 (Bocchi 4.1 新增):")
	fmt.Println("  bocchi \"?<脚本文件>\"        将脚本按顶层语句拆分为语句块")
	fmt.Println("                            所有块共享同一全局变量环境，逐块执行")
	fmt.Println("                            某块出错时报告精确的「行:列 + 语句类型」")
	fmt.Println("                            出错不中断后续块，最终输出失败清单")
	fmt.Println()
	fmt.Println("  示例:")
	fmt.Println("    bocchi \"?script.bocchi\"")
	fmt.Println("    bocchi \"?C:\\projects\\bug.bocchi\"")
	fmt.Println()
	fmt.Println("  适用场景:")
	fmt.Println("    · 大型脚本某行报错，但普通模式调用栈不清晰")
	fmt.Println("    · 一次执行发现全部错误位置，而不必改一行跑一遍")
	fmt.Println("    · 排查「变量是否在被污染后仍全局一致」的问题")
	fmt.Println()

	fmt.Println("🚀 性能分析功能:")
	fmt.Println("  -prof, --profile          启用性能分析")
	fmt.Println("  -profile-memory           启用内存跟踪分析")
	fmt.Println("  -profile-slow <毫秒>      设置慢操作阈值（默认: 100ms）")
	fmt.Println()
	fmt.Println("  示例:")
	fmt.Println("    bocchi -prof script.bocchi")
	fmt.Println("    bocchi --profile -profile-memory script.bocchi")
	fmt.Println("    bocchi -prof -profile-slow 50 script.bocchi")
	fmt.Println()

	fmt.Println("📦 包管理命令:")
	fmt.Println("  bocchi -install <包名>     安装包")
	fmt.Println("  bocchi -uninstall <包名>   卸载包")
	fmt.Println("  bocchi -list               列出已安装的包")
	fmt.Println()
	fmt.Println("  bocchi -package write <包名>   编辑包信息")
	fmt.Println("  bocchi -package make <包名>    创建加密包")
	fmt.Println("  bocchi -package install <包名> 安装加密包")
	fmt.Println()

	fmt.Println("🔗 可执行文件打包:")
	fmt.Println("  bocchi -link <源文件> <输出文件> [图标文件]")
	fmt.Println()
	fmt.Println("  示例:")
	fmt.Println("    bocchi -link script.bocchi app.exe")
	fmt.Println("    bocchi -link script.bocchi app.exe icon.ico  # Windows图标")
	fmt.Println()

	fmt.Println("🛠️ 系统集成:")
	fmt.Println("  bocchi -start              创建示例脚本文件")
	fmt.Println("  bocchi -install            将Bocchi安装到系统路径")
	fmt.Println("  （需要管理员权限）")
	fmt.Println()

	fmt.Println("🎯 4.1 新功能亮点:")
	fmt.Println("  • 拆分调试模式：?<文件> 逐语句块执行，精确定位错误行号")
	fmt.Println("  • 智能性能分析器：检测慢操作、内存使用和热点代码")
	fmt.Println("  • 图像识别自动化：支持图像查找、点击、等待等操作")
	fmt.Println("  • 加密包管理：安全的包分发机制")
	fmt.Println("  • 鼠标键盘自动化：全面的UI自动化支持")
	fmt.Println("  • 静态变量支持：static关键字声明全局静态变量")
	fmt.Println()

	fmt.Println("💡 性能优化建议:")
	fmt.Println("  1. 使用 -prof 参数识别程序瓶颈")
	fmt.Println("  2. 对于复杂计算，考虑使用尾递归优化")
	fmt.Println("  3. 避免在循环中创建大量临时对象")
	fmt.Println("  4. 使用 static 变量存储频繁访问的数据")
	fmt.Println("  5. 图像识别时合理设置置信度和搜索区域")
	fmt.Println()

	fmt.Println("  • 完整文档: 查看说明书或项目README")
	fmt.Println("  • 示例脚本: 使用 -start 命令创建")
	fmt.Println()

	// 页脚
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("By Yukio - 2024-2026 | Bocchi 4.1 | 高性能脚本语言解释器")
	fmt.Println()
}

func isEmbeddedExecutable() bool {
	_, err := readEmbeddedScript()
	return err == nil
}

func waitForExit() {
	fmt.Print("\n\n按任意键退出...")

	// 清除输入缓冲区
	reader := bufio.NewReader(os.Stdin)
	reader.ReadRune() // 等待单个字符

	// 可选：清空剩余输入
	for {
		reader.ReadRune()
		if reader.Buffered() == 0 {
			break
		}
	}
}

// 更新REPL帮助信息
func showREPLHelp() {
	fmt.Println("REPL命令:")
	fmt.Println("  exit/quit - 退出REPL")
	fmt.Println("  clear     - 清空屏幕")
	fmt.Println("  title     - 设置窗口标题")
	fmt.Println("  env       - 显示当前环境变量")
	fmt.Println("  static    - 显示静态变量")
	fmt.Println("  packages  - 显示已安装的包")
	fmt.Println("  args      - 显示命令行参数")
	fmt.Println("  help      - 显示此帮助信息")
	fmt.Println()
	fmt.Println("输入Bocchi代码直接执行，支持多行输入")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  title(\"Bocchi计算器\") - 设置窗口标题")
	fmt.Println("  os.args        - 获取所有命令行参数")
	fmt.Println("  os.args[0]     - 获取第一个参数")
	fmt.Println("  os.argc        - 获取参数数量")
}

func startREPL() {
	fmt.Println("Bocchi REPL v1.14.0 (Bocchi 4.1)")
	fmt.Println("输入 'exit' 或 'quit' 退出")
	fmt.Println("输入 'clear' 清空屏幕")
	fmt.Println("输入 'env' 查看当前环境变量")
	fmt.Println("输入 'static' 查看静态变量")
	fmt.Println("输入 'packages' 查看已安装的包")
	fmt.Println("输入 'args' 查看命令行参数")

	globalMu := &sync.Mutex{}
	replEnv := &ExecutionContext{
		env:           make(map[string]VarInfo),
		deferStack:    make(map[int][]DeferItem),
		wg:            &sync.WaitGroup{},
		globalEnv:     nil,
		globalMu:      globalMu,
		loadedModules: make(map[string]bool),
		Args:          []string{}, // REPL模式下参数为空
	}
	replEnv.globalEnv = replEnv

	// 在REPL启动时加载所有包
	loadAllPackages(replEnv)

	scanner := bufio.NewScanner(os.Stdin)
	var inputBuffer strings.Builder
	var lineCount int

	for {
		if inputBuffer.Len() == 0 {
			fmt.Print("bocchi> ")
		} else {
			fmt.Print("... ")
		}

		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		lineCount++

		if inputBuffer.Len() == 0 {
			trimmed := strings.TrimSpace(line)
			switch trimmed {
			case "exit", "quit":
				return
			case "clear":
				clearScreen()
				continue
			case "env":
				printEnvironment(replEnv)
				continue
			case "static":
				printStaticVariables()
				continue
			case "packages":
				listPackages()
				continue
			case "args":
				fmt.Println("命令行参数:", replEnv.Args)
				fmt.Printf("参数数量: %d\n", len(replEnv.Args))
				for i, arg := range replEnv.Args {
					fmt.Printf("  args[%d] = %s\n", i, arg)
				}
				continue
			case "help":
				showREPLHelp()
				continue
			}
		}

		inputBuffer.WriteString(line)
		inputBuffer.WriteString("\n")

		code := inputBuffer.String()

		success, shouldReset := tryExecute(code, replEnv)
		if success || shouldReset {
			inputBuffer.Reset()
		}

		if !success && !shouldReset {
			inputBuffer.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("读取输入错误: %v\n", err)
	}
}

func printStaticVariables() {
	staticMutex.Lock()
	defer staticMutex.Unlock()

	fmt.Println("$STATIC:")
	if len(staticVariables) == 0 {
		fmt.Println("  (空)")
		return
	}

	var names []string
	for name := range staticVariables {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		info := staticVariables[name]
		fmt.Printf("  %s: %v (%v)\n", name, info.Value, info.Type)
	}
}

func tryExecute(code string, ctx *ExecutionContext) (bool, bool) {
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("%v", r)
			if strings.Contains(errMsg, "未关闭") ||
				strings.Contains(errMsg, "Expected") ||
				strings.Contains(errMsg, "unexpected EOF") {
				return
			}
			fmt.Printf("错误: %v\n", r)
		}
	}()

	lexer := NewLexer(code)
	parser := NewParser(lexer)

	stmts := parser.Parse()

	if len(stmts) == 0 {
		return true, true
	}

	for _, stmt := range stmts {
		var result interface{}

		// 在 REPL 中也使用性能分析
		if globalProfiler != nil && globalProfiler.enabled {
			stmtType := fmt.Sprintf("%T", stmt)
			result = evalWithProfiling(stmt, stmtType, ctx)
		} else {
			result = stmt.Eval(ctx)
		}

		if result != nil {
			fmt.Printf("→ %v\n", result)
		}
	}

	return true, true
}

func clearScreen() {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func printEnvironment(ctx *ExecutionContext) {
	fmt.Println("$ENV:")
	if len(ctx.env) == 0 {
		fmt.Println("  (空)")
		return
	}

	var names []string
	for name := range ctx.env {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		info := ctx.env[name]
		fmt.Printf("  %s: %v (%v)\n", name, info.Value, info.Type)
	}
}

func (o OsCmdExpr) Eval(ctx *ExecutionContext) interface{} {
	cmdStr := fmt.Sprint(o.Command.Eval(ctx))

	// 安全警告 - 禁止潜在危险命令
	if strings.Contains(cmdStr, "&") || strings.Contains(cmdStr, "|") ||
		strings.Contains(cmdStr, ">") || strings.Contains(cmdStr, "<") ||
		strings.Contains(cmdStr, "rm") || strings.Contains(cmdStr, "del") ||
		strings.Contains(cmdStr, "format") || strings.Contains(cmdStr, "chkdsk") {
		return "警告: 检测到可能不安全的命令字符"
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/C", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("命令执行错误: %v", err)
	}

	// Windows中文系统下转换编码：
	// 子进程输出可能是 UTF-8（Go 程序等），也可能是 GBK（cmd 内建命令）。
	// 最小改动方案：合法 UTF-8 直接使用，否则按 GBK 解码。
	if runtime.GOOS == "windows" && isChineseSystem() {
		if utf8.Valid(output) {
			return strings.TrimSuffix(string(output), "\r\n")
		}
		utf8Output, err := gbkToUTF8(output)
		if err == nil {
			// 移除Windows命令输出的尾部换行符
			return strings.TrimSuffix(utf8Output, "\r\n")
		}
	}

	// 通用处理
	if runtime.GOOS == "windows" {
		return strings.TrimSuffix(string(output), "\r\n")
	}
	return string(output)
}

func gbkToUTF8(s []byte) (string, error) {
	reader := transform.NewReader(bytes.NewReader(s), simplifiedchinese.GBK.NewDecoder())
	d, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(d), nil
}

func isChineseSystem() bool {
	// 方法1: 检查注册表中的系统语言
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Nls\Language`, registry.READ)
	if err == nil {
		defer k.Close()
		lang, _, _ := k.GetStringValue("Default")
		if lang == "0804" { // 简体中文代码
			return true
		}
	}

	// 方法2: 检查控制台代码页
	cmd := exec.Command("cmd.exe", "/c", "chcp")
	output, err := cmd.Output()
	if err == nil {
		strOutput := string(output)
		if strings.Contains(strOutput, "936") {
			return true
		}
	}

	// 方法3: 检查系统区域设置
	k, err = registry.OpenKey(registry.CURRENT_USER, `Control Panel\International`, registry.READ)
	if err == nil {
		defer k.Close()
		locale, _, _ := k.GetStringValue("Locale")
		if locale == "00000804" {
			return true
		}
	}

	return false
}

// 包管理相关常量和结构
const (
	packagesDir    = ".bocchi_packages"
	packagesConfig = "packages.json"
)

type PackageConfig struct {
	InstalledPackages []string `json:"installed_packages"`
}

// 获取包目录路径
func getPackagesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("无法获取用户主目录: %v", err))
	}
	return filepath.Join(home, packagesDir)
}

// 获取包配置文件路径
func getPackagesConfigPath() string {
	return filepath.Join(getPackagesDir(), packagesConfig)
}

// 初始化包目录和配置
func initPackages() {
	dir := getPackagesDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(fmt.Sprintf("创建包目录失败: %v", err))
	}

	configPath := getPackagesConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := PackageConfig{InstalledPackages: []string{}}
		savePackageConfig(defaultConfig)
	}
}

// 读取包配置
func loadPackageConfig() PackageConfig {
	configPath := getPackagesConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return PackageConfig{InstalledPackages: []string{}}
	}

	var config PackageConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return PackageConfig{InstalledPackages: []string{}}
	}
	return config
}

// 保存包配置
func savePackageConfig(config PackageConfig) {
	configPath := getPackagesConfigPath()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("序列化包配置失败: %v", err))
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		panic(fmt.Sprintf("保存包配置失败: %v", err))
	}
}

// 安装包
func installPackage(packagePath string) {
	// 清理路径
	packagePath = filepath.Clean(packagePath)

	// 获取包名（去掉路径和扩展名）
	packageName := filepath.Base(packagePath)
	lowerName := strings.ToLower(packageName)
	if strings.HasSuffix(lowerName, ".bocchi") {
		packageName = packageName[:len(packageName)-len(".bocchi")]
	} else if strings.HasSuffix(lowerName, ".tuka") {
		// 扩展包（.tuka）同样作为可安装包处理：io3.tuka → io3
		packageName = packageName[:len(packageName)-len(".tuka")]
	}

	// 验证包名
	if !isValidPackageName(packageName) {
		fmt.Printf("错误: 包名 '%s' 不合法\n", packageName)
		fmt.Println("包名只能包含字母、数字、点、连字符和下划线，且不能以点开头或结尾")
		return
	}
	// 检查是否有对应的信息文件（.bocchi / .tuka 均支持）
	infoFile := strings.TrimSuffix(packagePath, filepath.Ext(packagePath)) + ".info.json"
	if _, err := os.Stat(infoFile); err == nil {
		// 复制信息文件到包目录
		packagesDir := getPackagesDir()
		destInfoFile := filepath.Join(packagesDir, packageName+".info.json")
		if err := copyFile(infoFile, destInfoFile); err != nil {
			fmt.Printf("警告: 无法复制包信息文件: %v\n", err)
		}
	}

	initPackages()
	config := loadPackageConfig()

	// 检查包是否已安装
	for _, pkg := range config.InstalledPackages {
		if pkg == packageName {
			fmt.Printf("包 '%s' 已经安装\n", packageName)
			return
		}
	}

	// 检查包文件是否存在
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		fmt.Printf("错误: 包文件 '%s' 不存在\n", packagePath)
		return
	}

	// 复制包文件到包目录
	packagesDir := getPackagesDir()
	destPath := filepath.Join(packagesDir, packageName+".bocchi")

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		fmt.Printf("创建目标目录失败: %v\n", err)
		return
	}

	if err := copyFile(packagePath, destPath); err != nil {
		fmt.Printf("安装包失败: %v\n", err)
		return
	}

	// 更新配置
	config.InstalledPackages = append(config.InstalledPackages, packageName)
	savePackageConfig(config)

	fmt.Printf("包 '%s' 安装成功\n", packageName)
}

// 卸载包
func uninstallPackage(packageName string) {
	initPackages()
	config := loadPackageConfig()

	found := false
	var newPackages []string
	for _, pkg := range config.InstalledPackages {
		if pkg == packageName {
			found = true
			// 修复：安装时统一保存为 <name>.bocchi，此处必须带后缀才能删除
			packagePath := filepath.Join(getPackagesDir(), packageName+".bocchi")
			if err := os.Remove(packagePath); err != nil && !os.IsNotExist(err) {
				//fmt.Printf("警告: 无法删除包文件: %v\n", err)
			}
			// 同时删除信息文件（若存在）
			_ = os.Remove(filepath.Join(getPackagesDir(), packageName+".info.json"))
		} else {
			newPackages = append(newPackages, pkg)
		}
	}

	if !found {
		fmt.Printf("包 '%s' 未安装\n", packageName)
		return
	}

	config.InstalledPackages = newPackages
	savePackageConfig(config)

	fmt.Printf("包 '%s' 卸载成功\n", packageName)
}

// 列出所有包
func listPackages() {
	initPackages()
	config := loadPackageConfig()

	if len(config.InstalledPackages) == 0 {
		fmt.Println("没有安装任何包")
		return
	}

	fmt.Println("已安装的包:")
	for i, pkg := range config.InstalledPackages {
		fmt.Printf("%d. %s\n", i+1, pkg)
	}
}

// 加载所有已安装的包
func loadAllPackages(globalEnv *ExecutionContext) {
	initPackages()
	config := loadPackageConfig()

	for _, pkg := range config.InstalledPackages {
		packageFile := pkg + ".bocchi"
		packagePath := filepath.Join(getPackagesDir(), packageFile)

		content, err := os.ReadFile(packagePath)
		if err != nil {
			fmt.Printf("警告: 无法加载包 '%s': %v\n", pkg, err)
			continue
		}

		//fmt.Printf("加载包: %s\n", pkg)

		// 直接使用同一个全局环境执行包代码
		executeModuleInSameEnv(string(content), globalEnv)
	}
}

func executeModuleInSameEnv(code string, ctx *ExecutionContext) {
	lexer := NewLexer(code)
	parser := NewParser(lexer)
	stmts := parser.Parse()

	// 使用相同的环境执行包代码
	for _, stmt := range stmts {
		// 如果启用了性能分析，使用性能分析包装器
		if globalProfiler != nil && globalProfiler.enabled {
			// 获取语句类型名称用于性能分析
			stmtType := fmt.Sprintf("%T", stmt)
			evalWithProfiling(stmt, stmtType, ctx)
		} else {
			stmt.Eval(ctx)
		}
	}
}

// 验证包名是否合法
func isValidPackageName(name string) bool {
	if len(name) == 0 || len(name) > 50 {
		return false
	}

	// 检查是否包含路径分隔符或其他非法字符
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return false
	}

	// 检查是否以点开头或结尾
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return false
	}

	// 检查是否包含空格
	if strings.Contains(name, " ") {
		return false
	}

	// 检查每个字符是否都是字母、数字、点、连字符或下划线
	for _, char := range name {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return false
		}
	}

	return true
}

func handlePackageCommand() {
	if len(os.Args) < 3 {
		showPackageHelp()
		return
	}

	subCommand := os.Args[2]

	switch subCommand {
	case "write":
		if len(os.Args) < 4 {
			fmt.Println("使用方法: bocchi -package write <包名>")
			return
		}
		packageName := os.Args[3]
		writePackageInfo(packageName)
	case "make":
		if len(os.Args) < 4 {
			fmt.Println("使用方法: bocchi -package make <包名>")
			return
		}
		packageName := os.Args[3]
		makeEncryptedPackage(packageName)
	case "install":
		if len(os.Args) < 4 {
			fmt.Println("使用方法: bocchi -package install <包名>")
			return
		}
		packageName := os.Args[3]
		installEncryptedPackage(packageName)
	default:
		showPackageHelp()
	}
}

// 显示包管理帮助
func showPackageHelp() {
	fmt.Println("包管理命令:")
	fmt.Println("  bocchi -package write <包名>    - 编辑包的名称和说明")
	fmt.Println("  bocchi -package make <包名>     - 加密一个新包")
	fmt.Println("  bocchi -package install <包名>  - 先解密包再安装")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  bocchi -package write math_utils")
	fmt.Println("  bocchi -package make math_utils")
	fmt.Println("  bocchi -package install math_utils")
}

// 包信息结构
type PackageInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	CreatedAt   string `json:"created_at"`
}

// 编辑包信息
func writePackageInfo(packageName string) {
	if !isValidPackageName(packageName) {
		fmt.Printf("错误: 包名 '%s' 不合法\n", packageName)
		return
	}

	// 检查包文件是否存在
	packageFile := packageName + ".bocchi"
	if _, err := os.Stat(packageFile); os.IsNotExist(err) {
		fmt.Printf("错误: 包文件 '%s' 不存在\n", packageFile)
		fmt.Println("请先创建包文件，然后再编辑信息")
		return
	}

	// 读取现有的包信息（如果有）
	infoFile := packageName + ".info.json"
	var info PackageInfo

	if _, err := os.Stat(infoFile); err == nil {
		data, err := os.ReadFile(infoFile)
		if err == nil {
			json.Unmarshal(data, &info)
		}
	}

	// 如果信息为空，设置默认值
	if info.Name == "" {
		info.Name = packageName
	}
	if info.Version == "" {
		info.Version = "1.0.0"
	}
	if info.CreatedAt == "" {
		info.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	}

	fmt.Printf("编辑包信息: %s\n", packageName)
	fmt.Println("(直接按Enter保持原值)")

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("包名称 [%s]: ", info.Name)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name != "" {
		info.Name = name
	}

	fmt.Printf("包描述 [%s]: ", info.Description)
	desc, _ := reader.ReadString('\n')
	desc = strings.TrimSpace(desc)
	if desc != "" {
		info.Description = desc
	}

	fmt.Printf("版本 [%s]: ", info.Version)
	version, _ := reader.ReadString('\n')
	version = strings.TrimSpace(version)
	if version != "" {
		info.Version = version
	}

	fmt.Printf("作者 [%s]: ", info.Author)
	author, _ := reader.ReadString('\n')
	author = strings.TrimSpace(author)
	if author != "" {
		info.Author = author
	}

	// 保存包信息
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		fmt.Printf("保存包信息失败: %v\n", err)
		return
	}

	if err := os.WriteFile(infoFile, data, 0644); err != nil {
		fmt.Printf("保存包信息失败: %v\n", err)
		return
	}

	fmt.Printf("包信息已保存到: %s\n", infoFile)
}

// 加密包
func makeEncryptedPackage(packageName string) {
	if !isValidPackageName(packageName) {
		fmt.Printf("错误: 包名 '%s' 不合法\n", packageName)
		return
	}

	// 检查包文件是否存在
	packageFile := packageName + ".bocchi"
	if _, err := os.Stat(packageFile); os.IsNotExist(err) {
		fmt.Printf("错误: 包文件 '%s' 不存在\n", packageFile)
		return
	}

	// 读取包信息
	infoFile := packageName + ".info.json"
	var info PackageInfo

	if _, err := os.Stat(infoFile); os.IsNotExist(err) {
		fmt.Println("警告: 未找到包信息文件，使用默认信息")
		info = PackageInfo{
			Name:      packageName,
			Version:   "1.0.0",
			CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		}
	} else {
		data, err := os.ReadFile(infoFile)
		if err != nil {
			fmt.Printf("读取包信息失败: %v\n", err)
			return
		}
		if err := json.Unmarshal(data, &info); err != nil {
			fmt.Printf("解析包信息失败: %v\n", err)
			return
		}
	}

	// 获取加密密码
	fmt.Print("输入加密密码: ")
	password, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	password = strings.TrimSpace(password)

	if password == "" {
		fmt.Println("错误: 密码不能为空")
		return
	}

	// 读取包文件内容
	content, err := os.ReadFile(packageFile)
	if err != nil {
		fmt.Printf("读取包文件失败: %v\n", err)
		return
	}

	// 加密包内容
	encryptedContent, err := encryptData(content, password)
	if err != nil {
		fmt.Printf("加密失败: %v\n", err)
		return
	}

	// 创建加密包文件
	encryptedFile := packageName + ".enc.bocchi"
	if err := os.WriteFile(encryptedFile, encryptedContent, 0644); err != nil {
		fmt.Printf("创建加密包文件失败: %v\n", err)
		return
	}

	// 创建加密包的信息文件（包含元数据但不包含密码）
	encryptedInfoFile := packageName + ".enc.info.json"
	encryptedInfo := map[string]interface{}{
		"name":        info.Name,
		"description": info.Description,
		"version":     info.Version,
		"author":      info.Author,
		"created_at":  info.CreatedAt,
		"encrypted":   true,
		"algorithm":   "AES-256-GCM",
	}

	infoData, err := json.MarshalIndent(encryptedInfo, "", "  ")
	if err != nil {
		fmt.Printf("创建加密包信息失败: %v\n", err)
		return
	}

	if err := os.WriteFile(encryptedInfoFile, infoData, 0644); err != nil {
		fmt.Printf("保存加密包信息失败: %v\n", err)
		return
	}

	fmt.Printf("加密包已创建: %s\n", encryptedFile)
	fmt.Printf("加密包信息: %s\n", encryptedInfoFile)
	fmt.Println("请妥善保管密码，安装时需要输入密码解密")
}

// 安装加密包
func installEncryptedPackage(packageName string) {
	if !isValidPackageName(packageName) {
		fmt.Printf("错误: 包名 '%s' 不合法\n", packageName)
		return
	}

	// 检查加密包文件是否存在
	encryptedFile := packageName + ".enc.bocchi"
	if _, err := os.Stat(encryptedFile); os.IsNotExist(err) {
		fmt.Printf("错误: 加密包文件 '%s' 不存在\n", encryptedFile)
		return
	}

	// 检查加密包信息文件
	encryptedInfoFile := packageName + ".enc.info.json"
	if _, err := os.Stat(encryptedInfoFile); os.IsNotExist(err) {
		fmt.Printf("错误: 加密包信息文件 '%s' 不存在\n", encryptedInfoFile)
		return
	}

	// 读取加密包信息
	infoData, err := os.ReadFile(encryptedInfoFile)
	if err != nil {
		fmt.Printf("读取加密包信息失败: %v\n", err)
		return
	}

	var encryptedInfo map[string]interface{}
	if err := json.Unmarshal(infoData, &encryptedInfo); err != nil {
		fmt.Printf("解析加密包信息失败: %v\n", err)
		return
	}

	fmt.Printf("安装加密包: %s\n", encryptedInfo["name"])
	if desc, ok := encryptedInfo["description"].(string); ok && desc != "" {
		fmt.Printf("描述: %s\n", desc)
	}
	fmt.Printf("版本: %s\n", encryptedInfo["version"])
	if author, ok := encryptedInfo["author"].(string); ok && author != "" {
		fmt.Printf("作者: %s\n", author)
	}

	// 获取解密密码
	fmt.Print("输入解密密码: ")
	password, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	password = strings.TrimSpace(password)

	if password == "" {
		fmt.Println("错误: 密码不能为空")
		return
	}

	// 读取加密包内容
	encryptedContent, err := os.ReadFile(encryptedFile)
	if err != nil {
		fmt.Printf("读取加密包文件失败: %v\n", err)
		return
	}

	// 解密包内容
	decryptedContent, err := decryptData(encryptedContent, password)
	if err != nil {
		fmt.Printf("解密失败: %v\n", err)
		fmt.Println("请检查密码是否正确")
		return
	}

	// 创建临时文件保存解密后的内容
	tempFile := packageName + ".decrypted.bocchi"
	if err := os.WriteFile(tempFile, decryptedContent, 0644); err != nil {
		fmt.Printf("创建解密文件失败: %v\n", err)
		return
	}
	defer os.Remove(tempFile) // 安装完成后删除临时文件

	// 安装解密后的包
	fmt.Println("解密成功，开始安装包...")
	installPackage(tempFile)

	// 保存包信息到包目录
	packagesDir := getPackagesDir()
	infoFile := filepath.Join(packagesDir, packageName+".info.json")

	standardInfo := PackageInfo{
		Name:        getStringValue(encryptedInfo, "name", packageName),
		Description: getStringValue(encryptedInfo, "description", ""),
		Version:     getStringValue(encryptedInfo, "version", "1.0.0"),
		Author:      getStringValue(encryptedInfo, "author", ""),
		CreatedAt:   getStringValue(encryptedInfo, "created_at", time.Now().Format("2006-01-02 15:04:05")),
	}

	infoData, err = json.MarshalIndent(standardInfo, "", "  ")
	if err != nil {
		fmt.Printf("保存包信息失败: %v\n", err)
		return
	}

	if err := os.WriteFile(infoFile, infoData, 0644); err != nil {
		fmt.Printf("保存包信息失败: %v\n", err)
		return
	}

	fmt.Printf("加密包安装成功: %s\n", packageName)
}

// 辅助函数：从map中获取字符串值
func getStringValue(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultValue
}

// 加密数据（内存版本）
func encryptData(data []byte, password string) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("生成盐值失败: %v", err)
	}

	key := pbkdf2.Key([]byte(password), salt, iter, keySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密块失败: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("生成nonce失败: %v", err)
	}

	encrypted := gcm.Seal(nonce, nonce, data, nil)
	result := append(salt, encrypted...)

	return result, nil
}

// 解密数据（内存版本）
func decryptData(data []byte, password string) ([]byte, error) {
	if len(data) < saltSize {
		return nil, fmt.Errorf("数据已损坏")
	}

	salt := data[:saltSize]
	encrypted := data[saltSize:]

	key := pbkdf2.Key([]byte(password), salt, iter, keySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建解密块失败: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, fmt.Errorf("加密数据已损坏")
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败，密码错误或数据损坏")
	}

	return plaintext, nil
}

//加入新功能：putsf 不换行输出 putsr 重写输出

type PutsfStmt struct {
	Exprs []ASTNode
	Prec  int
}

type PutsrStmt struct {
	Exprs []ASTNode
	Prec  int
}

func (p PutsfStmt) Eval(ctx *ExecutionContext) interface{} {
	var output strings.Builder
	for i, expr := range p.Exprs {
		val := expr.Eval(ctx)

		// 使用新的大数格式化函数
		var str string
		if p.Prec >= 0 {
			switch v := val.(type) {
			case float64:
				str = strconv.FormatFloat(v, 'f', p.Prec, 64)
			default:
				str = fmt.Sprintf("%v", val)
			}
		} else {
			str = formatLargeNumber(val)
		}

		if i > 0 {
			output.WriteString(" ")
		}
		output.WriteString(str)
	}

	// 不换行输出
	fmt.Print(output.String())
	return nil
}

func (p PutsrStmt) Eval(ctx *ExecutionContext) interface{} {
	var output strings.Builder
	for i, expr := range p.Exprs {
		val := expr.Eval(ctx)

		// 使用新的大数格式化函数
		var str string
		if p.Prec >= 0 {
			switch v := val.(type) {
			case float64:
				str = strconv.FormatFloat(v, 'f', p.Prec, 64)
			default:
				str = fmt.Sprintf("%v", val)
			}
		} else {
			str = formatLargeNumber(val)
		}

		if i > 0 {
			output.WriteString(" ")
		}
		output.WriteString(str)
	}

	// 重写输出：使用回车符回到行首
	fmt.Print("\r" + output.String())
	return nil
}

func (p *Parser) parsePutsfStmt() ASTNode {
	p.eat(PUTSF)
	var exprs []ASTNode

	for p.curr.Type != SEMI && p.curr.Type != EOF {
		if p.curr.Type == STRING && strings.Contains(p.curr.Value, string([]byte{0})) {
			parts := strings.Split(p.curr.Value, string([]byte{0}))
			var embeddedParts []interface{}

			for i, part := range parts {
				if i%2 == 0 {
					if part != "" {
						embeddedParts = append(embeddedParts, part)
					}
				} else {
					embeddedParts = append(embeddedParts, Variable{name: part})
				}
			}

			exprs = append(exprs, EmbeddedString{Parts: embeddedParts})
			p.eat(STRING)
		} else {
			expr := p.parseExpr()
			exprs = append(exprs, expr)
		}

		if p.curr.Type == COMMA {
			p.eat(COMMA)
		} else {
			break
		}
	}

	prec := -1
	if p.curr.Type == COLON {
		p.eat(COLON)
		if p.curr.Type == INT {
			precVal, _ := strconv.Atoi(p.curr.Value)
			prec = precVal
			p.eat(INT)
		}
	}
	p.eat(SEMI)
	return PutsfStmt{Exprs: exprs, Prec: prec}
}

func (p *Parser) parsePutsrStmt() ASTNode {
	p.eat(PUTSR)
	var exprs []ASTNode

	for p.curr.Type != SEMI && p.curr.Type != EOF {
		if p.curr.Type == STRING && strings.Contains(p.curr.Value, string([]byte{0})) {
			parts := strings.Split(p.curr.Value, string([]byte{0}))
			var embeddedParts []interface{}

			for i, part := range parts {
				if i%2 == 0 {
					if part != "" {
						embeddedParts = append(embeddedParts, part)
					}
				} else {
					embeddedParts = append(embeddedParts, Variable{name: part})
				}
			}

			exprs = append(exprs, EmbeddedString{Parts: embeddedParts})
			p.eat(STRING)
		} else {
			expr := p.parseExpr()
			exprs = append(exprs, expr)
		}

		if p.curr.Type == COMMA {
			p.eat(COMMA)
		} else {
			break
		}
	}

	prec := -1
	if p.curr.Type == COLON {
		p.eat(COLON)
		if p.curr.Type == INT {
			precVal, _ := strconv.Atoi(p.curr.Value)
			prec = precVal
			p.eat(INT)
		}
	}
	p.eat(SEMI)
	return PutsrStmt{Exprs: exprs, Prec: prec}
}

// 辅助函数：判断值的真值
func isTruthy(val interface{}) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return v != ""
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return true
	}
}

const (
	QUICK_LIST = iota + 100
)

// 添加快速列表函数
type QuickListExpr struct {
	FuncName string
	Args     []ASTNode
}

func (ql QuickListExpr) Eval(ctx *ExecutionContext) interface{} {
	switch ql.FuncName {
	case "range":
		return ql.evalRange()
	case "repeat":
		return ql.evalRepeat()
	case "seq":
		return ql.evalSequence()
	default:
		panic("未知的快速列表函数: " + ql.FuncName)
	}
}

func (ql QuickListExpr) evalRange() interface{} {
	if len(ql.Args) == 0 {
		panic("range函数需要至少1个参数")
	}

	var start, end, step float64
	switch len(ql.Args) {
	case 1:
		start = 0
		end = ql.Args[0].Eval(nil).(float64)
		step = 1
	case 2:
		start = ql.Args[0].Eval(nil).(float64)
		end = ql.Args[1].Eval(nil).(float64)
		step = 1
	case 3:
		start = ql.Args[0].Eval(nil).(float64)
		end = ql.Args[1].Eval(nil).(float64)
		step = ql.Args[2].Eval(nil).(float64)
	default:
		panic("range函数最多接受3个参数")
	}

	var result []interface{}
	if step > 0 {
		for i := start; i < end; i += step {
			result = append(result, i)
		}
	} else {
		for i := start; i > end; i += step {
			result = append(result, i)
		}
	}
	return result
}

func (ql QuickListExpr) evalRepeat() interface{} {
	if len(ql.Args) != 2 {
		panic("repeat函数需要2个参数: 值和重复次数")
	}

	value := ql.Args[0].Eval(nil)
	count := int(ql.Args[1].Eval(nil).(float64))

	result := make([]interface{}, count)
	for i := 0; i < count; i++ {
		result[i] = value
	}
	return result
}

func (ql QuickListExpr) evalSequence() interface{} {
	// 创建连续序列，如 seq(1, 10, 2) 生成 [1, 3, 5, 7, 9]
	if len(ql.Args) < 2 {
		panic("seq函数需要至少2个参数")
	}

	from := ql.Args[0].Eval(nil).(float64)
	to := ql.Args[1].Eval(nil).(float64)

	var step float64 = 1
	if len(ql.Args) > 2 {
		step = ql.Args[2].Eval(nil).(float64)
	}

	var result []interface{}
	if step > 0 {
		for i := from; i <= to; i += step {
			result = append(result, i)
		}
	} else {
		for i := from; i >= to; i += step {
			result = append(result, i)
		}
	}
	return result
}

func isQuickListFunction(name string) bool {
	return name == "range" || name == "repeat" || name == "seq"
}

// 列表推导式 AST 节点
type ListComprehension struct {
	Variable string
	Source   ASTNode
	Filter   ASTNode // 可选
	Result   ASTNode
}

func (lc ListComprehension) Eval(ctx *ExecutionContext) interface{} {
	sourceVal := lc.Source.Eval(ctx)

	var source []interface{}
	switch s := sourceVal.(type) {
	case []interface{}:
		source = s
	default:
		panic("列表推导式的源必须是列表")
	}

	var result []interface{}
	for _, item := range source {
		// 创建临时环境
		tempCtx := &ExecutionContext{
			env:        make(map[string]VarInfo),
			globalEnv:  ctx.globalEnv,
			deferStack: make(map[int][]DeferItem),
			wg:         ctx.wg,
		}

		// 复制父环境
		for k, v := range ctx.env {
			tempCtx.env[k] = v
		}

		// 设置循环变量
		tempCtx.env[lc.Variable] = VarInfo{
			Type:  getValueType(item),
			Value: item,
		}

		// 检查过滤条件
		if lc.Filter != nil {
			filterVal := lc.Filter.Eval(tempCtx)
			if filter, ok := filterVal.(bool); ok && !filter {
				continue
			}
		}

		// 计算结果并添加到结果列表
		resultVal := lc.Result.Eval(tempCtx)
		result = append(result, resultVal)
	}

	return result
}

// 在 Parser 中添加解析
func (p *Parser) parseListComprehension() ASTNode {
	p.eat(LBRACKET)

	// 解析表达式: [expr for var in source if condition]
	resultExpr := p.parseExpr()

	p.eat(FOR) // 需要添加 FOR token

	varName := p.curr.Value
	p.eat(IDENT)

	p.eat(IN)

	sourceExpr := p.parseExpr()

	var filterExpr ASTNode
	if p.curr.Type == IF {
		p.eat(IF)
		filterExpr = p.parseExpr()
	}

	p.eat(RBRACKET)

	return ListComprehension{
		Variable: varName,
		Source:   sourceExpr,
		Filter:   filterExpr,
		Result:   resultExpr,
	}
}

// 添加大数格式化函数
func formatLargeNumber(val interface{}) string {
	switch v := val.(type) {
	case float64:
		// 检查是否是无穷大
		if math.IsInf(v, 0) {
			return "Infinity"
		}
		// 检查是否是NaN
		if math.IsNaN(v) {
			return "NaN"
		}

		// 对于浮点数，如果值很大但不超过float64范围，使用字符串格式避免科学计数法
		absVal := math.Abs(v)
		if absVal >= 1e15 || (absVal > 0 && absVal < 1e-6) {
			// 使用大数处理
			return formatFloatWithoutScientific(v)
		}

		// 对于普通范围的数字，检查是否为整数
		if v == math.Trunc(v) {
			// 整数，直接格式化为整数
			return strconv.FormatInt(int64(v), 10)
		}

		// 浮点数，使用适当的精度
		return strconv.FormatFloat(v, 'f', -1, 64)

	case int, int64, int32, int16, int8:
		// 整数类型直接转换为字符串
		return fmt.Sprintf("%d", v)

	default:
		// 其他类型使用默认格式化
		return fmt.Sprintf("%v", v)
	}
}

// 格式化浮点数，避免科学计数法
func formatFloatWithoutScientific(f float64) string {
	// 处理特殊情况
	if math.IsInf(f, 1) {
		return "+Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if math.IsNaN(f) {
		return "NaN"
	}

	// 使用big.Float来精确格式化
	bf := big.NewFloat(f)
	bf.SetPrec(1000) // 设置高精度

	// 转换为字符串，避免科学计数法
	str := bf.Text('f', -1)

	// 如果字符串太长，可以适当截断末尾的0
	if strings.Contains(str, ".") {
		str = strings.TrimRight(str, "0")
		str = strings.TrimRight(str, ".")
	}

	return str
}

type OsAdminCmdExpr struct {
	Command ASTNode
}

func (o OsAdminCmdExpr) Eval(ctx *ExecutionContext) interface{} {
	cmdStr := fmt.Sprint(o.Command.Eval(ctx))

	// 安全检查 - 禁止危险命令
	if containsDangerousCommands(cmdStr) {
		return "警告: 检测到可能不安全的命令字符，已阻止执行"
	}

	// 检查当前权限
	if !isAdmin() {
		// 尝试以管理员权限运行
		return runCommandAsAdmin(cmdStr)
	}

	// 已经是管理员，直接执行
	return executeCommand(cmdStr)
}

// 检查是否包含危险命令
func containsDangerousCommands(cmdStr string) bool {
	dangerousPatterns := []string{
		"format", "chkdsk", "del ", "rm ", "rmdir",
		"&", "|", ">", "<", "&&", "||",
		"net user", "net localgroup",
	}

	lowerCmd := strings.ToLower(cmdStr)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerCmd, pattern) {
			return true
		}
	}
	return false
}

// 以管理员权限运行命令
func runCommandAsAdmin(cmdStr string) string {
	if runtime.GOOS != "windows" {
		return "管理员权限功能仅在Windows平台支持"
	}

	// 使用PowerShell以管理员权限运行命令
	psCommand := fmt.Sprintf(`Start-Process cmd -Verb RunAs -ArgumentList "/c %s" -Wait`, cmdStr)
	cmd := exec.Command("powershell", "-Command", psCommand)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("管理员权限执行失败: %v", err)
	}

	// 处理中文编码：合法 UTF-8 直接使用，否则按 GBK 解码
	if isChineseSystem() {
		if utf8.Valid(output) {
			return strings.TrimSuffix(string(output), "\r\n")
		}
		utf8Output, err := gbkToUTF8(output)
		if err == nil {
			return strings.TrimSuffix(utf8Output, "\r\n")
		}
	}

	return strings.TrimSuffix(string(output), "\r\n")
}

// 普通命令执行（已有功能增强）
func executeCommand(cmdStr string) string {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/C", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("命令执行错误: %v", err)
	}

	// Windows中文系统编码处理：合法 UTF-8 直接使用，否则按 GBK 解码
	if runtime.GOOS == "windows" && isChineseSystem() {
		if utf8.Valid(output) {
			return strings.TrimSuffix(string(output), "\r\n")
		}
		utf8Output, err := gbkToUTF8(output)
		if err == nil {
			return strings.TrimSuffix(utf8Output, "\r\n")
		}
	}

	if runtime.GOOS == "windows" {
		return strings.TrimSuffix(string(output), "\r\n")
	}
	return string(output)
}

const (
	PROFILER_ENABLED = true
	MAX_CALL_STACK   = 100
)

// 性能分析数据结构
type Profiler struct {
	enabled      bool
	startTime    time.Time
	metrics      *ProfilerMetrics
	callStack    []*CallFrame
	currentFrame *CallFrame
	mu           sync.Mutex
}

type ProfilerMetrics struct {
	TotalExecutionTime time.Duration
	FunctionCalls      map[string]*FunctionMetrics
	MemoryAllocations  int64
	GarbageCollections int64
	LineExecutions     map[string]int // 文件:行号 -> 执行次数
	SlowOperations     []*SlowOpRecord
}

type FunctionMetrics struct {
	Name        string
	CallCount   int64
	TotalTime   time.Duration
	MinTime     time.Duration
	MaxTime     time.Duration
	MemoryUsage int64
	ChildCalls  map[string]int64
}

type CallFrame struct {
	FunctionName string
	StartTime    time.Time
	MemoryStart  uint64
	Parent       *CallFrame
	Children     []*CallFrame
}

type SlowOpRecord struct {
	Operation string
	Duration  time.Duration
	Location  string // 文件:行号
	Timestamp time.Time
}

// 性能分析配置
type ProfilerConfig struct {
	Enabled         bool
	SlowOpThreshold time.Duration // 慢操作阈值
	MemoryTracking  bool
	CallStackDepth  int
	OutputFormat    string // "text", "json", "html"
}

// 全局性能分析器实例
var globalProfiler *Profiler

// 初始化性能分析器
func initProfiler(config ProfilerConfig) {
	if !PROFILER_ENABLED {
		return
	}

	globalProfiler = &Profiler{
		enabled: config.Enabled,
		metrics: &ProfilerMetrics{
			FunctionCalls:  make(map[string]*FunctionMetrics),
			LineExecutions: make(map[string]int),
			SlowOperations: make([]*SlowOpRecord, 0),
		},
		callStack: make([]*CallFrame, 0, MAX_CALL_STACK),
	}

	if config.Enabled {
		globalProfiler.startTime = time.Now()
	}
}

// 性能分析器方法
func (p *Profiler) StartFunctionCall(funcName string, ctx *ExecutionContext) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	frame := &CallFrame{
		FunctionName: funcName,
		StartTime:    time.Now(),
		MemoryStart:  p.getMemoryUsage(),
		Parent:       p.currentFrame,
	}

	if p.currentFrame != nil {
		p.currentFrame.Children = append(p.currentFrame.Children, frame)
	}

	p.currentFrame = frame
	p.callStack = append(p.callStack, frame)

	// 记录函数调用开始
	if _, exists := p.metrics.FunctionCalls[funcName]; !exists {
		p.metrics.FunctionCalls[funcName] = &FunctionMetrics{
			Name:       funcName,
			ChildCalls: make(map[string]int64),
		}
	}
}

func (p *Profiler) EndFunctionCall(funcName string, ctx *ExecutionContext) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.callStack) == 0 || p.currentFrame == nil {
		return
	}

	duration := time.Since(p.currentFrame.StartTime)
	memoryUsed := p.getMemoryUsage() - p.currentFrame.MemoryStart

	// 更新函数指标
	if metrics, exists := p.metrics.FunctionCalls[funcName]; exists {
		metrics.CallCount++
		metrics.TotalTime += duration
		metrics.MemoryUsage += int64(memoryUsed)

		if metrics.MinTime == 0 || duration < metrics.MinTime {
			metrics.MinTime = duration
		}
		if duration > metrics.MaxTime {
			metrics.MaxTime = duration
		}

		// 记录子调用关系
		if p.currentFrame.Parent != nil {
			parentMetrics := p.metrics.FunctionCalls[p.currentFrame.Parent.FunctionName]
			parentMetrics.ChildCalls[funcName]++
		}
	}

	// 检查是否为慢操作
	if duration > 100*time.Millisecond { // 100ms阈值
		p.metrics.SlowOperations = append(p.metrics.SlowOperations, &SlowOpRecord{
			Operation: funcName,
			Duration:  duration,
			Location:  p.getCallLocation(),
			Timestamp: time.Now(),
		})
	}

	// 弹出调用栈
	if len(p.callStack) > 0 {
		p.callStack = p.callStack[:len(p.callStack)-1]
	}

	if len(p.callStack) > 0 {
		p.currentFrame = p.callStack[len(p.callStack)-1]
	} else {
		p.currentFrame = nil
	}
}

func (p *Profiler) RecordLineExecution(file string, line int) {
	if !p.enabled {
		return
	}

	key := fmt.Sprintf("%s:%d", file, line)
	p.metrics.LineExecutions[key]++
}

func (p *Profiler) RecordMemoryAllocation(size int64) {
	if !p.enabled {
		return
	}

	atomic.AddInt64(&p.metrics.MemoryAllocations, size)
}

func (p *Profiler) getMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

func (p *Profiler) getCallLocation() string {
	// 获取调用栈信息
	pc := make([]uintptr, 10)
	n := runtime.Callers(3, pc) // 跳过3层
	if n == 0 {
		return "unknown"
	}

	frames := runtime.CallersFrames(pc[:n])
	if frame, more := frames.Next(); more {
		return fmt.Sprintf("%s:%d", frame.File, frame.Line)
	}

	return "unknown"
}

// 生成性能报告
func (p *Profiler) GenerateReport() string {
	if !p.enabled {
		return "性能分析器未启用"
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.metrics.TotalExecutionTime = time.Since(p.startTime)

	var report strings.Builder
	report.WriteString("=== BOCCHI 智能性能分析报告 ===\n\n")

	// 总体统计
	report.WriteString("总体统计:\n")
	report.WriteString(fmt.Sprintf("总执行时间: %v\n", p.metrics.TotalExecutionTime))
	report.WriteString(fmt.Sprintf("内存分配: %d bytes\n", p.metrics.MemoryAllocations))
	report.WriteString(fmt.Sprintf("函数调用总数: %d\n", len(p.metrics.FunctionCalls)))
	report.WriteString(fmt.Sprintf("慢操作数量: %d\n\n", len(p.metrics.SlowOperations)))

	// 函数性能排行
	report.WriteString("函数性能排行 (按总执行时间):\n")
	functions := make([]*FunctionMetrics, 0, len(p.metrics.FunctionCalls))
	for _, metrics := range p.metrics.FunctionCalls {
		functions = append(functions, metrics)
	}

	sort.Slice(functions, func(i, j int) bool {
		return functions[i].TotalTime > functions[j].TotalTime
	})

	for i, fn := range functions {
		if i >= 10 { // 只显示前10个
			break
		}
		avgTime := time.Duration(0)
		if fn.CallCount > 0 {
			avgTime = fn.TotalTime / time.Duration(fn.CallCount)
		}
		report.WriteString(fmt.Sprintf("%2d. %s: 调用%d次, 总时间%v, 平均%v, 内存%d bytes\n",
			i+1, fn.Name, fn.CallCount, fn.TotalTime, avgTime, fn.MemoryUsage))
	}
	report.WriteString("\n")

	// 热点代码行
	report.WriteString("热点代码行 (执行次数最多):\n")
	lines := make([]struct {
		location string
		count    int
	}, 0, len(p.metrics.LineExecutions))

	for location, count := range p.metrics.LineExecutions {
		lines = append(lines, struct {
			location string
			count    int
		}{location, count})
	}

	sort.Slice(lines, func(i, j int) bool {
		return lines[i].count > lines[j].count
	})

	for i, line := range lines {
		if i >= 5 { // 只显示前5个
			break
		}
		report.WriteString(fmt.Sprintf("%2d. %s: 执行%d次\n", i+1, line.location, line.count))
	}
	report.WriteString("\n")

	// 慢操作警告
	if len(p.metrics.SlowOperations) > 0 {
		report.WriteString("慢操作警告:\n")
		for i, op := range p.metrics.SlowOperations {
			if i >= 5 { // 只显示前5个
				break
			}
			report.WriteString(fmt.Sprintf("%2d. %s: 耗时%v (位置: %s)\n",
				i+1, op.Operation, op.Duration, op.Location))
		}
		report.WriteString("\n")
	}

	// 内存使用建议
	report.WriteString("内存使用建议:\n")
	totalMemory := int64(0)
	for _, fn := range functions {
		totalMemory += fn.MemoryUsage
	}

	if totalMemory > 100*1024*1024 { // 100MB
		report.WriteString("⚠️  检测到高内存使用，建议优化数据结构和算法\n")
	}

	if len(functions) > 0 && functions[0].MemoryUsage > totalMemory/2 {
		report.WriteString(fmt.Sprintf("⚠️  函数 '%s' 占用过多内存，建议优化\n", functions[0].Name))
	}

	return report.String()
}

// 性能分析装饰器 - 用于包装AST节点的Eval方法
func withProfiling(node ASTNode, nodeType string, ctx *ExecutionContext) func(*ExecutionContext) interface{} {
	return func(execCtx *ExecutionContext) interface{} {
		if globalProfiler == nil || !globalProfiler.enabled {
			return node.Eval(execCtx)
		}

		start := time.Now()
		globalProfiler.StartFunctionCall(nodeType, execCtx)

		defer func() {
			globalProfiler.EndFunctionCall(nodeType, execCtx)

			// 记录慢操作
			duration := time.Since(start)
			if duration > 50*time.Millisecond {
				if globalProfiler != nil {
					globalProfiler.mu.Lock()
					globalProfiler.metrics.SlowOperations = append(globalProfiler.metrics.SlowOperations, &SlowOpRecord{
						Operation: nodeType,
						Duration:  duration,
						Location:  globalProfiler.getCallLocation(),
						Timestamp: time.Now(),
					})
					globalProfiler.mu.Unlock()
				}
			}
		}()

		return node.Eval(execCtx)
	}
}

func (fc FunctionCall) EvalWithProfiling(ctx *ExecutionContext) interface{} {
	if globalProfiler != nil && globalProfiler.enabled {
		return withProfiling(fc, "FunctionCall", ctx)(ctx)
	}
	return fc.evalWithoutCache(ctx)
}

func (w WhileLoop) EvalWithProfiling(ctx *ExecutionContext) interface{} {
	if globalProfiler != nil && globalProfiler.enabled {
		return withProfiling(w, "WhileLoop", ctx)(ctx)
	}

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(BreakError); ok {
				return
			}
			panic(r)
		}
	}()

	for {
		cond := w.condition.Eval(ctx)
		switch c := cond.(type) {
		case bool:
			if !c {
				return nil
			}
		default:
			panic("Loop condition must be boolean")
		}
		for _, stmt := range w.body {
			stmt.Eval(ctx)
		}
	}
}

func setupProfiler() {
	// 检查命令行参数来决定是否启用性能分析
	enableProfiling := false
	for _, arg := range os.Args {
		if arg == "-profile" || arg == "--profile" {
			enableProfiling = true
			break
		}
	}

	if enableProfiling {
		profilerConfig := ProfilerConfig{
			Enabled:         true,
			SlowOpThreshold: 100 * time.Millisecond,
			MemoryTracking:  true,
			CallStackDepth:  50,
			OutputFormat:    "text",
		}
		initProfiler(profilerConfig)
		fmt.Println("性能分析器已启用")
	}
}

// 性能分析包装器
func evalWithProfiling(node ASTNode, nodeType string, ctx *ExecutionContext) interface{} {
	if globalProfiler == nil || !globalProfiler.enabled {
		return node.Eval(ctx)
	}

	start := time.Now()
	globalProfiler.StartFunctionCall(nodeType, ctx)

	defer func() {
		globalProfiler.EndFunctionCall(nodeType, ctx)

		// 记录慢操作
		duration := time.Since(start)
		if duration > 50*time.Millisecond {
			globalProfiler.mu.Lock()
			globalProfiler.metrics.SlowOperations = append(globalProfiler.metrics.SlowOperations, &SlowOpRecord{
				Operation: nodeType,
				Duration:  duration,
				Location:  globalProfiler.getCallLocation(),
				Timestamp: time.Now(),
			})
			globalProfiler.mu.Unlock()
		}
	}()

	return node.Eval(ctx)
}

// 键盘操作AST节点
type KeyboardAction struct {
	Action TokenType
	Key    ASTNode
	Text   ASTNode // 用于type操作
}

// 鼠标操作AST节点
type MouseAction struct {
	Action TokenType
	Button TokenType // left, right, middle
	X      ASTNode
	Y      ASTNode
	Delta  ASTNode // 用于scroll操作
}

// 获取鼠标位置
type MouseGetPos struct{}

// 键盘操作实现
func (k KeyboardAction) Eval(ctx *ExecutionContext) interface{} {
	switch k.Action {
	case PRESS:
		key := k.Key.Eval(ctx).(string)
		err := robotgo.KeyToggle(key, "down")
		if err != nil {
			panic(fmt.Sprintf("键盘按下失败: %v", err))
		}
		return "按键按下: " + key

	case RELEASE:
		key := k.Key.Eval(ctx).(string)
		err := robotgo.KeyToggle(key, "up")
		if err != nil {
			panic(fmt.Sprintf("键盘释放失败: %v", err))
		}
		return "按键释放: " + key

	case TYPE:
		text := k.Text.Eval(ctx).(string)
		robotgo.WriteAll(text)
		oldClipboard, _ := robotgo.ReadAll()
		// 执行粘贴
		robotgo.WriteAll(text)
		robotgo.KeyTap("v", "ctrl")
		// 恢复旧内容
		robotgo.WriteAll(oldClipboard)
		return "文本输入: " + text

	default:
		panic("未知的键盘操作")
	}
}

// 鼠标操作实现
func (m MouseAction) Eval(ctx *ExecutionContext) interface{} {
	switch m.Action {
	case MOVE:
		xVal := m.X.Eval(ctx)
		yVal := m.Y.Eval(ctx)

		var x, y int
		switch v := xVal.(type) {
		case float64:
			x = int(v)
		case int:
			x = v
		default:
			panic("X坐标必须是数字")
		}

		switch v := yVal.(type) {
		case float64:
			y = int(v)
		case int:
			y = v
		default:
			panic("Y坐标必须是数字")
		}

		robotgo.Move(x, y)
		return fmt.Sprintf("鼠标移动到: (%d, %d)", x, y)

	case CLICK:
		var button string
		switch m.Button {
		case LEFT:
			button = "left"
		case RIGHT:
			button = "right"
		case MIDDLE:
			button = "center"
		default:
			button = "left" // 默认左键
		}

		robotgo.Click(button)
		return "鼠标点击: " + button

	case SCROLL:
		deltaVal := m.Delta.Eval(ctx)
		var delta int
		switch v := deltaVal.(type) {
		case float64:
			delta = int(v)
		case int:
			delta = v
		default:
			panic("滚动量必须是数字")
		}

		robotgo.Scroll(0, delta)
		return fmt.Sprintf("鼠标滚动: %d", delta)

	default:
		panic("未知的鼠标操作")
	}
}

// 获取鼠标位置实现
func (m MouseGetPos) Eval(ctx *ExecutionContext) interface{} {
	x, y := robotgo.GetMousePos()
	return map[string]interface{}{
		"x": x,
		"y": y,
	}
}

var (
	user32       = syscall.NewLazyDLL("user32.dll")
	keybdEvent   = user32.NewProc("keybd_event")
	mouseEvent   = user32.NewProc("mouse_event")
	setCursorPos = user32.NewProc("SetCursorPos")
	getCursorPos = user32.NewProc("GetCursorPos")
)

// Windows原生键盘输入
func TypeStringWindows(text string) error {
	for _, char := range text {
		typeCharacter(uint16(char))

	}
	return nil
}

func typeCharacter(char uint16) {
	// 按下键
	keybdEvent.Call(uintptr(char), 0, 1, 0)
	// 释放键
	keybdEvent.Call(uintptr(char), 0, 3, 0)
}

// Windows原生鼠标移动
func MoveMouseWindows(x, y int) {
	setCursorPos.Call(uintptr(x), uintptr(y))
}

// 获取鼠标位置
func GetMousePosWindows() (int, int) {
	type point struct {
		X, Y int32
	}
	var pt point
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return int(pt.X), int(pt.Y)
}

// 图像查找结果
type ImageMatchResult struct {
	X          int
	Y          int
	Width      int
	Height     int
	Confidence float64
	Found      bool
}

// 图像查找表达式
type ImageFindExpr struct {
	TemplatePath ASTNode
	Region       *RectRegion // 可选：搜索区域
	Confidence   ASTNode     // 可选：置信度阈值，默认0.8
	MaxWait      ASTNode     // 可选：最大等待时间（秒）
}

// 区域定义
type RectRegion struct {
	X      ASTNode
	Y      ASTNode
	Width  ASTNode
	Height ASTNode
}

// 图像点击表达式
type ImageClickExpr struct {
	TemplatePath ASTNode
	Button       TokenType // 可选：left, right, middle
	Region       *RectRegion
	Confidence   ASTNode
	MaxWait      ASTNode
	ClickCount   ASTNode // 可选：点击次数，默认1
}

// 图像等待表达式
type ImageWaitExpr struct {
	TemplatePath     ASTNode
	Region           *RectRegion
	Confidence       ASTNode
	MaxWait          ASTNode
	WaitForDisappear bool // 等待消失还是等待出现
}

// 图像存在检查
type ImageExistsExpr struct {
	TemplatePath ASTNode
	Region       *RectRegion
	Confidence   ASTNode
}

// 屏幕截图表达式
type ScreenshotExpr struct {
	SavePath ASTNode     // 可选：保存路径，如果不提供则不保存
	Region   *RectRegion // 可选：截图区域
}

// 图像匹配表达式
type ImageMatchExpr struct {
	Image1Path ASTNode
	Image2Path ASTNode
	Confidence ASTNode // 相似度阈值
}

// 加载图像
func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// 将图像转换为灰度
func toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}

	return gray
}

// 模板匹配算法（归一化相关系数匹配）
func templateMatch(screen, template *image.Gray, confidence float64) (ImageMatchResult, error) {
	screenBounds := screen.Bounds()
	templateBounds := template.Bounds()

	sw, sh := screenBounds.Dx(), screenBounds.Dy()
	tw, th := templateBounds.Dx(), templateBounds.Dy()

	if sw < tw || sh < th {
		return ImageMatchResult{Found: false}, errors.New("模板图像比屏幕图像大")
	}

	bestX, bestY := 0, 0
	bestScore := -1.0

	// 简化版的模板匹配
	for y := 0; y <= sh-th; y++ {
		for x := 0; x <= sw-tw; x++ {
			var sumScreen, sumTemplate, sumScreen2, sumTemplate2, sumProduct float64

			// 计算局部区域的相关系数
			for ty := 0; ty < th; ty++ {
				for tx := 0; tx < tw; tx++ {
					screenPixel := float64(screen.GrayAt(x+tx, y+ty).Y)
					templatePixel := float64(template.GrayAt(tx, ty).Y)

					sumScreen += screenPixel
					sumTemplate += templatePixel
					sumScreen2 += screenPixel * screenPixel
					sumTemplate2 += templatePixel * templatePixel
					sumProduct += screenPixel * templatePixel
				}
			}

			n := float64(tw * th)
			numerator := n*sumProduct - sumScreen*sumTemplate
			denominator := math.Sqrt((n*sumScreen2 - sumScreen*sumScreen) * (n*sumTemplate2 - sumTemplate*sumTemplate))

			if denominator == 0 {
				continue
			}

			score := numerator / denominator
			if score > bestScore {
				bestScore = score
				bestX = x
				bestY = y
			}
		}
	}

	if bestScore >= confidence {
		return ImageMatchResult{
			X:          bestX + tw/2,
			Y:          bestY + th/2,
			Width:      tw,
			Height:     th,
			Confidence: bestScore,
			Found:      true,
		}, nil
	}

	return ImageMatchResult{Found: false}, nil
}

// 截图函数
func captureScreen(region *RectRegion, ctx *ExecutionContext) (image.Image, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, errors.New("没有找到显示器")
	}

	var img image.Image
	var err error

	if region != nil {
		// 截取指定区域
		x := int(region.X.Eval(ctx).(float64))
		y := int(region.Y.Eval(ctx).(float64))
		width := int(region.Width.Eval(ctx).(float64))
		height := int(region.Height.Eval(ctx).(float64))

		// 使用第一个显示器
		bounds := image.Rect(x, y, x+width, y+height)
		img, err = screenshot.CaptureRect(bounds)
	} else {
		// 截取整个屏幕
		img, err = screenshot.CaptureDisplay(0)
	}

	return img, err
}

// ImageClickExpr的Eval方法
func (i ImageClickExpr) Eval(ctx *ExecutionContext) interface{} {
	templatePath := i.TemplatePath.Eval(ctx).(string)

	// 获取置信度阈值
	confidence := 0.8
	if i.Confidence != nil {
		confidence = i.Confidence.Eval(ctx).(float64)
		if confidence < 0.1 {
			confidence = 0.1
		}
	}

	// 获取最大等待时间
	maxWait := 3.0
	if i.MaxWait != nil {
		maxWait = i.MaxWait.Eval(ctx).(float64)
		if maxWait > 30 {
			maxWait = 30
		}
	}

	// 获取点击次数
	clickCount := 1
	if i.ClickCount != nil {
		clickCount = int(i.ClickCount.Eval(ctx).(float64))
		if clickCount > 10 {
			clickCount = 10
		}
	}

	// 先尝试快速查找
	findExpr := ImageFindExpr{
		TemplatePath: String{templatePath},
		Region:       i.Region,
		Confidence:   Number{confidence},
		MaxWait:      Number{maxWait},
	}

	result := findExpr.Eval(ctx).(map[string]interface{})
	if !result["found"].(bool) {
		return fmt.Sprintf("图像未找到: %s", templatePath)
	}

	x := result["x"].(int)
	y := result["y"].(int)

	// 移动鼠标到图像中心
	robotgo.Move(x, y)
	time.Sleep(50 * time.Millisecond) // 等待鼠标移动

	// 执行点击
	var button string
	switch i.Button {
	case RIGHT:
		button = "right"
	case MIDDLE:
		button = "center"
	default:
		button = "left"
	}

	for c := 0; c < clickCount; c++ {
		robotgo.Click(button)
		if c < clickCount-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return fmt.Sprintf("点击成功: %s (%d, %d)，点击次数: %d", templatePath, x, y, clickCount)
}

// ImageWaitExpr的Eval方法
func (i ImageWaitExpr) Eval(ctx *ExecutionContext) interface{} {
	templatePath := i.TemplatePath.Eval(ctx).(string)

	// 1. 在循环开始前加载模板图像并处理错误
	templateImg, err := loadImage(templatePath)
	if err != nil {
		panic(fmt.Sprintf("无法加载模板图像 %s: %v", templatePath, err))
	}

	// 2. 获取并验证参数
	confidence := 0.8
	if i.Confidence != nil {
		confidence = i.Confidence.Eval(ctx).(float64)
		if confidence < 0.1 {
			confidence = 0.1
		}
		if confidence > 1.0 {
			confidence = 1.0
		}
	}

	maxWait := 10.0 // 默认超时设为10秒，这是关键修复！
	if i.MaxWait != nil {
		maxWait = i.MaxWait.Eval(ctx).(float64)
		if maxWait <= 0 {
			maxWait = 10.0
		} // 防止无效值
	}

	fmt.Printf("[Wait] 开始等待图像: %s, 置信度: %.2f, 超时: %.1f秒, 等待类型: %v\n",
		templatePath, confidence, maxWait, i.WaitForDisappear)

	matcher := &SimpleMatcher{}
	startTime := time.Now()
	lastLogTime := startTime
	checkInterval := 200 * time.Millisecond // 每次检查间隔200ms

	for attempt := 0; ; attempt++ {
		// 3. 强制超时检查 (关键修复点)
		if time.Since(startTime).Seconds() > maxWait {
			if i.WaitForDisappear {
				panic(fmt.Sprintf("等待图像消失超时: %s (%.1f秒后未消失)", templatePath, maxWait))
			} else {
				panic(fmt.Sprintf("等待图像出现超时: %s (%.1f秒内未找到)", templatePath, maxWait))
			}
		}

		// 4. 定期打印日志，避免"假死"
		if time.Since(lastLogTime).Seconds() > 2.0 { // 每2秒打印一次状态
			elapsed := time.Since(startTime).Seconds()
			fmt.Printf("[Wait] 仍在等待: %s, 已等待: %.1f秒, 尝试次数: %d\n",
				templatePath, elapsed, attempt)
			lastLogTime = time.Now()
		}

		// 5. 截图和匹配
		screenImg, err := captureScreen(i.Region, ctx)
		if err != nil {
			panic(fmt.Sprintf("截图失败: %v", err))
		}

		// 6. 动态性能优化：超时后降低匹配精度以提高速度
		currentConfidence := confidence
		if time.Since(startTime).Seconds() > maxWait/2 {
			// 等待时间过半后，使用更快的匹配模式
			currentConfidence = math.Max(0.3, confidence-0.2) // 最低降到0.3
			if attempt%5 == 0 {                               // 每5次尝试才匹配一次，进一步提速
				time.Sleep(checkInterval)
				continue
			}
		}

		// 7. 使用已加载的模板图像进行匹配
		result, err := matcher.FindTemplate(screenImg, templateImg, currentConfidence)
		if err != nil {
			panic(fmt.Sprintf("模板匹配失败: %v", err))
		}

		found := result.Found

		// 8. 根据等待类型判断结果
		if i.WaitForDisappear {
			if !found {
				elapsed := time.Since(startTime).Seconds()
				fmt.Printf("[Wait] 图像已消失: %s, 耗时: %.2f秒, 总尝试: %d\n",
					templatePath, elapsed, attempt)
				return map[string]interface{}{"status": "disappeared", "time": elapsed}
			}
		} else {
			if found {
				elapsed := time.Since(startTime).Seconds()
				fmt.Printf("[Wait] 图像已找到: %s, 位置: (%d, %d), 置信度: %.2f, 耗时: %.2f秒\n",
					templatePath, result.X, result.Y, result.Score, elapsed)
				return map[string]interface{}{
					"x":          result.X,
					"y":          result.Y,
					"width":      result.Width,
					"height":     result.Height,
					"confidence": result.Score,
					"found":      true,
					"time":       elapsed,
				}
			}
		}

		// 9. 等待下次检查
		time.Sleep(checkInterval)
	}
}

// ScreenshotExpr的Eval方法
func (s ScreenshotExpr) Eval(ctx *ExecutionContext) interface{} {
	screenImg, err := captureScreen(s.Region, ctx)
	if err != nil {
		panic(fmt.Sprintf("截图失败: %v", err))
	}

	if s.SavePath != nil {
		savePath := s.SavePath.Eval(ctx).(string)

		file, err := os.Create(savePath)
		if err != nil {
			panic(fmt.Sprintf("创建截图文件失败: %v", err))
		}
		defer file.Close()

		err = png.Encode(file, screenImg)
		if err != nil {
			panic(fmt.Sprintf("保存截图失败: %v", err))
		}

		return fmt.Sprintf("截图已保存: %s", savePath)
	}

	// 如果不保存，返回图像尺寸信息
	bounds := screenImg.Bounds()
	return map[string]interface{}{
		"width":  bounds.Dx(),
		"height": bounds.Dy(),
		"format": "image",
	}
}

// ImageExistsExpr的Eval方法
func (i ImageExistsExpr) Eval(ctx *ExecutionContext) interface{} {
	templatePath := i.TemplatePath.Eval(ctx).(string)

	confidence := 0.8
	if i.Confidence != nil {
		confidence = i.Confidence.Eval(ctx).(float64)
	}

	// 截图
	screenImg, err := captureScreen(i.Region, ctx)
	if err != nil {
		panic(fmt.Sprintf("截图失败: %v", err))
	}

	// 加载模板
	templateImg, err := loadImage(templatePath)
	if err != nil {
		panic(fmt.Sprintf("无法加载模板图像: %v", err))
	}

	// 转换为灰度
	screenGray := toGrayscale(screenImg)
	templateGray := toGrayscale(templateImg)

	// 进行匹配
	result, err := templateMatch(screenGray, templateGray, confidence)
	if err != nil {
		panic(fmt.Sprintf("模板匹配失败: %v", err))
	}

	return result.Found
}

// 解析区域参数
func (p *Parser) parseRegion() *RectRegion {
	p.eat(REGION)
	p.eat(LPAREN)
	x := p.parseExpr()
	p.eat(COMMA)
	y := p.parseExpr()
	p.eat(COMMA)
	width := p.parseExpr()
	p.eat(COMMA)
	height := p.parseExpr()
	p.eat(RPAREN)

	return &RectRegion{x, y, width, height}
}

// 解析置信度参数
func (p *Parser) parseConfidence() ASTNode {
	p.eat(CONFIDENCE)
	p.eat(ASSIGN)
	return p.parseExpr()
}

// 解析图像查找
func (p *Parser) parseImageFind() ASTNode {
	p.eat(IMAGE_FIND)
	p.eat(LPAREN)
	pathExpr := p.parseExpr()
	p.eat(RPAREN)

	var region *RectRegion
	var confidence, maxWait ASTNode

	// 解析可选参数
	for p.curr.Type != SEMI && p.curr.Type != EOF {
		if p.curr.Type == REGION {
			region = p.parseRegion()
		} else if p.curr.Type == CONFIDENCE {
			confidence = p.parseConfidence()
		} else if p.curr.Type == TIME && p.peek.Type == DOT {
			p.eat(TIME)
			p.eat(DOT)
			if p.curr.Type == SLEEP {
				p.eat(SLEEP)
				p.eat(LPAREN)
				maxWait = p.parseExpr()
				p.eat(RPAREN)
			} else {
				panic("期望 sleep 参数")
			}
		} else {
			break
		}
	}

	return ImageFindExpr{
		TemplatePath: pathExpr,
		Region:       region,
		Confidence:   confidence,
		MaxWait:      maxWait,
	}
}

// 解析图像点击
func (p *Parser) parseImageClick() ASTNode {
	p.eat(IMAGE_CLICK)
	p.eat(LPAREN)
	pathExpr := p.parseExpr()
	p.eat(RPAREN)

	var region *RectRegion
	var confidence, maxWait, clickCount ASTNode
	var button TokenType = LEFT

	// 解析可选参数
	for p.curr.Type != SEMI && p.curr.Type != EOF {
		if p.curr.Type == REGION {
			region = p.parseRegion()
		} else if p.curr.Type == CONFIDENCE {
			confidence = p.parseConfidence()
		} else if p.curr.Type == LEFT || p.curr.Type == RIGHT || p.curr.Type == MIDDLE {
			button = p.curr.Type
			p.eat(button)
		} else if p.curr.Type == COMMA {
			// 检查是否有点击次数参数
			p.eat(COMMA)
			if p.curr.Type == INT || p.curr.Type == FLOAT {
				clickCount = p.parseExpr()
			}
		} else if p.curr.Type == TIME && p.peek.Type == DOT {
			p.eat(TIME)
			p.eat(DOT)
			if p.curr.Type == SLEEP {
				p.eat(SLEEP)
				p.eat(LPAREN)
				maxWait = p.parseExpr()
				p.eat(RPAREN)
			}
		} else {
			break
		}
	}

	return ImageClickExpr{
		TemplatePath: pathExpr,
		Region:       region,
		Confidence:   confidence,
		MaxWait:      maxWait,
		Button:       button,
		ClickCount:   clickCount,
	}
}

// 解析图像等待
func (p *Parser) parseImageWait(waitForDisappear bool) ASTNode {
	if waitForDisappear {
		p.eat(IMAGE_WAIT_DISAPPEAR)
	} else {
		p.eat(IMAGE_WAIT)
	}

	p.eat(LPAREN)
	pathExpr := p.parseExpr()
	p.eat(RPAREN)

	var region *RectRegion
	var confidence, maxWait ASTNode

	for p.curr.Type != SEMI && p.curr.Type != EOF {
		if p.curr.Type == REGION {
			region = p.parseRegion()
		} else if p.curr.Type == CONFIDENCE {
			confidence = p.parseConfidence()
		} else if p.curr.Type == TIME && p.peek.Type == DOT {
			p.eat(TIME)
			p.eat(DOT)
			if p.curr.Type == SLEEP {
				p.eat(SLEEP)
				p.eat(LPAREN)
				maxWait = p.parseExpr()
				p.eat(RPAREN)
			}
		} else {
			break
		}
	}

	return ImageWaitExpr{
		TemplatePath:     pathExpr,
		Region:           region,
		Confidence:       confidence,
		MaxWait:          maxWait,
		WaitForDisappear: waitForDisappear,
	}
}

// 解析图像存在检查
func (p *Parser) parseImageExists() ASTNode {
	p.eat(IMAGE_EXISTS)
	p.eat(LPAREN)
	pathExpr := p.parseExpr()
	p.eat(RPAREN)

	var region *RectRegion
	var confidence ASTNode

	for p.curr.Type != SEMI && p.curr.Type != EOF {
		if p.curr.Type == REGION {
			region = p.parseRegion()
		} else if p.curr.Type == CONFIDENCE {
			confidence = p.parseConfidence()
		} else {
			break
		}
	}

	return ImageExistsExpr{
		TemplatePath: pathExpr,
		Region:       region,
		Confidence:   confidence,
	}
}

// 解析截图
func (p *Parser) parseScreenshot() ASTNode {
	p.eat(SCREENSHOT)
	p.eat(LPAREN)

	var savePath ASTNode
	if p.curr.Type != RPAREN {
		savePath = p.parseExpr()
	}
	p.eat(RPAREN)

	var region *RectRegion

	for p.curr.Type != SEMI && p.curr.Type != EOF {
		if p.curr.Type == REGION {
			region = p.parseRegion()
		} else {
			break
		}
	}

	return ScreenshotExpr{
		SavePath: savePath,
		Region:   region,
	}
}

// 图像匹配结果结构
type MatchResult struct {
	X      int
	Y      int
	Width  int
	Height int
	Score  float64
	Found  bool
}

// 图像匹配器接口
type ImageMatcher interface {
	FindTemplate(screen, template image.Image, threshold float64) (MatchResult, error)
}

// 简单模板匹配器（基于像素比较）
type SimpleMatcher struct{}

func (m *SimpleMatcher) FindTemplate(screen, template image.Image, threshold float64) (MatchResult, error) {
	screenBounds := screen.Bounds()
	templateBounds := template.Bounds()

	sw, sh := screenBounds.Dx(), screenBounds.Dy()
	tw, th := templateBounds.Dx(), templateBounds.Dy()

	if sw < tw || sh < th {
		return MatchResult{Found: false}, errors.New("模板图像比屏幕图像大")
	}

	// 将图像缩小以提高性能
	scaleFactor := calculateOptimalScale(sw, sh, tw, th)
	scaledScreen := screen
	scaledTemplate := template

	if scaleFactor < 1.0 {
		scaledScreen = imaging.Resize(screen, int(float64(sw)*scaleFactor), 0, imaging.Lanczos)
		scaledTemplate = imaging.Resize(template, int(float64(tw)*scaleFactor), 0, imaging.Lanczos)
	}

	// 转换到HSV颜色空间，减少光照影响
	screenHSV := convertToHSV(scaledScreen)
	templateHSV := convertToHSV(scaledTemplate)

	bestScore := -1.0
	bestX, bestY := 0, 0

	sw2, sh2 := scaledScreen.Bounds().Dx(), scaledScreen.Bounds().Dy()
	tw2, th2 := scaledTemplate.Bounds().Dx(), scaledTemplate.Bounds().Dy()

	// 使用步进搜索，减少搜索点
	step := 2
	for y := 0; y <= sh2-th2; y += step {
		for x := 0; x <= sw2-tw2; x += step {
			score := m.calculateSimilarity(screenHSV, templateHSV, x, y, tw2, th2)

			if score > bestScore {
				bestScore = score
				bestX = x
				bestY = y
			}

			// 如果已经找到足够好的匹配，提前返回
			if bestScore >= threshold {
				break
			}
		}
		if bestScore >= threshold {
			break
		}
	}

	// 调整坐标到原始尺寸
	if scaleFactor < 1.0 {
		bestX = int(float64(bestX) / scaleFactor)
		bestY = int(float64(bestY) / scaleFactor)
		tw = int(float64(tw) / scaleFactor)
		th = int(float64(th) / scaleFactor)
	}

	if bestScore >= threshold {
		return MatchResult{
			X:      bestX + tw/2,
			Y:      bestY + th/2,
			Width:  tw,
			Height: th,
			Score:  bestScore,
			Found:  true,
		}, nil
	}

	return MatchResult{Found: false}, nil
}

// 计算相似度（使用结构相似性SSIM的简化版本）
func (m *SimpleMatcher) calculateSimilarity(screen, template []float64, x, y, tw, th int) float64 {
	var sumScreen, sumTemplate, sumScreen2, sumTemplate2, sumProduct float64

	count := 0
	for ty := 0; ty < th; ty++ {
		for tx := 0; tx < tw; tx++ {
			sIdx := (y+ty)*tw*3 + (x+tx)*3
			tIdx := ty*tw*3 + tx*3

			// 比较RGB三个通道
			for c := 0; c < 3; c++ {
				sVal := screen[sIdx+c]
				tVal := template[tIdx+c]

				sumScreen += sVal
				sumTemplate += tVal
				sumScreen2 += sVal * sVal
				sumTemplate2 += tVal * tVal
				sumProduct += sVal * tVal
				count++
			}
		}
	}

	n := float64(count)
	if n == 0 {
		return 0
	}

	// 计算相关系数
	numerator := n*sumProduct - sumScreen*sumTemplate
	denom1 := n*sumScreen2 - sumScreen*sumScreen
	denom2 := n*sumTemplate2 - sumTemplate*sumTemplate

	if denom1 == 0 || denom2 == 0 {
		return 0
	}

	return numerator / math.Sqrt(denom1*denom2)
}

// 转换为HSV颜色空间
func convertToHSV(img image.Image) []float64 {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	hsv := make([]float64, width*height*3)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			rf := float64(r) / 65535.0
			gf := float64(g) / 65535.0
			bf := float64(b) / 65535.0

			// RGB to HSV
			maxVal := math.Max(math.Max(rf, gf), bf)
			minVal := math.Min(math.Min(rf, gf), bf)
			delta := maxVal - minVal

			var h, s, v float64
			v = maxVal

			if maxVal > 0 {
				s = delta / maxVal
			} else {
				s = 0
			}

			if delta > 0 {
				if maxVal == rf {
					h = (gf - bf) / delta
				} else if maxVal == gf {
					h = 2 + (bf-rf)/delta
				} else {
					h = 4 + (rf-gf)/delta
				}

				h *= 60
				if h < 0 {
					h += 360
				}
			}

			idx := (y*width + x) * 3
			hsv[idx] = h / 360.0
			hsv[idx+1] = s
			hsv[idx+2] = v
		}
	}

	return hsv
}

// 计算最优缩放比例
func calculateOptimalScale(screenWidth, screenHeight, templateWidth, templateHeight int) float64 {
	maxDimension := math.Max(float64(screenWidth), float64(screenHeight))

	if maxDimension > 2000 {
		return 0.5
	} else if maxDimension > 1000 {
		return 0.75
	}
	return 1.0
}

// 优化后的ImageFindExpr的Eval方法
func (i ImageFindExpr) Eval(ctx *ExecutionContext) interface{} {
	templatePath := i.TemplatePath.Eval(ctx).(string)

	// 加载模板图像
	templateImg, err := loadImage(templatePath)
	if err != nil {
		panic(fmt.Sprintf("无法加载模板图像: %v", err))
	}

	// 获取置信度阈值
	confidence := 0.8
	if i.Confidence != nil {
		confidence = i.Confidence.Eval(ctx).(float64)
		if confidence < 0.1 {
			confidence = 0.1
		}
	}

	// 获取最大等待时间
	maxWait := 3.0 // 默认3秒
	if i.MaxWait != nil {
		maxWait = i.MaxWait.Eval(ctx).(float64)
		if maxWait > 30 {
			maxWait = 30
		}
	}

	matcher := &SimpleMatcher{}
	startTime := time.Now()

	for attempt := 0; attempt < 50; attempt++ {
		// 截图
		screenImg, err := captureScreen(i.Region, ctx)
		if err != nil {
			panic(fmt.Sprintf("截图失败: %v", err))
		}

		// 进行模板匹配
		result, err := matcher.FindTemplate(screenImg, templateImg, confidence)
		if err != nil {
			panic(fmt.Sprintf("模板匹配失败: %v", err))
		}

		if result.Found {
			fmt.Printf("[图像识别] 找到图像，得分: %.2f，位置: (%d, %d)，耗时: %v\n",
				result.Score, result.X, result.Y, time.Since(startTime))

			return map[string]interface{}{
				"x":          result.X,
				"y":          result.Y,
				"width":      result.Width,
				"height":     result.Height,
				"confidence": result.Score,
				"found":      true,
			}
		}

		// 检查是否超时
		if time.Since(startTime).Seconds() > maxWait {
			break
		}

		// 等待一段时间再试
		sleepTime := 50 * time.Millisecond
		if attempt == 0 {
			sleepTime = 100 * time.Millisecond
		}
		time.Sleep(sleepTime)
	}

	fmt.Printf("[图像识别] 未找到图像: %s，耗时: %v，阈值: %.2f\n",
		templatePath, time.Since(startTime), confidence)

	return map[string]interface{}{"found": false}
}

// 测试结果结构
type TestResult struct {
	Name     string
	Passed   bool
	Message  string
	Duration time.Duration
}

// 测试块 AST 节点
type TestBlock struct {
	Name  string
	Block []ASTNode
}

// 断言语句 AST 节点
type AssertStmt struct {
	Condition ASTNode
	Message   ASTNode
}

// 运行测试 AST 节点
type RunTestsStmt struct{}

// 全局测试管理器
type TestManager struct {
	tests   map[string]*TestBlock
	results []TestResult
	mu      sync.RWMutex
}

var testManager = &TestManager{
	tests:   make(map[string]*TestBlock),
	results: make([]TestResult, 0),
}

// TestBlock 的 Eval 方法 - 注册测试但不执行
func (t TestBlock) Eval(ctx *ExecutionContext) interface{} {
	testManager.mu.Lock()
	defer testManager.mu.Unlock()

	// 检查测试名是否已存在
	if _, exists := testManager.tests[t.Name]; exists {
		panic(fmt.Sprintf("测试名称重复: %s", t.Name))
	}

	testManager.tests[t.Name] = &TestBlock{
		Name:  t.Name,
		Block: t.Block,
	}

	return nil
}

// AssertStmt 的 Eval 方法
func (a AssertStmt) Eval(ctx *ExecutionContext) interface{} {
	condition := a.Condition.Eval(ctx)

	// 检查条件是否为真
	if !isTruthy(condition) {
		var msg string
		if a.Message != nil {
			msg = a.Message.Eval(ctx).(string)
		} else {
			msg = fmt.Sprintf("断言失败: %v", condition)
		}

		// 在测试环境中，抛出特定的错误
		if _, isTestEnv := ctx.env["__test_mode"]; isTestEnv {
			panic(msg)
		}

		// 非测试环境中，输出警告但不中断程序
		fmt.Printf("警告: %s\n", msg)
	}

	return condition
}

// RunTestsStmt 的 Eval 方法
func (r RunTestsStmt) Eval(ctx *ExecutionContext) interface{} {
	testManager.mu.Lock()
	defer testManager.mu.Unlock()

	if len(testManager.tests) == 0 {
		fmt.Println("没有找到测试")
		return nil
	}

	fmt.Println("\n[DEBUG]:")

	total := 0
	passed := 0
	failed := 0

	// 按测试名称排序
	var testNames []string
	for name := range testManager.tests {
		testNames = append(testNames, name)
	}
	sort.Strings(testNames)

	for _, name := range testNames {
		test := testManager.tests[name]
		total++

		// 创建测试专用环境
		testEnv := &ExecutionContext{
			env:        make(map[string]VarInfo),
			deferStack: make(map[int][]DeferItem),
			wg:         &sync.WaitGroup{},
			globalEnv:  ctx.globalEnv,
			globalMu:   ctx.globalMu,
		}

		// 设置测试模式标志
		testEnv.env["__test_mode"] = VarInfo{
			Type:  BoolType,
			Value: true,
		}

		// 复制父环境的变量（不包括测试标志）
		for k, v := range ctx.env {
			if k != "__test_mode" {
				testEnv.env[k] = v
			}
		}

		// 运行测试并计时
		startTime := time.Now()
		result := TestResult{
			Name:     name,
			Passed:   true,
			Message:  "测试通过",
			Duration: 0,
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					result.Passed = false
					result.Message = fmt.Sprintf("测试失败: %v", r)
					failed++
				} else {
					passed++
				}
				result.Duration = time.Since(startTime)
			}()

			for _, stmt := range test.Block {
				stmt.Eval(testEnv)
			}
		}()

		testManager.results = append(testManager.results, result)

		// 显示测试结果
		if result.Passed {
			fmt.Printf("✓ %-40s [%v]\n", name, result.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("✗ %-40s [%v]\n", name, result.Duration.Round(time.Millisecond))
			fmt.Printf("  错误: %s\n", result.Message)
		}
	}

	// 打印总结
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("总计: %d, 通过: %d, 失败: %d, 成功率: %.1f%%\n",
		total, passed, failed, float64(passed)/float64(total)*100)
	fmt.Println(strings.Repeat("=", 60))

	return nil
}

// 解析测试块
func (p *Parser) parseTestBlock() ASTNode {
	p.eat(TEST)

	// 获取测试名称
	nameToken := p.curr
	if nameToken.Type != STRING {
		panic("测试名称必须是字符串")
	}
	p.eat(STRING)

	// 解析测试体
	body := p.parseBlock()

	return TestBlock{
		Name:  nameToken.Value,
		Block: body,
	}
}

// 解析断言语句
func (p *Parser) parseAssertStmt() ASTNode {
	p.eat(ASSERT)
	p.eat(LPAREN)

	condition := p.parseExpr()

	var message ASTNode
	if p.curr.Type == COMMA {
		p.eat(COMMA)
		message = p.parseExpr()
	}

	p.eat(RPAREN)
	p.eat(SEMI)

	return AssertStmt{
		Condition: condition,
		Message:   message,
	}
}

// 解析运行测试语句
func (p *Parser) parseRunTestsStmt() ASTNode {
	p.eat(RUN_TESTS)
	p.eat(SEMI)
	return RunTestsStmt{}
}

// 获取测试报告
func GetTestReport() string {
	testManager.mu.RLock()
	defer testManager.mu.RUnlock()

	if len(testManager.results) == 0 {
		return "没有测试结果"
	}

	var report strings.Builder
	report.WriteString("测试报告\n")
	report.WriteString(strings.Repeat("=", 60) + "\n")

	passed := 0
	total := len(testManager.results)

	for _, result := range testManager.results {
		if result.Passed {
			passed++
			report.WriteString(fmt.Sprintf("✓ %s (%.2fms)\n", result.Name, result.Duration.Seconds()*1000))
		} else {
			report.WriteString(fmt.Sprintf("✗ %s (%.2fms)\n", result.Name, result.Duration.Seconds()*1000))
			report.WriteString(fmt.Sprintf("  错误: %s\n", result.Message))
		}
	}

	report.WriteString(strings.Repeat("-", 60) + "\n")
	report.WriteString(fmt.Sprintf("总计: %d, 通过: %d, 失败: %d, 成功率: %.1f%%\n",
		total, passed, total-passed, float64(passed)/float64(total)*100))

	return report.String()
}

// ==================== 新增：环境安装/卸载功能 ====================

// askInstallExtension 在安装完成后询问是否安装扩展包 io3.tuka
// 扩展包从当前工作目录读取，使用与 `bocchi -install <包名>` 相同的安装流程
// 若 io3 已在安装列表中，则直接覆盖原文件
func askInstallExtension() {
	const extPkgName = "io3"
	const extFileName = "io3.tuka"

	fmt.Println()
	fmt.Println("── 扩展安装 ──")
	fmt.Printf("检测到可选扩展: %s（Bocchi 标准库扩展）\n", extFileName)
	fmt.Print("是否安装该扩展？(y/N): ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Println("已跳过扩展安装。")
		return
	}

	// 检查当前目录下是否存在扩展文件
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	extPath := filepath.Join(cwd, extFileName)

	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		fmt.Printf("⚠️  当前目录下未找到扩展文件 '%s'，已跳过。\n", extPath)
		return
	}

	// 检查 io3 是否已在安装列表中
	initPackages()
	config := loadPackageConfig()
	alreadyInstalled := false
	for _, pkg := range config.InstalledPackages {
		if pkg == extPkgName {
			alreadyInstalled = true
			break
		}
	}

	// 覆盖模式：已安装则直接用新文件替换原有文件
	if alreadyInstalled {
		fmt.Printf("检测到扩展 '%s' 已安装，将覆盖原文件\n", extPkgName)

		destPath := filepath.Join(getPackagesDir(), extPkgName+".bocchi")
		if err := copyFile(extPath, destPath); err != nil {
			fmt.Printf("❌ 覆盖扩展失败: %v\n", err)
			return
		}

		// 若存在扩展信息文件，一并覆盖
		srcInfo := strings.TrimSuffix(extPath, filepath.Ext(extPath)) + ".info.json"
		if _, err := os.Stat(srcInfo); err == nil {
			destInfo := filepath.Join(getPackagesDir(), extPkgName+".info.json")
			if err := copyFile(srcInfo, destInfo); err != nil {
				fmt.Printf("⚠️  覆盖扩展信息失败: %v\n", err)
			}
		}

		fmt.Printf("✅ 扩展 '%s' 已覆盖更新\n", extPkgName)
		return
	}

	// 全新安装：走原有 installPackage 流程
	fmt.Printf("正在安装扩展: %s\n", extPath)
	installPackage(extPath)
}

// installEnvironment 安装Bocchi环境
func installEnvironment() {
	fmt.Println("开始安装Bocchi环境...")

	// 获取当前可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("❌ 获取可执行文件路径失败: %v\n", err)
		return
	}

	// 确定安装目录
	var installDir string
	switch runtime.GOOS {
	case "windows":
		installDir = filepath.Join(os.Getenv("ProgramFiles"), "Bocchi")
		fmt.Printf("安装目录: %s\n", installDir)

		// 创建安装目录
		if err := os.MkdirAll(installDir, 0755); err != nil {
			fmt.Printf("❌ 创建安装目录失败: %v\n", err)
			return
		}

		// 复制可执行文件
		destExe := filepath.Join(installDir, "bocchi.exe")
		if err := copyFile(exePath, destExe); err != nil {
			fmt.Printf("❌ 复制可执行文件失败: %v\n", err)
			return
		}

		// 添加系统PATH
		if err := addToPath(installDir); err != nil {
			fmt.Printf("❌ 添加系统PATH失败: %v\n", err)
			return
		}

		// 创建文件关联
		createFileAssociation(installDir)

		// 创建桌面快捷方式
		createDesktopShortcut(installDir)

		// 创建开始菜单快捷方式
		createStartMenuShortcut(installDir)

	case "darwin": // macOS
		installDir = "/usr/local/bin"
		fmt.Printf("安装目录: %s\n", installDir)

		// 检查目录是否存在
		if _, err := os.Stat(installDir); os.IsNotExist(err) {
			if err := os.MkdirAll(installDir, 0755); err != nil {
				fmt.Printf("❌ 创建安装目录失败: %v\n", err)
				return
			}
		}

		// 创建符号链接
		destExe := filepath.Join(installDir, "bocchi")
		if _, err := os.Stat(destExe); err == nil {
			os.Remove(destExe)
		}

		if err := os.Symlink(exePath, destExe); err != nil {
			fmt.Printf("❌ 创建符号链接失败: %v\n", err)
			return
		}

		// 设置可执行权限
		if err := os.Chmod(destExe, 0755); err != nil {
			fmt.Printf("❌ 设置可执行权限失败: %v\n", err)
			return
		}

	case "linux":
		installDir = "/usr/local/bin"
		fmt.Printf("安装目录: %s\n", installDir)

		// 检查目录是否存在
		if _, err := os.Stat(installDir); os.IsNotExist(err) {
			if err := os.MkdirAll(installDir, 0755); err != nil {
				fmt.Printf("❌ 创建安装目录失败: %v\n", err)
				return
			}
		}

		// 复制可执行文件
		destExe := filepath.Join(installDir, "bocchi")
		if err := copyFile(exePath, destExe); err != nil {
			fmt.Printf("❌ 复制可执行文件失败: %v\n", err)
			return
		}

		// 设置可执行权限
		if err := os.Chmod(destExe, 0755); err != nil {
			fmt.Printf("❌ 设置可执行权限失败: %v\n", err)
			return
		}

		// 创建桌面文件（用于应用程序菜单）
		createLinuxDesktopFile(installDir)

	default:
		fmt.Printf("❌ 不支持的操作系统: %s\n", runtime.GOOS)
		return
	}

	fmt.Println("✅ Bocchi环境安装成功！")
	fmt.Println("\n现在您可以在任何目录下使用 'bocchi' 命令了。")

	// 显示使用示例
	fmt.Println("\n使用示例：")
	fmt.Println("  1. 执行脚本: bocchi script.bocchi")
	fmt.Println("  2. 启动REPL: bocchi -repl")
	fmt.Println("  3. 查看帮助: bocchi -help")

	if runtime.GOOS == "windows" {
		fmt.Println("  4. 您现在可以将 .bocchi 文件与Bocchi关联")
	}

}

// uninstallEnvironment 卸载Bocchi环境
func uninstallEnvironment() {
	fmt.Println("开始卸载Bocchi环境...")

	switch runtime.GOOS {
	case "windows":
		// 卸载目录
		installDir := filepath.Join(os.Getenv("ProgramFiles"), "Bocchi")

		// 移除PATH中的目录
		if err := removeFromPath(installDir); err != nil {
			fmt.Printf("⚠️  从PATH移除失败: %v\n", err)
		}

		// 移除文件关联
		removeFileAssociation()

		// 移除开始菜单快捷方式
		removeStartMenuShortcut()

		// 移除安装目录
		if _, err := os.Stat(installDir); err == nil {
			if err := os.RemoveAll(installDir); err != nil {
				fmt.Printf("⚠️  移除安装目录失败: %v\n", err)
			}
		}

	case "darwin", "linux":
		// 移除符号链接或可执行文件
		destExe := "/usr/local/bin/bocchi"
		if _, err := os.Stat(destExe); err == nil {
			if err := os.Remove(destExe); err != nil {
				fmt.Printf("⚠️  移除可执行文件失败: %v\n", err)
			}
		}

		if runtime.GOOS == "linux" {
			// 移除桌面文件
			removeLinuxDesktopFile()
		}
	}

	fmt.Println("✅ Bocchi环境已卸载。")
}

// checkEnvironmentStatus 检查环境状态
func checkEnvironmentStatus() {
	fmt.Println("检查Bocchi环境状态...")

	var status []string

	// 检查是否在PATH中
	exeName := "bocchi"
	if runtime.GOOS == "windows" {
		exeName = "bocchi.exe"
	}

	path, err := exec.LookPath(exeName)
	if err == nil {
		status = append(status, fmt.Sprintf("✅ 在PATH中找到: %s", path))
	} else {
		status = append(status, "❌ 未在PATH中找到")
	}

	// 检查安装目录
	switch runtime.GOOS {
	case "windows":
		installDir := filepath.Join(os.Getenv("ProgramFiles"), "Bocchi")
		if _, err := os.Stat(installDir); err == nil {
			status = append(status, fmt.Sprintf("✅ 安装目录存在: %s", installDir))

			// 检查文件关联
			if checkFileAssociation() {
				status = append(status, "✅ 文件关联已设置")
			} else {
				status = append(status, "❌ 文件关联未设置")
			}
		} else {
			status = append(status, fmt.Sprintf("❌ 安装目录不存在: %s", installDir))
		}

	case "darwin", "linux":
		destExe := "/usr/local/bin/bocchi"
		if _, err := os.Stat(destExe); err == nil {
			status = append(status, fmt.Sprintf("✅ 可执行文件存在: %s", destExe))

			// 检查权限
			info, err := os.Stat(destExe)
			if err == nil && info.Mode()&0111 != 0 {
				status = append(status, "✅ 可执行权限已设置")
			} else {
				status = append(status, "❌ 可执行权限未设置")
			}
		} else {
			status = append(status, fmt.Sprintf("❌ 可执行文件不存在: %s", destExe))
		}
	}

	// 显示状态
	for _, s := range status {
		fmt.Println(s)
	}
}

// ==================== Windows特定函数 ====================

// createFileAssociation 创建文件关联
func createFileAssociation(installDir string) {
	if runtime.GOOS != "windows" {
		return
	}

	fmt.Println("创建.bocchi文件关联...")

	exePath := filepath.Join(installDir, "bocchi.exe")

	// 使用注册表创建文件关联
	k, err := registry.OpenKey(registry.CLASSES_ROOT, `.bocchi`, registry.ALL_ACCESS)
	if err != nil {
		// 如果键不存在，创建它
		k, _, err = registry.CreateKey(registry.CLASSES_ROOT, `.bocchi`, registry.ALL_ACCESS)
		if err != nil {
			fmt.Printf("⚠️  创建文件关联失败: %v\n", err)
			return
		}
	}
	defer k.Close()

	// 设置默认值
	k.SetStringValue("", "BocchiScript")

	// 创建BocchiScript键
	appKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, `BocchiScript`, registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("⚠️  创建BocchiScript键失败: %v\n", err)
		return
	}
	defer appKey.Close()

	appKey.SetStringValue("", "Bocchi Script File")

	// 创建DefaultIcon键
	iconKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, `BocchiScript\DefaultIcon`, registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("⚠️  创建DefaultIcon键失败: %v\n", err)
		return
	}
	defer iconKey.Close()

	iconKey.SetStringValue("", fmt.Sprintf(`%s,0`, exePath))

	// 创建shell键
	shellKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, `BocchiScript\shell\open\command`, registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("⚠️  创建shell键失败: %v\n", err)
		return
	}
	defer shellKey.Close()

	shellKey.SetStringValue("", fmt.Sprintf(`"%s" "%%1"`, exePath))

	fmt.Println("✅ 文件关联创建成功")
}

// removeFileAssociation 移除文件关联
func removeFileAssociation() {
	if runtime.GOOS != "windows" {
		return
	}

	fmt.Println("移除.bocchi文件关联...")

	// 删除.bocchi键
	registry.DeleteKey(registry.CLASSES_ROOT, `.bocchi`)

	// 删除BocchiScript键
	registry.DeleteKey(registry.CLASSES_ROOT, `BocchiScript`)

	fmt.Println("✅ 文件关联已移除")
}

// checkFileAssociation 检查文件关联
func checkFileAssociation() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	_, err := registry.OpenKey(registry.CLASSES_ROOT, `.bocchi`, registry.READ)
	return err == nil
}

// removeFromPath 从PATH中移除目录
func removeFromPath(dir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	pathVal, err := getSystemPath()
	if err != nil {
		return err
	}

	paths := strings.Split(pathVal, ";")
	var newPaths []string
	found := false

	for _, p := range paths {
		if !strings.EqualFold(p, dir) {
			newPaths = append(newPaths, p)
		} else {
			found = true
		}
	}

	if !found {
		return nil
	}

	newPath := strings.Join(newPaths, ";")

	key := `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer k.Close()

	if err := k.SetStringValue("Path", newPath); err != nil {
		return fmt.Errorf("更新PATH失败: %v", err)
	}

	broadcastEnvChange()

	return nil
}

// createStartMenuShortcut 创建开始菜单快捷方式
func createStartMenuShortcut(installDir string) {
	if runtime.GOOS != "windows" {
		return
	}

	fmt.Println("创建开始菜单快捷方式...")

	// 获取开始菜单目录
	startMenu := getStartMenuPath()
	if startMenu == "" {
		fmt.Println("⚠️  无法获取开始菜单目录")
		return
	}

	bocchiStartMenu := filepath.Join(startMenu, "Bocchi")
	if err := os.MkdirAll(bocchiStartMenu, 0755); err != nil {
		fmt.Printf("⚠️  创建开始菜单目录失败: %v\n", err)
		return
	}

	shortcutPath := filepath.Join(bocchiStartMenu, "Bocchi.lnk")
	targetPath := filepath.Join(installDir, "bocchi.exe")

	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf(`$WshShell = New-Object -ComObject WScript.Shell; 
$Shortcut = $WshShell.CreateShortcut('%s'); 
$Shortcut.TargetPath = '%s'; 
$Shortcut.WorkingDirectory = '%s'; 
$Shortcut.Description = 'Bocchi脚本解释器'; 
$Shortcut.Save();`,
			strings.ReplaceAll(shortcutPath, `\`, `\\`),
			strings.ReplaceAll(targetPath, `\`, `\\`),
			strings.ReplaceAll(installDir, `\`, `\\`)))

	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  创建开始菜单快捷方式失败: %v\n", err)
	} else {
		fmt.Println("✅ 开始菜单快捷方式创建成功")
	}
}

// removeStartMenuShortcut 移除开始菜单快捷方式
func removeStartMenuShortcut() {
	if runtime.GOOS != "windows" {
		return
	}

	fmt.Println("移除开始菜单快捷方式...")

	startMenu := getStartMenuPath()
	if startMenu == "" {
		return
	}

	bocchiStartMenu := filepath.Join(startMenu, "Bocchi")
	if _, err := os.Stat(bocchiStartMenu); err == nil {
		if err := os.RemoveAll(bocchiStartMenu); err != nil {
			fmt.Printf("⚠️  移除开始菜单目录失败: %v\n", err)
		}
	}

	fmt.Println("✅ 开始菜单快捷方式已移除")
}

// getStartMenuPath 获取开始菜单路径
func getStartMenuPath() string {
	if runtime.GOOS != "windows" {
		return ""
	}

	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\Shell Folders`,
		registry.READ)
	if err != nil {
		return ""
	}
	defer key.Close()

	startMenu, _, err := key.GetStringValue("Programs")
	if err != nil {
		return ""
	}

	return startMenu
}

// ==================== Linux特定函数 ====================

// createLinuxDesktopFile 创建Linux桌面文件
func createLinuxDesktopFile(installDir string) {
	if runtime.GOOS != "linux" {
		return
	}

	fmt.Println("创建Linux桌面文件...")

	desktopFile := `[Desktop Entry]
Name=Bocchi Interpreter
Comment=Bocchi Script Language Interpreter
Exec=bocchi %f
Icon=terminal
Terminal=true
Type=Application
Categories=Development;Utility;
MimeType=text/plain;application/x-bocchi;
`

	desktopPath := "/usr/share/applications/bocchi.desktop"
	if err := os.WriteFile(desktopPath, []byte(desktopFile), 0644); err != nil {
		fmt.Printf("⚠️  创建桌面文件失败: %v\n", err)
	} else {
		fmt.Println("✅ Linux桌面文件创建成功")
	}
}

// removeLinuxDesktopFile 移除Linux桌面文件
func removeLinuxDesktopFile() {
	if runtime.GOOS != "linux" {
		return
	}

	fmt.Println("移除Linux桌面文件...")

	desktopPath := "/usr/share/applications/bocchi.desktop"
	if _, err := os.Stat(desktopPath); err == nil {
		if err := os.Remove(desktopPath); err != nil {
			fmt.Printf("⚠️  移除桌面文件失败: %v\n", err)
		}
	}

	fmt.Println("✅ Linux桌面文件已移除")
}

// ==================== 添加：os.Args 支持 ====================

// OsArgsExpr 结构体
type OsArgsExpr struct{}

func (o OsArgsExpr) Eval(ctx *ExecutionContext) interface{} {
	// 返回命令行参数列表
	args := make([]interface{}, len(ctx.Args))
	for i, arg := range ctx.Args {
		args[i] = arg
	}
	return args
}

// OsGetArgExpr 获取特定位置的参数
type OsGetArgExpr struct {
	Index ASTNode // 索引，从0开始
}

func (o OsGetArgExpr) Eval(ctx *ExecutionContext) interface{} {
	indexVal := o.Index.Eval(ctx)

	var index int
	switch v := indexVal.(type) {
	case float64:
		index = int(v)
	case int:
		index = v
	default:
		panic("参数索引必须是数字")
	}

	if index < 0 || index >= len(ctx.Args) {
		return ""
	}

	return ctx.Args[index]
}

// OsArgcExpr 获取参数数量
type OsArgcExpr struct{}

func (o OsArgcExpr) Eval(ctx *ExecutionContext) interface{} {
	return float64(len(ctx.Args))
}

// 在AST节点定义部分添加（可以在PopupStmt后面添加）
type TitleStmt struct {
	Expr ASTNode
}

func (t TitleStmt) Eval(ctx *ExecutionContext) interface{} {
	titleVal := t.Expr.Eval(ctx)
	var titleStr string

	switch v := titleVal.(type) {
	case string:
		titleStr = v
	default:
		titleStr = fmt.Sprintf("%v", v)
	}

	setWindowTitle(titleStr)
	return nil
}

// 在showWindowsPopup函数附近添加
func setWindowTitle(title string) {
	if runtime.GOOS == "windows" {
		setWindowsConsoleTitle(title)
	} else {
		setUnixTerminalTitle(title)
	}
}

// Windows系统设置控制台标题
func setWindowsConsoleTitle(title string) {
	// 使用kernel32.dll的SetConsoleTitleW函数
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleTitle := kernel32.NewProc("SetConsoleTitleW")

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	setConsoleTitle.Call(uintptr(unsafe.Pointer(titlePtr)))
}

// Unix/Linux/Mac系统设置终端标题
func setUnixTerminalTitle(title string) {
	// 使用ANSI转义序列设置终端标题
	fmt.Printf("\033]0;%s\007", title)
}

func (p *Parser) parseTitleStmt() ASTNode {
	p.eat(TITLE)
	p.eat(LPAREN)
	expr := p.parseExpr()
	p.eat(RPAREN)
	p.eat(SEMI)
	return TitleStmt{Expr: expr}
}

type TitlefStmt struct {
	Format ASTNode
	Args   []ASTNode
}

func (t TitlefStmt) Eval(ctx *ExecutionContext) interface{} {
	formatVal := t.Format.Eval(ctx)
	format, ok := formatVal.(string)
	if !ok {
		panic("titlef格式必须是字符串")
	}

	var args []interface{}
	for _, argExpr := range t.Args {
		args = append(args, argExpr.Eval(ctx))
	}

	title := fmt.Sprintf(format, args...)
	setWindowTitle(title)
	return nil
}

// 在 AST 接口实现部分添加键盘记录节点
type KeyboardRecordExpr struct {
	Duration ASTNode // 可选：记录时长（秒），如果为nil则持续记录直到手动停止
	FilePath ASTNode // 可选：保存记录的文件路径
}

type KeyboardStopRecordExpr struct{}

// 执行键盘记录
func (kr KeyboardRecordExpr) Eval(ctx *ExecutionContext) interface{} {
	// 获取参数
	var duration float64 = 0 // 0表示持续记录直到手动停止
	var filePath string = "keyboard_record.txt"

	if kr.Duration != nil {
		durVal := kr.Duration.Eval(ctx)
		if f, ok := durVal.(float64); ok {
			duration = f
		}
	}

	if kr.FilePath != nil {
		pathVal := kr.FilePath.Eval(ctx)
		if path, ok := pathVal.(string); ok {
			filePath = path
		}
	}

	// 开始键盘记录
	return startKeyboardRecord(duration, filePath)
}

// 停止键盘记录
func (ks KeyboardStopRecordExpr) Eval(ctx *ExecutionContext) interface{} {
	return stopKeyboardRecord()
}

// 键盘记录管理器
type KeyboardRecorder struct {
	isRecording bool
	recordChan  chan string
	stopChan    chan bool
	mu          sync.Mutex
	file        *os.File
}

var keyboardRecorder = &KeyboardRecorder{
	recordChan: make(chan string, 100),
	stopChan:   make(chan bool),
}

// 开始键盘记录
func startKeyboardRecord(duration float64, filePath string) string {
	keyboardRecorder.mu.Lock()
	defer keyboardRecorder.mu.Unlock()

	if keyboardRecorder.isRecording {
		return "键盘记录已经在进行中"
	}

	// 打开或创建记录文件
	var err error
	keyboardRecorder.file, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Sprintf("无法打开记录文件: %v", err)
	}

	keyboardRecorder.isRecording = true

	// 启动记录协程
	go func() {
		var timer *time.Timer
		if duration > 0 {
			timer = time.NewTimer(time.Duration(duration) * time.Second)
		}

		for {
			select {
			case key := <-keyboardRecorder.recordChan:
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				recordLine := fmt.Sprintf("[%s] %s\n", timestamp, key)

				keyboardRecorder.mu.Lock()
				if keyboardRecorder.file != nil {
					keyboardRecorder.file.WriteString(recordLine)
				}
				keyboardRecorder.mu.Unlock()

			case <-keyboardRecorder.stopChan:
				keyboardRecorder.mu.Lock()
				if keyboardRecorder.file != nil {
					keyboardRecorder.file.Close()
					keyboardRecorder.file = nil
				}
				keyboardRecorder.isRecording = false
				keyboardRecorder.mu.Unlock()
				return

			case <-func() <-chan time.Time {
				if timer != nil {
					return timer.C
				}
				// 如果没有定时器，创建一个永不触发的通道
				return make(chan time.Time)
			}():
				stopKeyboardRecord()
				return
			}
		}
	}()

	// 启动键盘监听
	go listenToKeyboard()

	if duration > 0 {
		return fmt.Sprintf("开始记录键盘输入，时长%.1f秒，保存到: %s", duration, filePath)
	}
	return fmt.Sprintf("开始记录键盘输入，保存到: %s (使用keyboard.stopRecord()停止)", filePath)
}

// 停止键盘记录
func stopKeyboardRecord() string {
	keyboardRecorder.mu.Lock()
	defer keyboardRecorder.mu.Unlock()

	if !keyboardRecorder.isRecording {
		return "没有活动的键盘记录"
	}

	// 发送停止信号
	select {
	case keyboardRecorder.stopChan <- true:
	default:
	}

	return "已停止键盘记录"
}

// 监听键盘事件（Windows实现）
func listenToKeyboard() {
	// 使用Windows API监听键盘
	user32 := syscall.NewLazyDLL("user32.dll")
	getAsyncKeyState := user32.NewProc("GetAsyncKeyState")

	// 上次按键时间，用于防抖
	lastKeyTime := make(map[int]time.Time)

	for {
		keyboardRecorder.mu.Lock()
		if !keyboardRecorder.isRecording {
			keyboardRecorder.mu.Unlock()
			break
		}
		keyboardRecorder.mu.Unlock()

		// 检查所有按键
		for vk := 0; vk < 256; vk++ {
			ret, _, _ := getAsyncKeyState.Call(uintptr(vk))
			if ret&0x8000 != 0 {
				now := time.Now()

				// 检查上次按键时间，防抖
				if lastTime, ok := lastKeyTime[vk]; ok {
					if now.Sub(lastTime) < 100*time.Millisecond {
						continue
					}
				}

				lastKeyTime[vk] = now

				// 转换虚拟键码为按键名称
				keyName := getKeyName(vk)
				if keyName != "" {
					select {
					case keyboardRecorder.recordChan <- keyName:
					default:
						// 通道已满，丢弃记录
					}
				}
			}
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// 获取按键名称
func getKeyName(vk int) string {
	switch vk {
	case 0x08:
		return "[退格]"
	case 0x09:
		return "[Tab]"
	case 0x0D:
		return "[回车]"
	case 0x10, 0xA0, 0xA1:
		return "[Shift]"
	case 0x11, 0xA2, 0xA3:
		return "[Ctrl]"
	case 0x12, 0xA4, 0xA5:
		return "[Alt]"
	case 0x1B:
		return "[Esc]"
	case 0x20:
		return "[空格]"
	case 0x21, 0x22, 0x23, 0x24:
		return "[导航键]"
	case 0x25, 0x26, 0x27, 0x28:
		return "[方向键]"
	case 0x2D:
		return "[Ins]"
	case 0x2E:
		return "[Del]"
	case 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x7B:
		return fmt.Sprintf("[F%d]", vk-0x6F)
	}

	// 字母键
	if vk >= 0x41 && vk <= 0x5A {
		return string(rune(vk))
	}

	// 数字键
	if vk >= 0x30 && vk <= 0x39 {
		return string(rune(vk))
	}

	// 小键盘
	if vk >= 0x60 && vk <= 0x69 {
		return fmt.Sprintf("[小键盘%d]", vk-0x60)
	}

	return fmt.Sprintf("[VK:0x%X]", vk)
}

// 对象定义
type ObjectDefinition struct {
	Name       string
	ParentName string // 新增：父类名
	Members    map[string]ASTNode
	InitBlock  []ASTNode // 新增：初始化块
}

// 对象成员访问
type ObjectMemberAccess struct {
	ObjectName string
	MemberName string
}

// 对象文字（用于直接创建对象）
type ObjectLiteral struct {
	Members map[string]ASTNode
}

func (o ObjectDefinition) Eval(ctx *ExecutionContext) interface{} {
	// 1. 创建基础对象
	obj := make(map[string]interface{})

	// 2. 如果有父类，复制父类的成员
	if o.ParentName != "" {
		parentInfo, exists := ctx.env[o.ParentName]
		if !exists {
			panic(fmt.Sprintf("父类 '%s' 未定义", o.ParentName))
		}

		parentObj, ok := parentInfo.Value.(map[string]interface{})
		if !ok {
			panic(fmt.Sprintf("'%s' 不是有效的对象", o.ParentName))
		}

		// 复制父类的所有非方法成员到当前对象
		for key, value := range parentObj {
			if _, isMethod := value.(FunctionValue); !isMethod {
				obj[key] = value
			}
		}
	}

	// 3. 创建一个临时的执行环境用于初始化块
	initCtx := &ExecutionContext{
		env:        make(map[string]VarInfo),
		deferStack: make(map[int][]DeferItem),
		wg:         ctx.wg,
		globalEnv:  ctx.globalEnv,
		globalMu:   ctx.globalMu,
	}

	// 4. 将当前对象作为 this 放入环境
	initCtx.env["this"] = VarInfo{
		Type:  DictType,
		Value: obj,
	}

	// 5. 复制父类方法到环境（用于在 init 块中调用）
	if o.ParentName != "" {
		parentInfo, _ := ctx.env[o.ParentName]
		parentObj, _ := parentInfo.Value.(map[string]interface{})

		for key, value := range parentObj {
			if fv, ok := value.(FunctionValue); ok {
				// 复制父类方法，但重新绑定 this 到当前对象
				copiedFv := fv
				copiedFv.Env = deepCopyEnv(fv.Env)
				copiedFv.Env["this"] = VarInfo{
					Type:  DictType,
					Value: obj,
				}
				initCtx.env[key] = VarInfo{
					Type:  FunctionType,
					Value: copiedFv,
				}
			}
		}
	}

	// 6. 执行初始化块
	for _, stmt := range o.InitBlock {
		stmt.Eval(initCtx)

		// 检查是否在 init 块中定义了新变量，如果有则添加到对象
		for name, info := range initCtx.env {
			if name != "this" && name != "super" {
				// 检查这个变量是否已经在对象中
				if _, exists := obj[name]; !exists {
					obj[name] = info.Value
				}
			}
		}
	}

	// 7. 处理普通的 let 成员（如果有的话，会覆盖 init 块中定义的同名变量）
	for key, expr := range o.Members {
		if letStmt, ok := expr.(LetStatement); ok {
			// 处理 let 语句
			val := letStmt.Expr.Eval(initCtx)
			obj[key] = val
		}
	}

	// 8. 处理方法定义（包括父类方法的重写）
	methodFunctions := make(map[string]FunctionValue)

	// 先处理父类方法
	if o.ParentName != "" {
		parentInfo, _ := ctx.env[o.ParentName]
		parentObj, _ := parentInfo.Value.(map[string]interface{})

		for key, value := range parentObj {
			if fv, ok := value.(FunctionValue); ok {
				methodFunctions[key] = fv
			}
		}
	}

	// 处理当前对象的方法定义
	for key, expr := range o.Members {
		if methodDef, ok := expr.(ObjectMethodDefinition); ok {
			// 创建方法函数
			methodEnv := deepCopyEnv(initCtx.env)

			// 添加 this 引用
			methodEnv["this"] = VarInfo{
				Type:  DictType,
				Value: obj,
			}

			// 创建方法函数
			methodFunc := FunctionValue{
				Name:        methodDef.Name,
				Params:      methodDef.Params,
				Body:        methodDef.Body,
				Env:         methodEnv,
				Defaults:    methodDef.Defaults,
				HasVarargs:  methodDef.HasVarargs,
				VarargsName: methodDef.VarargsName,
			}

			methodFunctions[key] = methodFunc
			obj[key] = methodFunc
		}
	}

	// 9. 更新所有方法中的 this 引用
	for key, methodFunc := range methodFunctions {
		methodFunc.Env["this"] = VarInfo{
			Type:  DictType,
			Value: obj,
		}
		obj[key] = methodFunc
	}

	// 10. 将对象存储到环境中
	ctx.env[o.Name] = VarInfo{
		Type:  DictType,
		Value: obj,
	}

	return obj
}

type ObjectMethodCall struct {
	ObjectName string
	MethodName string
	Args       []ASTNode
}

func (omc ObjectMethodCall) Eval(ctx *ExecutionContext) interface{} {
	var obj map[string]interface{}

	// 处理 this 作为对象名的情况
	if omc.ObjectName == "this" {
		// 从环境中获取 this
		thisInfo, exists := ctx.env["this"]
		if !exists {
			panic("'this' 只能在对象方法中使用")
		}

		// 检查是否是对象/字典
		var ok bool
		obj, ok = thisInfo.Value.(map[string]interface{})
		if !ok {
			panic("'this' 不是对象")
		}
	} else {
		// 从环境中获取对象
		objInfo, exists := ctx.env[omc.ObjectName]
		if !exists {
			panic(fmt.Sprintf("对象 '%s' 未定义", omc.ObjectName))
		}

		// 检查是否是对象/字典
		var ok bool
		obj, ok = objInfo.Value.(map[string]interface{})
		if !ok {
			panic(fmt.Sprintf("'%s' 不是对象", omc.ObjectName))
		}
	}

	// 获取方法
	methodVal, exists := obj[omc.MethodName]
	if !exists {
		// 尝试从环境中直接获取函数（用于对象内部方法的互调）
		if methodInfo, exists := ctx.env[omc.MethodName]; exists && methodInfo.Type == FunctionType {
			methodVal = methodInfo.Value
		} else {
			panic(fmt.Sprintf("对象没有方法 '%s'", omc.MethodName))
		}
	}

	fv, ok := methodVal.(FunctionValue)
	if !ok {
		panic(fmt.Sprintf("'%s.%s' 不是方法", omc.ObjectName, omc.MethodName))
	}

	// 评估参数
	argValues := make([]interface{}, len(omc.Args))
	for i, arg := range omc.Args {
		argValues[i] = arg.Eval(ctx)
	}

	// 绑定参数
	actualArgs := fv.bindArguments(argValues, ctx)

	// 创建新的执行环境
	newEnv := deepCopyEnv(fv.Env)

	// 绑定参数到环境
	for i, param := range fv.Params {
		if i < len(actualArgs) {
			argVal := actualArgs[i]
			newEnv[param] = VarInfo{
				Type:  getValueType(argVal),
				Value: argVal,
			}
		}
	}

	// 如果调用的是 this.method，确保环境中包含 this
	if omc.ObjectName == "this" {
		newEnv["this"] = VarInfo{
			Type:  DictType,
			Value: obj,
		}
	}

	// 执行方法
	var result interface{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ret, ok := r.(ReturnValue); ok {
					result = ret.Value
				} else {
					panic(r)
				}
			}
		}()

		methodCtx := &ExecutionContext{
			env:        newEnv,
			deferStack: make(map[int][]DeferItem),
			wg:         ctx.wg,
			globalEnv:  ctx.globalEnv,
			globalMu:   ctx.globalMu,
		}

		for _, stmt := range fv.Body {
			stmt.Eval(methodCtx)
		}
	}()

	return result
}

func (p *Parser) parseObjectMethodCall(objectName string) ASTNode {
	p.eat(DOLLAR)

	methodName := p.curr.Value
	p.eat(IDENT)

	// 检查是否是方法调用还是属性访问
	if p.curr.Type == LPAREN {
		// 方法调用
		p.eat(LPAREN)
		var args []ASTNode
		for p.curr.Type != RPAREN {
			arg := p.parseExpr()
			args = append(args, arg)
			if p.curr.Type == COMMA {
				p.eat(COMMA)
			}
		}
		p.eat(RPAREN)

		return ObjectMethodCall{
			ObjectName: objectName,
			MethodName: methodName,
			Args:       args,
		}
	} else {
		// 属性访问
		return ObjectMemberAccess{
			ObjectName: objectName,
			MemberName: methodName,
		}
	}
}

func (o ObjectMemberAccess) Eval(ctx *ExecutionContext) interface{} {
	var obj map[string]interface{}

	// 检查是否是 this 访问
	if o.ObjectName == "this" {
		// 从环境中获取 this
		thisInfo, exists := ctx.env["this"]
		if !exists {
			panic("'this' 只能在对象方法中使用")
		}

		// 检查是否是对象/字典
		var ok bool
		obj, ok = thisInfo.Value.(map[string]interface{})
		if !ok {
			panic("'this' 不是对象")
		}
	} else {
		// 从环境中获取对象
		objInfo, exists := ctx.env[o.ObjectName]
		if !exists {
			panic(fmt.Sprintf("对象 '%s' 未定义", o.ObjectName))
		}

		// 检查是否是对象/字典
		var ok bool
		obj, ok = objInfo.Value.(map[string]interface{})
		if !ok {
			panic(fmt.Sprintf("'%s' 不是对象", o.ObjectName))
		}
	}

	// 获取成员值
	val, exists := obj[o.MemberName]
	if !exists {
		panic(fmt.Sprintf("对象没有成员 '%s'", o.MemberName))
	}

	return val
}

func (o ObjectLiteral) Eval(ctx *ExecutionContext) interface{} {
	obj := make(map[string]interface{})

	// 收集所有成员
	memberValues := make(map[string]interface{})

	for key, expr := range o.Members {
		// 直接计算值
		val := expr.Eval(ctx)
		memberValues[key] = val
		obj[key] = val
	}

	// 对于函数成员，创建闭包环境
	for key, value := range obj {
		if fv, ok := value.(FunctionValue); ok {
			// 创建闭包环境，包含所有成员
			closureEnv := make(map[string]VarInfo)
			for k, v := range memberValues {
				closureEnv[k] = VarInfo{
					Type:  getValueType(v),
					Value: v,
				}
			}

			// 添加 this 引用
			closureEnv["this"] = VarInfo{
				Type:  DictType,
				Value: obj,
			}

			// 更新函数环境
			fv.Env = closureEnv
			obj[key] = fv
		}
	}

	return obj
}

func (p *Parser) parseObjectDefinition() ASTNode {
	p.eat(OBJECT_TYPE) // 吃掉 "obj"

	// 获取对象名
	name := p.curr.Value
	p.eat(IDENT)

	// 检查是否有继承
	var parentName string
	if p.curr.Type == EXTENDS {
		p.eat(EXTENDS)
		parentName = p.curr.Value
		p.eat(IDENT)
	}

	p.eat(LBRACE)

	members := make(map[string]ASTNode)
	var initBlock []ASTNode

	for p.curr.Type != RBRACE && p.curr.Type != EOF {
		if p.curr.Type == LET {
			// 处理 let 语句
			p.eat(LET)

			// 获取成员名
			memberName := p.curr.Value
			p.eat(IDENT)

			p.eat(ASSIGN)

			// 解析成员值表达式
			expr := p.parseExpr()

			members[memberName] = LetStatement{
				Name: memberName,
				Expr: expr,
			}

			// 可选的分号
			if p.curr.Type == SEMI {
				p.eat(SEMI)
			}
		} else if p.curr.Type == DEF {
			// 处理方法定义
			p.eat(DEF)

			// 获取方法名
			methodName := p.curr.Value
			p.eat(IDENT)

			// 解析方法定义
			methodDef := p.parseObjectMethodDefinition(methodName)
			members[methodName] = methodDef

			// 可选的分号
			if p.curr.Type == SEMI {
				p.eat(SEMI)
			}
		} else if p.curr.Type == IDENT && p.curr.Value == "init" {
			// 处理 init 块
			p.eat(IDENT)

			// 解析 init 块
			initBlock = p.parseInitBlock()
		} else {
			panic(fmt.Sprintf("对象定义中期望 'let', 'fn' 或 'init'，得到 %v (值: %s)", p.curr.Type, p.curr.Value))
		}
	}

	p.eat(RBRACE)

	return ObjectDefinition{
		Name:       name,
		ParentName: parentName,
		Members:    members,
		InitBlock:  initBlock,
	}
}

func (p *Parser) parseInitBlock() []ASTNode {
	if p.curr.Type != LBRACE {
		panic("期望 '{' 开始 init 块")
	}
	p.eat(LBRACE)

	var stmts []ASTNode
	for p.curr.Type != RBRACE && p.curr.Type != EOF {
		stmt := p.parseStatement()
		stmts = append(stmts, stmt)
	}

	p.eat(RBRACE)
	return stmts
}

func (p *Parser) parseObjectMethodDefinition(methodName string) ASTNode {
	p.eat(LPAREN)

	var params []string
	defaults := make(map[string]ASTNode)
	hasVarargs := false
	varargsName := ""

	// 解析参数列表（和全局函数一样）
	for p.curr.Type != RPAREN {
		// 检查是否是可变参数
		if p.curr.Type == ELLIPSIS {
			p.eat(ELLIPSIS)
			if p.curr.Type != IDENT {
				panic("可变参数必须指定参数名")
			}
			hasVarargs = true
			varargsName = p.curr.Value
			params = append(params, varargsName)
			p.eat(IDENT)
			break // 可变参数必须是最后一个
		}

		// 普通参数
		if p.curr.Type == IDENT {
			paramName := p.curr.Value
			p.eat(IDENT)
			params = append(params, paramName)

			// 检查是否有默认值
			if p.curr.Type == ASSIGN {
				p.eat(ASSIGN)
				defaultExpr := p.parseExpr()
				defaults[paramName] = defaultExpr
			}
		}

		if p.curr.Type == COMMA {
			p.eat(COMMA)
		}
	}
	p.eat(RPAREN)

	// 保存当前作用域，为方法体创建新作用域
	savedScopes := p.scopes
	p.scopes = append(p.scopes, make(map[string]VarInfo))

	// 将参数添加到新作用域（和全局函数一样）
	for _, param := range params {
		p.scopes[len(p.scopes)-1][param] = VarInfo{
			Type:  StringType,
			Value: nil,
		}
	}

	// 添加 this 到作用域
	p.scopes[len(p.scopes)-1]["this"] = VarInfo{
		Type:  DictType,
		Value: nil, // 将在运行时设置
	}

	// 解析方法体（和全局函数一样）
	body := p.parseBlock()

	// 恢复原来的作用域
	p.scopes = savedScopes

	return ObjectMethodDefinition{
		Name:        methodName,
		Params:      params,
		Body:        body,
		Defaults:    defaults,
		HasVarargs:  hasVarargs,
		VarargsName: varargsName,
	}
}

type ObjectMethodDefinition struct {
	Name        string
	Params      []string
	Body        []ASTNode
	Defaults    map[string]ASTNode
	HasVarargs  bool
	VarargsName string
}

func (o ObjectMethodDefinition) Eval(ctx *ExecutionContext) interface{} {
	// 创建一个可以递归调用的函数
	// 注意：这里我们创建一个占位符函数，实际的方法名会在调用时绑定
	return FunctionValue{
		Params:      o.Params,
		Body:        o.Body,
		Env:         make(map[string]VarInfo), // 空环境，方法名将在调用时添加
		Defaults:    o.Defaults,
		HasVarargs:  o.HasVarargs,
		VarargsName: o.VarargsName,
	}
}

func (p *Parser) parseObjectLiteral() ASTNode {
	p.eat(LBRACE)

	members := make(map[string]ASTNode)

	for p.curr.Type != RBRACE && p.curr.Type != EOF {
		// 成员名必须是标识符或字符串
		var keyName string
		if p.curr.Type == IDENT || p.curr.Type == STRING {
			keyName = p.curr.Value
			p.eat(p.curr.Type)
		} else {
			panic("对象字面量的键必须是标识符或字符串")
		}

		p.eat(COLON)

		// 解析值表达式
		value := p.parseExpr()
		members[keyName] = value

		if p.curr.Type == COMMA {
			p.eat(COMMA)
		}
	}

	p.eat(RBRACE)
	return ObjectLiteral{Members: members}
}

//加入新功能：使obj{}内支持fn函数

// 11. 添加ThisExpr节点
type ThisExpr struct{}

func (t ThisExpr) Eval(ctx *ExecutionContext) interface{} {
	// 查找环境中的 this
	if val, ok := ctx.env["this"]; ok {
		// 确保 this 是有效的对象
		if obj, ok := val.Value.(map[string]interface{}); ok {
			return obj
		}
		panic("'this' 不是有效的对象")
	}
	panic("'this' 只能在对象方法中使用")
}

type SuperCall struct {
	MethodName string
	Args       []ASTNode
}

func (sc SuperCall) Eval(ctx *ExecutionContext) interface{} {
	// 获取父类对象
	thisInfo, exists := ctx.env["this"]
	if !exists {
		panic("super 只能在对象方法中使用")
	}

	obj, ok := thisInfo.Value.(map[string]interface{})
	if !ok {
		panic("this 不是有效的对象")
	}

	// 从环境中获取对象的父类引用
	superInfo, exists := ctx.env["__parent__"]
	if !exists {
		panic("无法找到父类引用")
	}

	parentObj, ok := superInfo.Value.(map[string]interface{})
	if !ok {
		panic("父类引用无效")
	}

	// 获取父类方法
	methodVal, exists := parentObj[sc.MethodName]
	if !exists {
		panic(fmt.Sprintf("父类没有方法 '%s'", sc.MethodName))
	}

	fv, ok := methodVal.(FunctionValue)
	if !ok {
		panic(fmt.Sprintf("'%s' 不是方法", sc.MethodName))
	}

	// 评估参数
	argValues := make([]interface{}, len(sc.Args))
	for i, arg := range sc.Args {
		argValues[i] = arg.Eval(ctx)
	}

	// 绑定参数
	actualArgs := fv.bindArguments(argValues, ctx)

	// 创建新的执行环境
	newEnv := deepCopyEnv(fv.Env)

	// 绑定参数到环境
	for i, param := range fv.Params {
		if i < len(actualArgs) {
			argVal := actualArgs[i]
			newEnv[param] = VarInfo{
				Type:  getValueType(argVal),
				Value: argVal,
			}
		}
	}

	// 确保环境中包含 this（指向当前对象）
	newEnv["this"] = VarInfo{
		Type:  DictType,
		Value: obj,
	}

	// 执行方法
	var result interface{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ret, ok := r.(ReturnValue); ok {
					result = ret.Value
				} else {
					panic(r)
				}
			}
		}()

		methodCtx := &ExecutionContext{
			env:        newEnv,
			deferStack: make(map[int][]DeferItem),
			wg:         ctx.wg,
			globalEnv:  ctx.globalEnv,
			globalMu:   ctx.globalMu,
		}

		for _, stmt := range fv.Body {
			stmt.Eval(methodCtx)
		}
	}()

	return result
}

func (oma ObjectMemberAssignment) Eval(ctx *ExecutionContext) interface{} {
	// 这个节点现在只包含赋值的目标，不包含表达式
	// 在 set 语句中，表达式会在外层传递
	// 所以我们需要一个完整版本的赋值节点
	return oma
}

type ObjectMemberAssignmentFull struct {
	ObjectName string
	MemberName string
	Expr       ASTNode
	IsDollar   bool
}

func (omaf ObjectMemberAssignmentFull) Eval(ctx *ExecutionContext) interface{} {
	// 获取对象
	objInfo, exists := ctx.env[omaf.ObjectName]
	if !exists {
		panic(fmt.Sprintf("对象 '%s' 未定义", omaf.ObjectName))
	}

	// 检查是否是对象/字典
	obj, ok := objInfo.Value.(map[string]interface{})
	if !ok {
		panic(fmt.Sprintf("'%s' 不是对象", omaf.ObjectName))
	}

	// 计算表达式值
	val := omaf.Expr.Eval(ctx)

	// 赋值给对象成员
	obj[omaf.MemberName] = val

	// 更新环境中的对象
	ctx.env[omaf.ObjectName] = VarInfo{
		Type:  DictType,
		Value: obj,
	}

	return val
}

// 在 TokenType 定义之后，ASTNode 接口定义之前添加
type ObjectMemberAssignment struct {
	ObjectName string
	MemberName string
	IsDollar   bool // 使用 $ 还是 . 访问
}

// 加入新功能：obj中加入init{let a = 1;}使变量定义后可以被继承或更改
type LetStatement struct {
	Name string
	Expr ASTNode
}

func (l LetStatement) Eval(ctx *ExecutionContext) interface{} {
	val := l.Expr.Eval(ctx)

	// 将变量添加到当前环境
	ctx.env[l.Name] = VarInfo{
		Type:  getValueType(val),
		Value: val,
	}

	return val
}

type ExitStmt struct {
	Code ASTNode // 可选的退出码，可以为nil
}

func (e ExitStmt) Eval(ctx *ExecutionContext) interface{} {
	var exitCode int

	if e.Code != nil {
		// 获取退出码
		codeVal := e.Code.Eval(ctx)

		// 处理不同类型的退出码
		switch v := codeVal.(type) {
		case float64:
			exitCode = int(v)
		case int:
			exitCode = v
		case string:
			// 尝试将字符串转换为数字
			if code, err := strconv.Atoi(v); err == nil {
				exitCode = code
			} else {
				// 如果无法转换，使用字符串哈希作为退出码
				exitCode = 1
			}
		default:
			exitCode = 0
		}
	} else {
		exitCode = 0 // 默认退出码为0
	}

	// 如果有defer语句，先执行它们
	processDeferStatements(ctx)

	// 等待所有goroutine完成（可选）
	ctx.wg.Wait()

	// 退出程序
	os.Exit(exitCode)
	return nil // 这行代码实际上不会执行
}

// 步骤4：在Parser中添加解析exit语句的方法
func (p *Parser) parseExitStmt() ASTNode {
	p.eat(EXIT)

	var codeExpr ASTNode = nil

	// 检查是否有退出码参数
	if p.curr.Type == LPAREN {
		p.eat(LPAREN)

		// 如果右括号紧跟在左括号后面，表示没有参数
		if p.curr.Type != RPAREN {
			codeExpr = p.parseExpr()
		}

		p.eat(RPAREN)
	}

	p.eat(SEMI)
	return ExitStmt{Code: codeExpr}
}

// ==================== 内存写入与读取 ====================

// 全局共享内存（线程安全，供 go 块跨协程使用）
var (
	globalMemory   = make(map[string]interface{})
	globalMemoryMu sync.RWMutex
)

// 深拷贝值，防止外部修改污染内存中的存储内容
func deepCopyMemoryValue(v interface{}) interface{} {
	switch val := v.(type) {
	case []interface{}:
		newList := make([]interface{}, len(val))
		for i, item := range val {
			newList[i] = deepCopyMemoryValue(item)
		}
		return newList
	case map[string]interface{}:
		newDict := make(map[string]interface{}, len(val))
		for k, item := range val {
			newDict[k] = deepCopyMemoryValue(item)
		}
		return newDict
	default:
		return v
	}
}

// memory.write(key, value);
type MemoryWrite struct {
	Key   ASTNode
	Value ASTNode
}

func (m MemoryWrite) Eval(ctx *ExecutionContext) interface{} {
	key := fmt.Sprint(m.Key.Eval(ctx))
	value := m.Value.Eval(ctx)

	globalMemoryMu.Lock()
	globalMemory[key] = deepCopyMemoryValue(value)
	globalMemoryMu.Unlock()
	return nil
}

// memory.read(key [, default]);
type MemoryRead struct {
	Key     ASTNode
	Default ASTNode
}

func (m MemoryRead) Eval(ctx *ExecutionContext) interface{} {
	key := fmt.Sprint(m.Key.Eval(ctx))

	globalMemoryMu.RLock()
	val, exists := globalMemory[key]
	globalMemoryMu.RUnlock()

	if !exists {
		if m.Default != nil {
			return m.Default.Eval(ctx)
		}
		return nil
	}
	return deepCopyMemoryValue(val)
}

// memory.has(key);
type MemoryHas struct{ Key ASTNode }

func (m MemoryHas) Eval(ctx *ExecutionContext) interface{} {
	key := fmt.Sprint(m.Key.Eval(ctx))
	globalMemoryMu.RLock()
	_, exists := globalMemory[key]
	globalMemoryMu.RUnlock()
	return exists
}

// memory.delete(key);
type MemoryDelete struct{ Key ASTNode }

func (m MemoryDelete) Eval(ctx *ExecutionContext) interface{} {
	key := fmt.Sprint(m.Key.Eval(ctx))
	globalMemoryMu.Lock()
	_, exists := globalMemory[key]
	delete(globalMemory, key)
	globalMemoryMu.Unlock()
	return exists
}

// memory.clear();
type MemoryClear struct{}

func (m MemoryClear) Eval(ctx *ExecutionContext) interface{} {
	globalMemoryMu.Lock()
	globalMemory = make(map[string]interface{})
	globalMemoryMu.Unlock()
	return nil
}

// memory.keys();
type MemoryKeys struct{}

func (m MemoryKeys) Eval(ctx *ExecutionContext) interface{} {
	globalMemoryMu.RLock()
	keys := make([]string, 0, len(globalMemory))
	for k := range globalMemory {
		keys = append(keys, k)
	}
	globalMemoryMu.RUnlock()

	sort.Strings(keys)
	result := make([]interface{}, len(keys))
	for i, k := range keys {
		result[i] = k
	}
	return result
}

// memory.size();
type MemorySize struct{}

func (m MemorySize) Eval(ctx *ExecutionContext) interface{} {
	globalMemoryMu.RLock()
	n := len(globalMemory)
	globalMemoryMu.RUnlock()
	return float64(n)
}

// memory.add(key, delta);
type MemoryAdd struct {
	Key   ASTNode
	Delta ASTNode
}

func (m MemoryAdd) Eval(ctx *ExecutionContext) interface{} {
	key := fmt.Sprint(m.Key.Eval(ctx))
	deltaVal := m.Delta.Eval(ctx)
	delta, ok := deltaVal.(float64)
	if !ok {
		panic("memory.add 需要数字增量")
	}

	globalMemoryMu.Lock()
	defer globalMemoryMu.Unlock()

	var current float64 = 0
	if v, exists := globalMemory[key]; exists {
		if f, ok := v.(float64); ok {
			current = f
		} else {
			panic(fmt.Sprintf("memory.add 目标 '%s' 不是数字", key))
		}
	}
	newVal := current + delta
	globalMemory[key] = newVal
	return newVal
}

func (p *Parser) parseMemoryExpr() ASTNode {
	p.eat(MEMORY)
	p.eat(DOT)

	method := p.curr

	switch method.Type {
	case READ:
		p.eat(READ)
		p.eat(LPAREN)
		key := p.parseExpr()
		var defaultVal ASTNode
		if p.curr.Type == COMMA {
			p.eat(COMMA)
			defaultVal = p.parseExpr()
		}
		p.eat(RPAREN)
		return MemoryRead{Key: key, Default: defaultVal}

	case WRITE:
		p.eat(WRITE)
		p.eat(LPAREN)
		key := p.parseExpr()
		p.eat(COMMA)
		value := p.parseExpr()
		p.eat(RPAREN)
		return MemoryWrite{Key: key, Value: value}

	case DELETE:
		p.eat(DELETE)
		p.eat(LPAREN)
		key := p.parseExpr()
		p.eat(RPAREN)
		return MemoryDelete{Key: key}

	case CLEAR:
		p.eat(CLEAR)
		p.eat(LPAREN)
		p.eat(RPAREN)
		return MemoryClear{}

	case ADD_METHOD: // "add" 被词法分析为 ADD_METHOD
		p.eat(ADD_METHOD)
		p.eat(LPAREN)
		key := p.parseExpr()
		p.eat(COMMA)
		delta := p.parseExpr()
		p.eat(RPAREN)
		return MemoryAdd{Key: key, Delta: delta}
	case APPEND: // memory.append(key, value)
		p.eat(APPEND)
		p.eat(LPAREN)
		key := p.parseExpr()
		p.eat(COMMA)
		value := p.parseExpr()
		p.eat(RPAREN)
		return MemoryAppend{Key: key, Value: value}

	case IDENT:
		switch method.Value {
		case "getString", "getStr":
			p.eat(IDENT)
			p.eat(LPAREN)
			key := p.parseExpr()
			var defaultVal ASTNode
			if p.curr.Type == COMMA {
				p.eat(COMMA)
				defaultVal = p.parseExpr()
			}
			p.eat(RPAREN)
			return MemoryGetString{Key: key, Default: defaultVal}

		case "writeString", "writeStr":
			p.eat(IDENT)
			p.eat(LPAREN)
			key := p.parseExpr()
			p.eat(COMMA)
			value := p.parseExpr()
			p.eat(RPAREN)
			return MemoryWriteString{Key: key, Value: value}

		case "has":
			p.eat(IDENT)
			p.eat(LPAREN)
			key := p.parseExpr()
			p.eat(RPAREN)
			return MemoryHas{Key: key}
		case "keys":
			p.eat(IDENT)
			p.eat(LPAREN)
			p.eat(RPAREN)
			return MemoryKeys{}
		case "size":
			p.eat(IDENT)
			p.eat(LPAREN)
			p.eat(RPAREN)
			return MemorySize{}
		default:
			panic(fmt.Sprintf("未知的 memory 方法: %s", method.Value))
		}

	default:
		panic(fmt.Sprintf("未知的 memory 方法: %s", method.Value))
	}
}

// 带位置的语句包装，用于拆分执行时定位错误
type LocatedStmt struct {
	Stmt   ASTNode
	Line   int
	Column int
}

// ParseWithLocations 解析整个文件，同时记录每条顶层语句的起始位置
func (p *Parser) ParseWithLocations() []LocatedStmt {
	var stmts []LocatedStmt
	for p.curr.Type != EOF {
		line := p.lexer.line
		col := p.lexer.column
		stmt := p.parseStatement()
		stmts = append(stmts, LocatedStmt{Stmt: stmt, Line: line, Column: col})
	}
	return stmts
}

// executeSplitFile 将目标文件拆分成多个语句块，在共享全局环境中逐块执行
// 用于定位脚本中的错误：每块出错时报告对应行号与错误信息
func executeSplitFile(filePath string) {
	fmt.Printf("🔧 拆分执行模式: %s\n\n", filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("❌ 读取文件失败: %v\n", err)
		return
	}

	// 解析所有顶层语句，并记录每条语句的起始位置
	lexer := NewLexer(string(content))
	parser := NewParser(lexer)

	var locatedStmts []LocatedStmt
	parseFailed := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				parseFailed = true
				fmt.Printf("❌ 解析失败（语法错误）：%v\n", r)
				fmt.Printf("   位置: 第 %d 行第 %d 列\n", lexer.line, lexer.column)
			}
		}()
		locatedStmts = parser.ParseWithLocations()
	}()

	if parseFailed {
		return
	}
	if len(locatedStmts) == 0 {
		fmt.Println("⚠️  文件中没有可执行的语句")
		return
	}

	fmt.Printf("✅ 解析成功，共 %d 个语句块\n", len(locatedStmts))
	fmt.Println(strings.Repeat("─", 60))

	// 创建共享的全局执行环境 —— 所有语句块共用同一个 env
	// 创建共享的全局执行环境 —— 所有语句块共用同一个 env
	globalMu := &sync.Mutex{}
	globalEnv := &ExecutionContext{
		env:           make(map[string]VarInfo),
		deferStack:    make(map[int][]DeferItem),
		wg:            &sync.WaitGroup{},
		globalEnv:     nil,
		globalMu:      globalMu,
		loadedModules: make(map[string]bool),
		profiler:      globalProfiler,
		Args:          os.Args[2:],
		SuppressClear: true, // 新增：拆分调试模式下不执行 clear()
	}
	globalEnv.globalEnv = globalEnv

	// 与主流程保持一致：预先加载已安装的包
	loadAllPackages(globalEnv)

	successCount := 0
	failCount := 0
	var failures []string

	for i, ls := range locatedStmts {
		idx := i + 1
		stmtType := fmt.Sprintf("%T", ls.Stmt)

		func() {
			defer func() {
				if r := recover(); r != nil {
					failCount++
					msg := fmt.Sprintf("[第%d部分] 行 %d 列 %d 类型 %s —— 错误: %v",
						idx, ls.Line, ls.Column, stmtType, r)
					failures = append(failures, msg)
					fmt.Printf("❌ [%d/%d] 行 %d:%d —— 执行失败 (%s)\n",
						idx, len(locatedStmts), ls.Line, ls.Column, stmtType)
					fmt.Printf("   错误: %v\n\n", r)
				}
			}()

			ls.Stmt.Eval(globalEnv)
			successCount++
			fmt.Printf("✓ [%d/%d] 行 %d:%d —— 执行成功 (%s)\n",
				idx, len(locatedStmts), ls.Line, ls.Column, stmtType)
		}()
	}

	// 等待所有 go 块
	globalEnv.wg.Wait()

	// 执行 defer
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("⚠️  defer 执行错误: %v\n", r)
			}
		}()
		processDeferStatements(globalEnv)
	}()

	// 汇总报告
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("执行完成: 成功 %d, 失败 %d, 总计 %d\n",
		successCount, failCount, len(locatedStmts))

	if failCount > 0 {
		fmt.Println("\n❌ 失败清单（请修复以下部分）:")
		for _, f := range failures {
			fmt.Println("  " + f)
		}
		fmt.Printf("\n提示: 上表显示了每段失败语句的起始行号，请检查对应位置\n")
	} else {
		fmt.Println("✅ 所有语句块执行成功")
	}
}

// memory.append(key, value) —— 字符串拼接 / 列表合并 / 数字累加
type MemoryAppend struct {
	Key   ASTNode
	Value ASTNode
}

func (m MemoryAppend) Eval(ctx *ExecutionContext) interface{} {
	key := fmt.Sprint(m.Key.Eval(ctx))
	newVal := m.Value.Eval(ctx)

	globalMemoryMu.Lock()
	defer globalMemoryMu.Unlock()

	existing, exists := globalMemory[key]
	if !exists {
		globalMemory[key] = deepCopyMemoryValue(newVal)
		return newVal
	}

	switch old := existing.(type) {
	case string:
		// 目标是字符串：新值若是字符串则直接拼接，否则先转字符串再拼接
		var appendStr string
		if s, ok := newVal.(string); ok {
			appendStr = s
		} else {
			appendStr = fmt.Sprint(newVal)
		}
		result := old + appendStr
		globalMemory[key] = result
		return result

	case []interface{}:
		oldCopy := deepCopyMemoryValue(old).([]interface{})
		if lst, ok := newVal.([]interface{}); ok {
			result := append(oldCopy, deepCopyMemoryValue(lst).([]interface{})...)
			globalMemory[key] = result
			return result
		}
		result := append(oldCopy, deepCopyMemoryValue(newVal))
		globalMemory[key] = result
		return result

	case float64:
		if f, ok := newVal.(float64); ok {
			result := old + f
			globalMemory[key] = result
			return result
		}
		panic(fmt.Sprintf("memory.append 类型不匹配: 目标为数字，追加值类型为 %T", newVal))

	default:
		// 其他类型直接覆盖
		globalMemory[key] = deepCopyMemoryValue(newVal)
		return newVal
	}
}

// memory.getString(key[, default]) —— 明确读取字符串（不存在时用默认值或 ""）
type MemoryGetString struct {
	Key     ASTNode
	Default ASTNode
}

func (m MemoryGetString) Eval(ctx *ExecutionContext) interface{} {
	key := fmt.Sprint(m.Key.Eval(ctx))

	globalMemoryMu.RLock()
	val, exists := globalMemory[key]
	globalMemoryMu.RUnlock()

	if !exists {
		if m.Default != nil {
			return fmt.Sprint(m.Default.Eval(ctx))
		}
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprint(val)
}

// memory.writeString(key, str) —— 强制以字符串形式写入
type MemoryWriteString struct {
	Key   ASTNode
	Value ASTNode
}

func (m MemoryWriteString) Eval(ctx *ExecutionContext) interface{} {
	key := fmt.Sprint(m.Key.Eval(ctx))
	strVal := fmt.Sprint(m.Value.Eval(ctx))

	globalMemoryMu.Lock()
	globalMemory[key] = strVal
	globalMemoryMu.Unlock()
	return strVal
}

// ==================== 新增：Bocchi 版本检查与扩展更新 ====================

// CurrentBocchiVersion 当前解释器版本（与 -version 输出保持一致）
const CurrentBocchiVersion = "1.14.1"

// parseVersionNumbers 将 "1.14.0" 拆解为 []int{1, 14, 0}
func parseVersionNumbers(ver string) []int {
	parts := strings.Split(strings.TrimSpace(ver), ".")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		numStr := ""
		for _, r := range p {
			if r >= '0' && r <= '9' {
				numStr += string(r)
			} else {
				break
			}
		}
		if numStr == "" {
			result = append(result, 0)
		} else {
			n, _ := strconv.Atoi(numStr)
			result = append(result, n)
		}
	}
	return result
}

// compareVersion 版本比较：a<b 返回 -1；a==b 返回 0；a>b 返回 1
// 支持任意长度（如 1.9.0 < 1.14.0 正确处理）
func compareVersion(a, b string) int {
	av := parseVersionNumbers(a)
	bv := parseVersionNumbers(b)
	maxLen := len(av)
	if len(bv) > maxLen {
		maxLen = len(bv)
	}
	for i := 0; i < maxLen; i++ {
		var x, y int
		if i < len(av) {
			x = av[i]
		}
		if i < len(bv) {
			y = bv[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

// pathsEqual 判断两个路径是否指向同一文件（Windows 下忽略大小写）
func pathsEqual(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(absA, absB)
	}
	return absA == absB
}

// getInstalledBocchiInfo 获取已安装 bocchi 的路径和版本号
// 若正在运行的就是已安装程序，直接返回当前版本，避免自我调用
func getInstalledBocchiInfo() (path string, version string, err error) {
	exeName := "bocchi"
	if runtime.GOOS == "windows" {
		exeName = "bocchi.exe"
	}

	var candidates []string
	if p, e := exec.LookPath(exeName); e == nil {
		candidates = append(candidates, p)
	}
	if runtime.GOOS == "windows" {
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			candidates = append(candidates, filepath.Join(pf, "Bocchi", "bocchi.exe"))
		}
	} else {
		candidates = append(candidates, "/usr/local/bin/bocchi")
	}

	selfPath, _ := os.Executable()

	for _, candidate := range candidates {
		if _, e := os.Stat(candidate); e != nil {
			continue
		}
		// 若正好是当前运行的程序，直接使用当前版本
		if pathsEqual(candidate, selfPath) {
			return candidate, CurrentBocchiVersion, nil
		}
		// 运行 -version 解析版本号
		out, e := exec.Command(candidate, "-version").Output()
		if e != nil {
			continue
		}
		re := regexp.MustCompile(`BOCCHI\s+([\d.]+)`)
		matches := re.FindStringSubmatch(string(out))
		if len(matches) >= 2 {
			return candidate, matches[1], nil
		}
	}
	return "", "", fmt.Errorf("未找到已安装的 bocchi 解释器")
}

// updateBocchiInstallation 用当前可执行文件覆盖目标位置
func updateBocchiInstallation(targetPath string) error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前可执行文件路径失败: %v", err)
	}
	if pathsEqual(selfPath, targetPath) {
		return nil
	}
	data, err := os.ReadFile(selfPath)
	if err != nil {
		return fmt.Errorf("读取当前可执行文件失败: %v", err)
	}
	if err := os.WriteFile(targetPath, data, 0755); err != nil {
		return fmt.Errorf("写入目标文件失败: %v", err)
	}
	return nil
}

// checkBocchiVersionAndUpdate 检查已安装 Bocchi 版本，若低于当前版本则提示覆盖更新
func checkBocchiVersionAndUpdate() {
	installedPath, installedVer, err := getInstalledBocchiInfo()
	if err != nil {
		// 未安装或无法检测，跳过
		return
	}

	selfPath, _ := os.Executable()
	if pathsEqual(installedPath, selfPath) {
		fmt.Printf("\n📌 Bocchi 版本: %s (当前运行的程序即为已安装版本)\n", installedVer)
		return
	}

	cmp := compareVersion(installedVer, CurrentBocchiVersion)

	if cmp < 0 {
		fmt.Printf("\n⚠️  检测到已安装的 Bocchi 版本低于当前版本:\n")
		fmt.Printf("   已安装版本: %s (%s)\n", installedVer, installedPath)
		fmt.Printf("   当前版本:   %s\n", CurrentBocchiVersion)
		fmt.Print("   是否更新为当前版本（覆盖安装）？(Y/n): ")

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))

		if answer == "n" || answer == "no" {
			fmt.Println("   已跳过更新")
			return
		}

		if runtime.GOOS == "windows" && !isAdmin() {
			fmt.Println("   需要管理员权限来更新 Bocchi，正在请求...")
			runAsAdmin("-env")
			return
		}

		if err := updateBocchiInstallation(installedPath); err != nil {
			fmt.Printf("   ❌ 更新失败: %v\n", err)
			return
		}
		fmt.Printf("   ✅ 已更新至版本 %s\n", CurrentBocchiVersion)
	} else if cmp == 0 {
		fmt.Printf("\n📌 Bocchi 已是最新版本: %s\n", installedVer)
	} else {
		fmt.Printf("\n📌 已安装的 Bocchi 版本 (%s) 高于当前版本 (%s)\n",
			installedVer, CurrentBocchiVersion)
	}
}

// ExtensionInfo 描述一个待安装的扩展包
type ExtensionInfo struct {
	Name    string
	Version int
	Path    string
}

// findNewExtensionsInCWD 扫描当前目录下的 io4.tuka 及以上扩展，按版本降序返回
func findNewExtensionsInCWD() []ExtensionInfo {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return nil
	}

	var extensions []ExtensionInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "io") || !strings.HasSuffix(name, ".tuka") {
			continue
		}
		verStr := strings.TrimSuffix(strings.TrimPrefix(name, "io"), ".tuka")
		ver, err := strconv.Atoi(verStr)
		if err != nil {
			continue
		}
		if ver >= 4 { // io4 及以上
			extensions = append(extensions, ExtensionInfo{
				Name:    strings.TrimSuffix(name, ".tuka"),
				Version: ver,
				Path:    filepath.Join(cwd, name),
			})
		}
	}

	sort.Slice(extensions, func(i, j int) bool {
		return extensions[i].Version > extensions[j].Version
	})
	return extensions
}

// checkExtensionUpdates 检测当前目录下的新版扩展并提示安装，同时清理旧 io2/io3
func checkExtensionUpdates() {
	newExts := findNewExtensionsInCWD()
	if len(newExts) == 0 {
		// 没有检测到新扩展 → 直接继续
		return
	}

	latest := newExts[0]
	fmt.Printf("\n📦 检测到新扩展包: %s (版本 %d)\n",
		filepath.Base(latest.Path), latest.Version)

	// 检查已安装的旧扩展
	initPackages()
	config := loadPackageConfig()

	var oldExtensions []string
	for _, pkg := range config.InstalledPackages {
		if pkg == "io2" || pkg == "io3" {
			oldExtensions = append(oldExtensions, pkg)
		}
	}

	if len(oldExtensions) > 0 {
		fmt.Printf("   检测到需要移除的旧扩展: %v\n", oldExtensions)
	}

	fmt.Print("   是否安装最新扩展")
	if len(oldExtensions) > 0 {
		fmt.Print("（并移除旧扩展）")
	}
	fmt.Print("？(Y/n): ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer == "n" || answer == "no" {
		fmt.Println("   已跳过扩展安装")
		return
	}

	// 先移除旧扩展
	if len(oldExtensions) > 0 {
		for _, pkg := range oldExtensions {
			fmt.Printf("   正在移除旧扩展 '%s'...\n", pkg)
			uninstallPackage(pkg)
		}
		fmt.Println("   ✅ 旧扩展已移除")
	}

	// 再安装新扩展
	fmt.Printf("   正在安装扩展: %s\n", latest.Path)
	installPackage(latest.Path)

	// 若已安装同名扩展则 installPackage 会直接返回"已经安装"，
	// 此时提示用户手动覆盖（保持向后兼容，不强制重装）
}

func (p *Parser) parseServerExpr() ASTNode {
	p.eat(DOT)

	switch p.curr.Type {
	case ROUTE_GET, ROUTE_POST, ROUTE_PUT, DELETE:
		method := strings.ToUpper(p.curr.Value)
		if p.curr.Type == DELETE {
			method = "DELETE"
		}
		p.eat(p.curr.Type)
		p.eat(LPAREN)
		path := p.parseExpr()
		p.eat(COMMA)
		handler := p.parseExpr()
		p.eat(RPAREN)
		return ServerRoute{Method: method, Path: path, Handler: handler}

	case LISTEN:
		p.eat(LISTEN)
		p.eat(LPAREN)
		port := p.parseExpr()
		p.eat(RPAREN)
		return ServerListen{Port: port}

	case SERVER_STOP:
		p.eat(SERVER_STOP)
		p.eat(LPAREN)
		p.eat(RPAREN)
		return ServerStop{}

	case SERVER_WAIT:
		p.eat(SERVER_WAIT)
		p.eat(LPAREN)
		p.eat(RPAREN)
		return ServerWait{}

	default:
		panic(fmt.Sprintf("未知的 server 方法: %s", p.curr.Value))
	}
}

// ==================== 服务器功能 ====================

var (
	serverRoutes   = make(map[string]map[string]interface{}) // method -> path -> handler
	serverRoutesMu sync.RWMutex
	serverInstance *http.Server
	serverMu       sync.Mutex
	serverRunning  bool
)

// 路由注册: server.get("/path", handler)
type ServerRoute struct {
	Method  string
	Path    ASTNode
	Handler ASTNode
}

func (sr ServerRoute) Eval(ctx *ExecutionContext) interface{} {
	pathVal := sr.Path.Eval(ctx)
	path, ok := pathVal.(string)
	if !ok {
		panic("server 路由路径必须是字符串")
	}
	handler := sr.Handler.Eval(ctx)

	if _, ok := handler.(FunctionValue); !ok {
		panic(fmt.Sprintf("server.%s 的 handler 必须是函数", strings.ToLower(sr.Method)))
	}

	serverRoutesMu.Lock()
	if serverRoutes[sr.Method] == nil {
		serverRoutes[sr.Method] = make(map[string]interface{})
	}
	serverRoutes[sr.Method][path] = handler
	serverRoutesMu.Unlock()

	return nil
}

// 启动并阻塞服务器: server.listen(port)
type ServerListen struct {
	Port ASTNode
}

func (sl ServerListen) Eval(ctx *ExecutionContext) interface{} {
	portVal := sl.Port.Eval(ctx)
	portF, ok := portVal.(float64)
	if !ok {
		panic("server.listen 端口必须是数字")
	}
	port := int(portF)
	if port < 1 || port > 65535 {
		panic("server.listen 端口必须在 1-65535 之间")
	}

	capturedCtx := ctx

	// ===== 快照当前路由表，避免运行期注册影响本次监听 =====
	routesSnapshot := make(map[string]map[string]interface{})
	serverRoutesMu.RLock()
	for method, paths := range serverRoutes {
		routesSnapshot[method] = make(map[string]interface{}, len(paths))
		for path, h := range paths {
			routesSnapshot[method][path] = h
		}
	}
	serverRoutesMu.RUnlock()

	mux := http.NewServeMux()

	// ===== 单入口：所有请求在这里查表分派，避免重复注册 panic =====
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// 路径是否被任意方法注册过
		pathExists := false
		for _, paths := range routesSnapshot {
			if _, ok := paths[path]; ok {
				pathExists = true
				break
			}
		}
		if !pathExists {
			http.NotFound(w, r)
			return
		}

		// 查当前方法
		if paths, ok := routesSnapshot[r.Method]; ok {
			if handler, ok := paths[path]; ok {
				handleServerRequest(w, r, handler, capturedCtx)
				return
			}
		}

		// 路径存在但方法不匹配
		w.Header().Set("Allow", allowedMethods(routesSnapshot, path))
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	serverMu.Lock()
	serverInstance = srv
	serverRunning = true
	serverMu.Unlock()

	fmt.Printf("[Server] 正在监听 http://localhost:%d\n", port)

	err := srv.ListenAndServe()

	serverMu.Lock()
	serverRunning = false
	serverInstance = nil
	serverMu.Unlock()

	if err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("服务器错误: %v", err))
	}

	return nil
}

// 收集某路径支持的所有方法，供 Allow 头使用
func allowedMethods(routes map[string]map[string]interface{}, path string) string {
	var methods []string
	for method, paths := range routes {
		if _, ok := paths[path]; ok {
			methods = append(methods, method)
		}
	}
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}

// 停止服务器: server.stop()
type ServerStop struct{}

func (ss ServerStop) Eval(ctx *ExecutionContext) interface{} {
	serverMu.Lock()
	if serverInstance == nil || !serverRunning {
		serverMu.Unlock()
		return nil
	}
	srv := serverInstance
	serverRunning = false
	serverInstance = nil
	serverMu.Unlock()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("[Server] 关闭失败: %v\n", err)
	}
	fmt.Println("[Server] 已停止")
	return nil
}

// 等待服务器关闭: server.wait()
type ServerWait struct{}

func (sw ServerWait) Eval(ctx *ExecutionContext) interface{} {
	for {
		serverMu.Lock()
		running := serverRunning
		serverMu.Unlock()
		if !running {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// 处理单个 HTTP 请求
func handleServerRequest(w http.ResponseWriter, r *http.Request, handler interface{}, ctx *ExecutionContext) {
	// 构造请求对象
	query := make(map[string]interface{})
	for k, vs := range r.URL.Query() {
		if len(vs) > 0 {
			query[k] = vs[0]
		}
	}
	headers := make(map[string]interface{})
	for k, vs := range r.Header {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}
	bodyBytes, _ := io.ReadAll(r.Body)

	reqObj := map[string]interface{}{
		"method":  r.Method,
		"path":    r.URL.Path,
		"query":   query,
		"headers": headers,
		"body":    string(bodyBytes),
	}

	var result interface{}

	// 用 defer/recover 保护，防止 handler 崩溃影响服务器
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				if ret, ok := rec.(ReturnValue); ok {
					result = ret.Value
				} else {
					result = map[string]interface{}{
						"status": 500,
						"body":   fmt.Sprintf("服务器内部错误: %v", rec),
					}
				}
			}
		}()

		fv, ok := handler.(FunctionValue)
		if !ok {
			result = map[string]interface{}{"status": 500, "body": "Handler 不是函数"}
			return
		}

		// 构造 handler 执行环境
		newEnv := deepCopyEnv(fv.Env)
		if len(fv.Params) > 0 {
			newEnv[fv.Params[0]] = VarInfo{Type: DictType, Value: reqObj}
		}

		handlerCtx := &ExecutionContext{
			env:        newEnv,
			deferStack: make(map[int][]DeferItem),
			wg:         ctx.wg,
			globalEnv:  ctx.globalEnv,
			globalMu:   ctx.globalMu,
		}

		for _, stmt := range fv.Body {
			stmt.Eval(handlerCtx)
		}
	}()

	writeServerResponse(w, result)
}

// 写出响应
func writeServerResponse(w http.ResponseWriter, result interface{}) {
	switch v := result.(type) {
	case nil:
		w.WriteHeader(http.StatusOK)
	case string:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(v))
	case map[string]interface{}:
		status := http.StatusOK
		if s, ok := v["status"].(float64); ok {
			status = int(s)
		}
		if hs, ok := v["headers"].(map[string]interface{}); ok {
			for k, hv := range hs {
				w.Header().Set(k, fmt.Sprint(hv))
			}
		}
		body := ""
		if b, ok := v["body"]; ok {
			body = fmt.Sprint(b)
		}
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprint(v)))
	}
}
