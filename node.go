package main

import (
	"fmt"
	"strings"
)

type Node struct {
	id                   string
	kind_str             string       //  		[json: 'kind'] 				// e.g. "IntegerLiteral"
	location             NodeLocation //	[json: 'loc']
	range0               Range
	previous_declaration string      //	[json: 'previousDecl']
	name                 string      // e.g. "my_var_name"
	ast_type             AstJsonType //	[json: 'type']
	class_modifier       string      //	[json: 'storageClass']
	tags                 string      //	[json: 'tagUsed']
	initialization_type  string      //	[json: 'init'] 				// "c" => "cinit"
	value                string      // e.g. "777" for IntegerLiteral
	value_number         int         // 		[json: 'value'] 			// For CharacterLiterals, since `value` is a number there, not at string
	opcode               string      // e.g. "+" in BinaryOperator
	ast_argument_type    AstJsonType //	[json: 'argType']
	array_filler         []*Node     // for InitListExpr
	declaration_id       string      //   		[json: 'declId'] 			// for goto labels
	label_id             string      //	[json: 'targetLabelDeclId'] // for goto statements
	is_postfix           bool        //	[json: 'isPostfix']
	ast_line_nr          int

	//parent_node &Node [skip] = unsafe {nil }
	inner                []*Node
	ref_declaration      RefDeclarationNode // [json: 'referencedDecl'] 	//&Node
	kind                 NodeKind
	current_child_id     int
	is_builtin_type      bool
	redeclarations_count int // increased when some *other* Node had previous_decl == this Node.id
}

type NodeLocation struct {
	offset        int
	file          string
	line          int
	source_file   SourceFile // [json: 'includedFrom']
	spelling_file SourceFile // [json: 'spellingLoc']
}

type Range struct {
	begin Begin
}

type Begin struct {
	spelling_file SourceFile // [json: 'spellingLoc']
}

type SourceFile struct {
	path string // [json: 'file']
}

type AstJsonType struct {
	desugared_qualified string // [json: 'desugaredQualType']
	qualified           string // [json: 'qualType']
}

// ???
func (p *AstJsonType) str() string {
	return p.qualified
}

type RefDeclarationNode struct {
	kind_str string // [json: 'kind'] // e.g. "IntegerLiteral"
	name     string
	kind     NodeKind
}

func is_bad_node(n *Node) bool {
	return n.kind != bad
}

func (node *Node) kindof(expected_kind NodeKind) bool {
	return node.kind == expected_kind
}

func (node *Node) has_child_of_kind(expected_kind NodeKind) bool {
	for _, child := range node.inner {
		if child.kindof(expected_kind) {
			return true
		}
	}

	return false
}

func (node *Node) count_children_of_kind(kind_filter NodeKind) int {
	count := 0

	for _, child := range node.inner {
		if child.kindof(kind_filter) {
			count++
		}
	}

	return count
}

func (node *Node) find_children(wanted_kind NodeKind) []*Node {
	suitable_children := []*Node{}

	if len(node.inner) == 0 {
		return suitable_children
	}

	for _, child := range node.inner {
		if child.kindof(wanted_kind) {
			suitable_children = append(suitable_children, child)
		}
	}

	return suitable_children
}

func (node *Node) try_get_next_child_of_kind(wanted_kind NodeKind) *Node {
	if node.current_child_id >= len(node.inner) {
		fmt.Printf("No more children\n")
		return nil
	}

	current_child := node.inner[node.current_child_id]

	if !current_child.kindof(wanted_kind) {
		fmt.Printf("try_get_next_child_of_kind(): WANTED ${%s} BUT GOT ${%s}\n",
			wanted_kind.str(), current_child.kind.str())
		return nil
	}

	node.current_child_id++
	return current_child
}

func (node Node) try_get_next_child() *Node {
	if node.current_child_id >= len(node.inner) {
		fmt.Printf("No more children\n")
		return nil
	}

	current_child := node.inner[node.current_child_id]
	node.current_child_id++

	return current_child
}

func (node *Node) initialize_node_and_children() {
	node.kind = convert_str_into_node_kind(node.kind_str)

	for _, child := range node.inner {
		child.initialize_node_and_children()
	}
}

func (node *Node) is_builtin() bool {
	return node.is_invalid_locations() || line_is_builtin_header(node.location.file) ||
		line_is_builtin_header(node.location.source_file.path) ||
		line_is_builtin_header(node.location.spelling_file.path) ||
		line_is_builtin_header(node.range0.begin.spelling_file.path) ||
		in_builtin_fn_names(node.name)
}

// ?? for Linux and Mac?
var builtin_headers = []string{"usr/include", "/opt/", "usr/lib", "usr/local", "/Library/", "lib/clang"}

func line_is_builtin_header(val string) bool {
	for _, hfile := range builtin_headers {
		if strings.Contains(val, hfile) {
			return true
		}
	}

	return false
}

func (node *Node) is_invalid_locations() bool {
	return node.location.file == "" && node.location.line == 0 && node.location.offset == 0 &&
		node.location.spelling_file.path == "" && node.range0.begin.spelling_file.path == ""
}

func (node *Node) str() string {
	// TODO impl
	return node.name
}

var builtin_fn_names = map[string]int{
	"fopen": 1, "puts": 1, "fflush": 1, "printf": 1, "memset": 1, "atoi": 1, "memcpy": 1, "remove": 1,
	"strlen": 1, "rename": 1, "stdout": 1, "stderr": 1, "stdin": 1, "ftell": 1, "fclose": 1, "fread": 1, "read": 1, "perror": 1,
	"ftruncate": 1, "FILE": 1, "strcmp": 1, "toupper": 1, "strchr": 1, "strdup": 1, "strncasecmp": 1, "strcasecmp": 1,
	"isspace": 1, "strncmp": 1, "malloc": 1, "close": 1, "open": 1, "lseek": 1, "fseek": 1, "fgets": 1, "rewind": 1, "write": 1,
	"calloc": 1, "setenv": 1, "gets": 1, "abs": 1, "sqrt": 1, "erfl": 1, "fprintf": 1, "snprintf": 1, "exit": 1, "__stderrp": 1,
	"fwrite": 1, "scanf": 1, "sscanf": 1, "strrchr": 1, "div": 1, "free": 1, "memcmp": 1, "memmove": 1, "vsnprintf": 1,
	"rintf": 1, "rint": 1,
}

func in_builtin_fn_names(fname string) bool {
	_, exists := builtin_fn_names[fname]
	return exists
}
