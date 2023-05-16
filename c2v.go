package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var go_keywords = map[string]int{
	"break":       1,
	"case":        2,
	"chan":        3,
	"const":       4,
	"continue":    5,
	"default":     6,
	"defer":       7,
	"else":        8,
	"fallthrough": 9,
	"for":         10,
	"func":        11,
	"go":          12,
	"goto":        13,
	"if":          14,
	"import":      15,
	"interface":   16,
	"map":         17,
	"package":     18,
	"range":       19,
	"return":      20,
	"select":      21,
	"struct":      22,
	"switch":      23,
	"type":        24,
	"var":         25,
}

func is_go_keyword(fname string) bool {
	_, exists := go_keywords[fname]
	return exists
}

var builtin_type_names = []string{"ldiv_t", "__float2", "__double2", "exception", "double_t"}

func in_builtin_type_names(tname string) bool {
	for _, v := range builtin_type_names {
		if v == tname {
			return true
		}
	}
	return false
}

var builtin_global_names = []string{"sys_nerr", "sys_errlist", "suboptarg"}

func in_builtin_global_names(tname string) bool {
	for _, v := range builtin_global_names {
		if v == tname {
			return true
		}
	}
	return false
}

var tabs = []string{"", "\t", "\t\t", "\t\t\t", "\t\t\t\t", "\t\t\t\t\t"}

var cur_dir string

type Type struct {
	name      string
	is_const  bool
	is_static bool
}

func find_clang_in_path() string {
	// Empty
	return ""
}

type LabelStmt struct {
	name string
}

type C2V struct {
	tree            *Node
	is_dir          bool // when translating a directory (multiple C=>V files)
	c_file_contents string
	line_i          int
	node_i          int      // when parsing nodes
	unhandled_nodes []string // when coming across an unknown Clang AST node
	// out  stuff
	out                 str_builder       // os.File
	globals_out         map[string]string // `globals_out["myglobal"] == "extern int myglobal = 0;"` // strings.Builder
	out_file            os_file
	out_line_empty      bool
	types               []string            // to avoid dups
	enums               []string            // to avoid dups
	enum_vals           map[string]*str_arr // enum_vals["Color"] = ["green", "blue"], for converting C globals  to enum values
	fns                 []string            // to avoid dups
	outv                string
	cur_file            string
	consts              []string
	globals             map[string]*Global
	inside_switch       int // used to be a bool, a counter to handle switches inside switches
	inside_switch_enum  bool
	inside_for          bool // to handle `;;++i`
	inside_array_index  bool // for enums used as int array index: `if player.weaponowned[.wp_chaingun]`
	global_struct_init  string
	cur_out_line        string
	inside_main         bool
	indent              int
	empty_line          bool // for indents
	is_wrapper          bool
	wrapper_module_name string // name of the wrapper module
	nm_lines            []string
	is_verbose          bool
	skip_parens         bool              // for skipping unnecessary params like in `enum Foo { bar = (1+2) }`
	labels              map[string]string // for goto stmts: `label_stmts[label_id] == "labelname"`
	//
	project_folder string // the final folder passed on the CLI, or the folder of the last file, passed on the CLI. Will be used for searching for a c2v.toml file, containing project configuration overrides, when the C2V_CONFIG env variable is not set explicitly.
	//conf           toml.Doc = empty_toml_doc() // conf will be set by parsing the TOML configuration file
	//
	project_output_dirname   string // by default, "c2v_out.dir"; override with `[project] output_dirname = "another"`
	project_additional_flags string // what to pass to clang, so that it could parse all the input files; mainly -I directives to find additional headers; override with `[project] additional_flags = "-I/some/folder"`
	project_uses_sdl         bool   // if a project uses sdl, then the additional flags will include the result of `sdl2-config --cflags` too; override with `[project] uses_sdl = true`
	file_additional_flags    string // can be added per file, appended to project_additional_flags ; override with `["info.c"] additional_flags = -I/xyz`
	//
	project_globals_path string // where to store the _globals.v file, that will contain all the globals/consts for the project folder; calculated using project_output_dirname and project_folder
	//
	translations            int   // how many translations were done so far
	translation_start_ticks int64 // initialised before the loop calling .translate_file()
	has_cfile               bool
	returning_bool          bool
}

type Global struct {
	name      string
	typ       string
	is_extern bool
}

type NameType struct {
	name string
	typ  Type
}

type str_arr struct {
	inner []string
}

func (p *str_arr) contains(s string) bool {
	for _, v := range p.inner {
		if v == s {
			return true
		}
	}
	return false
}

func (p *str_arr) add(s string) {
	p.inner = append(p.inner, s)
}

type str_builder struct {
	arr str_arr
}

func (p *str_builder) write_string(s string) {
	p.arr.inner = append(p.arr.inner, s)
}

func (p *str_builder) writeln(s string) {
	p.arr.inner = append(p.arr.inner, s) // ?
}

func (p *str_builder) cut_to(pos int) string {
	s := p.str()
	return s[:pos]
}

// helper
func (p *str_builder) str() string {
	s := ""
	for _, val := range p.arr.inner {
		s += val + ", "
	}
	return s
}

func filter_line(s string) string {
	s1 := strings.ReplaceAll(s, "false_", "false")
	s1 = strings.ReplaceAll(s, "true_", "true")
	return s1
}

func (c *C2V) genln(s string) {
	if c.indent > 0 && c.out_line_empty {
		c.out.write_string(tabs[c.indent])
	}
	if c.cur_out_line != "" {
		c.out.write_string(filter_line(c.cur_out_line))
		c.cur_out_line = ""
	}
	c.out.writeln(filter_line(s))
	c.out_line_empty = true
}

func (c *C2V) gen(s string) {
	if c.indent > 0 && c.out_line_empty {
		c.out.write_string(tabs[c.indent])
	}
	c.cur_out_line += s
	c.out_line_empty = false
}

func vprintln(s string) {
	// TODO parse variables in string
	fmt.Printf("%s\n", s)
}

func vprintf(format string, a ...any) {
	// TODO parse variables in string
	fmt.Printf(format, a...)
}

func vprint(s string) {
	// TODO parse variables in string
	fmt.Printf("%s", s)
}

func eprintln(s string) {
	// TODO parse variables in string
	fmt.Printf("%s\n", s)
}

func map2str(smap map[string]string) string {
	s := ""
	for k, v := range smap {
		line := fmt.Sprintf("%s:%s, ", k, v)
		s += line + "\n"
	}

	return s
}

type os_file struct {
}

func (p *os_file) write_string(s string) bool {
	// TODO implement
	return false
}

func (p *os_file) close() {
	// TODO implement
}

func (c *C2V) save() {
	vprintln("\n\n")
	s := c.out.str()
	vprintf("VVVV len=%d\n", len(c.labels))
	vprintln(map2str(c.labels))
	// If there are goto statements, replace all placeholders with actual `goto label_name;`
	// Because JSON AST doesn't have label names for some reason, just IDs.
	if len(c.labels) > 0 {
		for label_name, label_id := range c.labels {
			vprintf(`%v" => "%v\n`, label_id, label_name)
			s = strings.ReplaceAll(s, "_GOTO_PLACEHOLDER_"+label_id, label_name)
		}
	}

	if !c.out_file.write_string(s) {
		// TODO error handling
		panic("failed to write to the .v file: ${err}")
	}
	c.out_file.close()
	if strings.Contains(s, "FILE") {
		c.has_cfile = true
	}

	// unsupported
	/*
		if !c.is_wrapper && !c.outv.contains("st_lib.v") {
			os.system("v fmt -translated -w ${c.outv} > /dev/null")
		}
	*/
}

// recursive
func set_kind_enum(n *Node) {
	for _, child := range n.inner {
		child.kind = convert_str_into_node_kind(child.kind_str)
		if child.ref_declaration.kind_str != "" {
			child.ref_declaration.kind = convert_str_into_node_kind(child.ref_declaration.kind_str)
		}
		if len(child.inner) > 0 {
			set_kind_enum(child)
		}
	}
}

func new_c2v(args []string) *C2V {
	c2v := new(C2V)
	c2v.is_wrapper = false

	//c2v.handle_configuration(args)
	return c2v
}

func json_decode(content string) *Node {
	return nil
}

func (c2v *C2V) add_file(ast_path string, outv string, c_file string) {
	vprintf("new tree(outv=%v c_file=%s)\n", outv, c_file)

	c_file_contents := ""
	if c_file == "" {
		c_file_contents = ""
	} else {
		var err error
		c_file_contents, err = ReadTextFile(c_file)
		if err != nil {
			fmt.Printf("Cannot read file: %s!\n", c_file)
			return
		}
	}
	ast_txt, err := ReadTextFile(ast_path)
	if err != nil {
		vprintln("failed to read ast file " + ast_path + ": ${err}")
		panic(err)
	}
	// TODO parse Json file
	c2v.tree = json_decode(ast_txt)

	c2v.outv = outv
	c2v.c_file_contents = c_file_contents
	c2v.cur_file = c_file

	if c2v.is_wrapper {
		// unsupported by cgogo
	} else {
		// TODO out file
		//c2v.out_file = os.create(c2v.outv)
	}
	c2v.genln("[translated]")
	// Predeclared identifiers
	if !c2v.is_wrapper {
		c2v.genln("module main\n")
	} else if c2v.is_wrapper {
		c2v.genln("module ${c2v.wrapper_module_name}\n")
	}

	// Convert Clang JSON AST nodes to C2V's nodes with extra info. Skip nodes from libc.
	set_kind_enum(c2v.tree)
	for i, node := range c2v.tree.inner {
		vprintf("\nQQQQ %d %s", i, node.name)
		// Builtin types have completely empty "loc" objects:
		// `"loc": {}`
		// Mark them with `is_std`
		if node.is_builtin() {
			vprintf("%v is_std name=%s\n", c2v.line_i, node.name)
			node.is_builtin_type = true
			continue
		} else if line_is_source(node.location.file) {
			vprintf("%d is_source\n", c2v.line_i)
		}
		// if node.name.contains("mobj_t") {
		//}
		vprintf("ADDED TOP NODE line_i=%v\n", c2v.line_i)
	}
	if len(c2v.unhandled_nodes) > 0 {
		vprintln("GOT SOME UNHANDLED NODES:")
		for _, s := range c2v.unhandled_nodes {
			vprintln(s)
		}
		panic("Unknown error")
	}
}

func line_is_source(val string) bool {
	return ends_with(val, ".c")
}

func (c *C2V) func_call(node *Node) {
	expr := node.try_get_next_child()
	c.expr(expr) // this is `func_name(`
	// Clean up macos builtin func names
	// $if macos
	is_memcpy := contains_substr(c.cur_out_line, "__builtin___memcpy_chk")
	is_memmove := contains_substr(c.cur_out_line, "__builtin___memmove_chk")
	is_memset := contains_substr(c.cur_out_line, "__builtin___memset_chk")
	if is_memcpy {
		c.cur_out_line = replace_str(c.cur_out_line, "__builtin___memcpy_chk", "C.memcpy")
	}
	if is_memmove {
		c.cur_out_line = replace_str(c.cur_out_line, "__builtin___memmove_chk", "C.memmove")
	}
	if is_memset {
		c.cur_out_line = replace_str(c.cur_out_line, "__builtin___memset_chk", "C.memset")
	}
	if contains_substr(c.cur_out_line, "memset") {
		vprintf("!! %v\n", c.cur_out_line)
		c.cur_out_line = replace_str(c.cur_out_line, "memset(", "C.memset(")
	}
	// Drop last argument if we have memcpy_chk
	is_m := is_memcpy || is_memmove || is_memset
	len0 := 0
	if is_m {
		len0 = 3
	} else {
		len0 = len(node.inner) - 1
	}
	c.gen("(")
	for i, arg := range node.inner {
		if is_m && i > len0 {
			break
		}
		if i > 0 {
			c.expr(arg)
			if i < len0 {
				c.gen(", ")
			}
		}
	}
	c.gen(")")
}

func to_lower(s string) string {
	return strings.ToLower(s)
}

func capitalize(s string) string {
	return strings.ToUpper(s) // ??
}

func index(s, part string) int {
	return strings.Index(s, part)
}

func before(s, substr string) string {
	pos := strings.Index(s, substr)
	if pos == -1 {
		return s // ???
	}
	return s[:pos]
}

func trim_space(s string) string {
	return strings.TrimSpace(s)
}

func join_strs(arr []string, deli string) string {
	s := ""
	for _, v := range arr {
		s += v + deli
	}
	return s
}

func (c *C2V) func_decl(node *Node, gen_types string) {
	vprintln("1FN DECL name=" + node.name + " cur_file=" + c.cur_file + "")
	c.inside_main = false
	if contains_substr(node.location.file, "usr/include") {
		vprintln("\nskipping func:")
		vprintln("")
		return
	}
	if c.is_dir && ends_with(c.cur_file, "/info.c") {
		// TODO tmp doom hack
		return
	}
	// No statements - it"s a function declration, skip it
	var no_stmts bool
	if !node.has_child_of_kind(compound_stmt) {
		no_stmts = true
	} else {
		no_stmts = false
	}

	vprintf("no_stmts: %v\n", no_stmts)
	for _, child := range node.inner {
		s := fmt.Sprintf("INNER: %d %s", child.kind, child.kind_str)
		vprintln(s)
	}
	// Skip C++ tmpl args
	if node.has_child_of_kind(template_argument) {
		cnt := node.count_children_of_kind(template_argument)
		for i := 0; i < cnt; i++ {
			node.try_get_next_child_of_kind(template_argument)
		}
	}
	name := node.name
	if (name == "invalid") || (name == "referenced") {
		return
	}
	if !c.contains_word(name) {
		vprintln("RRRR ${name} not here, skipping")
		// This func is not found in current .c file, means that it was only
		// in the include file, so it"s declared and used in some other .c file,
		// no need to genenerate it here.
		// TODO perf right now this searches an entire .c file for each global.
		return
	}
	if contains_substr(node.ast_type.qualified, "...)") {
		// TODO handle this better (`...any` ?)
		c.genln("[c2v_variadic]")
	}
	if contains_substr(name, "blkcpy") {
		vprintln("GOT FINISH")
	}
	if c.is_wrapper {
		// unsupported
	}
	c.fns = append(c.fns, name)
	typ := trim_space(before(node.ast_type.qualified, "("))
	if typ == "void" {
		typ = ""
	} else {
		typ = convert_type(typ).name
	}
	if contains_substr(typ, "...") {
		c.gen("F")
	}
	if name == "main" {
		c.inside_main = true
		typ = ""
	}
	if true || contains_substr(name, "Vile") {
		vprintln("\nFN DECL name=" + name + " typ=" + typ + "")
	}

	// Build func args
	params := c.func_params(node)

	str_args := ""
	if name == "main" {
		str_args = ""
	} else {
		str_args = join_strs(params, ", ")
	}
	if !no_stmts || c.is_wrapper {
		c_name := name + gen_types
		if c.is_wrapper {
			//c.genln("func C.${c_name}(${str_args}) ${typ}\n")
		}
		v_name := to_lower(name)
		if v_name != c_name && !c.is_wrapper {
			c.genln(`[c:"${c_name}"]`)
		}
		if c.is_wrapper {
		} else {
			s := fmt.Sprintf("func %s(%s) %s {", v_name, str_args, typ)
			c.genln(s)
		}

		if !c.is_wrapper {
			// For wrapper generation just generate function definitions without bodies
			stmts := node.try_get_next_child_of_kind(compound_stmt)

			c.statements(stmts)
		} else if c.is_wrapper {
		}
	} else {
		lower := to_lower(name)
		if lower != name {
			// This fixes unknown symbols errors when building separate .c => .v files into .o files
			// example:
			//
			// [c: "P_TryMove"]
			// func p_trymove(thing &Mobj_t, x int, y int) bool
			//
			// Now every time `p_trymove` is called, `P_TryMove` will be generated instead.
			c.genln(`[c:"${name}"]`)
		}
		name = lower
		c.genln("func ${name}(${str_args}) ${typ}")
	}
	c.genln("")
	vprintln("END OF FN DECL ast line=${c.line_i}")
}

func (c *C2V) func_params(node *Node) []string {
	str_args := []string{}
	nr_params := node.count_children_of_kind(parm_var_decl)
	for i := 0; i < nr_params; i++ {
		param := node.try_get_next_child_of_kind(parm_var_decl)

		arg_typ := convert_type(param.ast_type.qualified)
		if contains_substr(arg_typ.name, "...") {
			vprintln("vararg: " + arg_typ.name)
		}
		param_name := to_lower(filter_name(param.name))
		str_args = append(str_args, param_name+" "+arg_typ.name)
	}
	return str_args
}

// converts a C type to a V type
func convert_type(typ_ string) Type {
	typ := typ_
	if true || contains_substr(typ, "type_t") {
		vprintln(`\nconvert_type("${typ}")`)
	}

	if contains_substr(typ, "__va_list_tag *") {
		return Type{
			name: "va_list",
		}
	}
	// TODO DOOM hack
	typ = replace_str(typ, "fixed_t", "int")

	is_const := contains_substr(typ, "const ")
	if is_const {
	}
	typ = replace_str(typ, "const ", "")
	typ = replace_str(typ, "volatile ", "")
	typ = replace_str(typ, "std::", "")
	if typ == "char **" {
		return Type{
			name: "&&u8",
		}
	}
	if typ == "void *" {
		return Type{
			name: "voidptr",
		}
	} else if typ == "void **" {
		return Type{
			name: "&voidptr",
		}
	} else if starts_with(typ, "void *[") {
		return Type{
			name: "[" + sub_str(typ, len("void *["), len(typ)-1) + "]voidptr",
		}
	}

	// enum
	if starts_with(typ, "enum ") {
		return Type{
			name:     capitalize(sub_str(typ, len("enum "), len(typ))),
			is_const: is_const,
		}
	}

	// int[3]
	idx := ""
	if contains_substr(typ, "[") && contains_substr(typ, "]") {
		if true {
			pos := index(typ, "[")
			idx = typ[pos:]
			typ = typ[:pos]
		} else {
			idx = after(typ, "[")
			idx = "[" + idx
			typ = before(typ, "[")
		}
	}
	// leveldb::DB
	if contains_substr(typ, "::") {
		typ = after(typ, "::")
	} else if contains_substr(typ, ":") {
		// boolean:boolean
		typ = all_before(typ, ":")
	}
	typ = replace_str(typ, " void *", "voidptr")

	// char*** => ***char
	base := trim_space(typ)
	base = strings.ReplaceAll(base, "struct ", "") //, "signed ", ""])???
	if starts_with(base, "signed ") {
		// "signed char" == "char", so just ignore "signed "
		// TODO ???
		//base = base["signed ".len..]
	}
	if ends_with(base, "*") {
		base = before(base, " *")
	}

	/*
		// TODO ???
		base = match base {
			"long long" {
				"i64"
			}
			"long" {
				"int"
			}
			"unsigned int" {
				"u32"
			}
			"unsigned long long" {
				"i64"
			}
			"unsigned long" {
				"u32"
			}
			"unsigned char" {
				"u8"
			}
			"*unsigned char" {
				"&u8"
			}
			"unsigned short" {
				"u16"
			}
			"uint32_t" {
				"u32"
			}
			"int32_t" {
				"int"
			}
			"uint64_t" {
				"u64"
			}
			"int64_t" {
				"i64"
			}
			"int16_t" {
				"i16"
			}
			"uint8_t" {
				"u8"
			}
			"__int64_t" {
				"i64"
			}
			"__int32_t" {
				"int"
			}
			"__uint32_t" {
				"u32"
			}
			"__uint64_t" {
				"u64"
			}
			"short" {
				"i16"
			}
			"char" {
				"i8"
			}
			"float" {
				"f32"
			}
			"double" {
				"f64"
			}
			"byte" {
				"u8"
			}
			//  just to avoid capitalizing these:
			"int" {
				"int"
			}
			"voidptr" {
				"voidptr"
			}
			"intptr_t" {
				"C.intptr_t"
			}
			"void" {
				"void"
			}
			"u32" {
				"u32"
			}
			"size_t" {
				"usize"
			}
			"ptrdiff_t" {
				"isize"
			}
			"boolean", "_Bool", "Bool", "bool (int)", "bool" {
				"bool"
			}
			"FILE" {
				"C.FILE"
			}
			else {
				trim_underscores(base.capitalize())
			}
		}
	*/
	amps := ""

	if ends_with(typ, "*") {
		star_pos := index(typ, "*")

		nr_stars := len(typ[star_pos:])
		amps = repeat(`&`, nr_stars)
		typ = amps + base
	} else if contains_substr(typ, "(*)") {
		// func type
		// int (*)(void *, int, char **, char **)
		// func (voidptr, int, *byteptr, *byteptr) int
		ret_typ := convert_type(all_before(typ, "("))
		s := "func ("
		// move func to the right place
		typ = replace_str(typ, "(*)", " ")
		// handle each arg
		sargs := find_between(typ, "(", ")")
		args := split(sargs, ",")
		for i, arg := range args {
			t := convert_type(arg)
			s += t.name
			if i < len(args)-1 {
				s += ", "
			}
		}
		// Function doesn't return anything
		if ret_typ.name == "void" {
			typ = s + ")"
		} else {
			typ = "${s}) ${ret_typ.name}"
		}
		// C allows having func(void) instead of func()
		typ = replace_str(typ, "(void)", "()")
	} else {
		typ = base
	}
	// User & => &User
	if ends_with(typ, " &") {
		typ = typ[:len(typ)-2]
		base = typ
		typ = "&" + typ
	}
	typ = trim_space(typ)
	if contains_substr(typ, "&& ") {
		typ = replace_str(typ, " ", "")
	}
	if contains_substr(typ, " ") {
	}
	vprintln(`"${typ_}" => "${typ}" base="${base}"`)

	name := idx + typ
	return Type{
		name:     name,
		is_const: is_const,
	}
}

// |-RecordDecl 0x7fd7c302c560 <a.c:3:1, line:5:1> line:3:8 struct User definition
func (c *C2V) record_decl(node *Node) {
	vprintln(`record_decl("${node.name}")`)
	// Skip empty structs (extern or forward decls)
	if node.kindof(record_decl) && len(node.inner) == 0 {
		return
	}
	name := node.name
	// Dont generate struct header if it was already generated by typedef
	// Confusing, but typedefs in C AST are really messy.
	// ...
	// If the struct has no name, then it"s `typedef struct { ... } name`
	// AST: 1) RecordDecl struct definition 2) TypedefDecl struct name

	if len(c.tree.inner) > c.node_i+1 {
		next_node := c.tree.inner[c.node_i+1]

		if next_node.kind == typedef_decl {
			if c.is_verbose {
				c.genln("// typedef struct")
			}

			name = next_node.name

			if contains_substr(name, "apthing_t") {
				vprintln(node.str())
			}
		}
	}

	if in_builtin_type_names(name) {
		return
	}
	if c.is_verbose {
		c.genln(`// struct decl name="${name}"`)
	}
	if c.in_c_types(name) {
		return
	}
	if (name != "struct") && (name != "union") {
		c.types = append(c.types, name)
		name = capitalize_type(name)
		if contains_substr(node.tags, "union") {
			c.genln("union ${name} { ")
		} else {
			c.genln("struct ${name} { ")
		}
	}
	for _, field := range node.inner {
		// There may be comments, skip them
		if field.kind != field_decl {
			continue
		}
		field_type := convert_type(field.ast_type.qualified)
		field_name := filter_name(field.name)
		if contains(field_type.name, "anonymous at") {
			continue
		}
		/*
			if field_type.name.contains("union") {
				continue // TODO
			}
		*/
		if ends_with(field_type.name, "_s") { // TODO doom _t _s hack, remove
			n := field_type.name[:len(field_type.name)-2] + "_t"
			c.genln(fmt.Sprintf("\t%s %s", field_name, n))
		} else {
			c.genln(fmt.Sprintf("\t%s %s", field_name, field_type.name))
		}
	}
	c.genln("}")
}

func (c *C2V) in_c_types(s string) bool {
	for _, v := range c.types {
		if v == s {
			return true
		}
	}

	return false
}

func (c *C2V) in_c_enums(s string) bool {
	for _, v := range c.enums {
		if v == s {
			return true
		}
	}

	return false
}

// Typedef node goes after struct enum, but we need to parse it first, so that "type name { " is
// generated first
func (c *C2V) typedef_decl(node *Node) {
	typ := node.ast_type.qualified
	// just a single line typedef: (alias)
	// typedef sha1_context_t sha1_context_s ;
	// typedef after enum decl, just generate "enum NAME {" header
	alias_name := node.name // get_val(-2)
	vprintln(`TYPEDEF "${node.name}" ${node.is_builtin_type} ${typ}`)
	if contains(alias_name, "et_context_t") {
		// TODO remove this
		return
	}
	if in_builtin_type_names(node.name) {
		return
	}
	if !contains(typ, alias_name) {
		if contains(typ, "(*)") {
			tt := convert_type(typ)
			typ = tt.name
		} else {
			// Struct types have junk before spaces
			alias_name = all_after(alias_name, " ")
			tt := convert_type(typ)
			typ = tt.name
		}
		if starts_with(alias_name, "__") {
			// Skip internal stuff like __builtin_ms_va_list
			return
		}
		if c.in_c_types(alias_name) || c.in_c_enums(alias_name) {
			// This means that this is a struct/enum typedef that has already been defined.
			return
		}
		if c.in_c_enums(typ) {
			return
		}
		c.types = append(c.types, alias_name)
		cgen_alias := typ
		if starts_with(cgen_alias, "_") {
			cgen_alias = trim_underscores(typ)
		}
		if !is_ptr_size(typ) && !starts_with(typ, "func (") {
			// TODO handle this better
			cgen_alias = capitalize(cgen_alias)
		}
		c.genln("type ${alias_name.capitalize()} = ${cgen_alias}") // typedef alias (SINGLE LINE)")
		return
	}
	if contains(typ, "enum ") {
		// enums were alredy handled in enum_decl
		return
	} else if contains(typ, "struct ") {
		// structs were already handled in struct_decl
		return
	} else if contains(typ, "union ") {
		// unions were alredy handled in struct_decl
		return
	}
}

func is_ptr_size(s string) bool {
	arr := []string{"int", "i8", "i16", "i64", "u8", "u16", "u32", "u64", "f32", "f64", "usize", "isize", "bool", "void", "voidptr"}
	for _, v := range arr {
		if s == v {
			return true
		}
	}
	return false
}

// this calls typedef_decl() above
func (c *C2V) parse_next_typedef() bool {
	// Hack: typedef with the actual enum name is next, parse it and generate "enum NAME {" first
	/*
		XTODO
		next_line := c.lines[c.line_i + 1]
		if next_line.contains("TypedefDecl") {
			c.line_i++
			c.parse_next_node()
			return true
		}
	*/
	return false
}

func (c *C2V) in_consts(s string) bool {
	for _, v := range c.consts {
		if v == s {
			return true
		}
	}

	return false
}

func (c *C2V) enum_decl(node *Node) {
	// Hack: typedef with the actual enum name is next, parse it and generate "enum NAME {" first
	enum_name := node.name //""
	if len(c.tree.inner) > c.node_i+1 {
		next_node := c.tree.inner[c.node_i+1]
		if next_node.kind == typedef_decl {
			enum_name = next_node.name
		}
	}
	if enum_name == "boolean" {
		return
	}
	if enum_name == "" {
		// empty enum means it"s just a list of #define"ed consts
		c.genln("\nconst ( // empty enum")
	} else {
		enum_name = replace_str(capitalize(enum_name), "Enum ", "")
		if c.in_c_enums(enum_name) {
			return
		}

		c.genln("enum ${enum_name} {")
	}
	vals := c.enum_vals[enum_name]
	for i, child := range node.inner {
		name := filter_name(to_lower(child.name))
		vals.add(name)
		has_anon_generated := false
		// empty enum means it"s just a list of #define"ed consts
		if enum_name == "" {
			if !starts_with(name, "_") && c.in_consts(name) {
				c.consts = append(c.consts, name)
				c.gen("\t${name}")
				has_anon_generated = true
			}
		} else {
			c.gen("\t" + name)
		}
		// handle custom enum vals, e.g. `MF_SHOOTABLE = 4`
		if len(child.inner) > 0 {
			const_expr := child.try_get_next_child()
			if const_expr.kind == constant_expr {
				c.gen(" = ")
				c.skip_parens = true
				c.expr(const_expr.try_get_next_child())
				c.skip_parens = false
			}
		} else if has_anon_generated {
			c.genln(fmt.Sprintf(" = %d", i))
		}
	}
	if enum_name != "" {
		vprintln(`decl enum "${enum_name}" with ${vals.len} vals`)
		c.enum_vals[enum_name] = vals
		c.genln("}\n")
	} else {
		c.genln(")\n")
	}
	if enum_name != "" {
		c.enums = append(c.enums, enum_name)
	}
}

func (c *C2V) statements(compound_stmt *Node) {
	c.indent++
	// Each CompoundStmt"s child is a statement
	for i, _ := range compound_stmt.inner {
		c.statement(compound_stmt.inner[i])
	}
	c.indent--
	c.genln("}")
}

func (c *C2V) statements_no_rcbr(compound_stmt *Node) {
	for i, _ := range compound_stmt.inner {
		c.statement(compound_stmt.inner[i])
	}
}

func (c *C2V) statement(child *Node) {
	if child.kindof(decl_stmt) {
		c.var_decl(child)
		c.genln("")
	} else if child.kindof(return_stmt) {
		c.return_st(child)
		c.genln("")
	} else if child.kindof(if_stmt) {
		c.if_statement(child)
	} else if child.kindof(while_stmt) {
		c.while_st(child)
	} else if child.kindof(for_stmt) {
		c.for_st(child)
	} else if child.kindof(do_stmt) {
		c.do_st(child)
	} else if child.kindof(switch_stmt) {
		c.switch_st(child)
	} else if child.kindof(compound_stmt) {
		// Just  { }
		c.genln("{")
		c.statements(child)
	} else if child.kindof(gcc_asm_stmt) {
		c.genln("__asm__") // TODO
	} else if child.kindof(goto_stmt) {
		c.goto_stmt(child)
	} else if child.kindof(label_stmt) {
		label := child.name // child.get_val(-1)
		c.labels[child.name] = child.declaration_id
		c.genln("/*RRRREG ${child.name} id=${child.declaration_id} */")
		c.genln(fmt.Sprintf("%s: ", label))
		c.statements_no_rcbr(child)
	} else if child.kindof(cxx_for_range_stmt) {
		// C++
		c.for_range(child)
	} else {
		c.expr(child)
		c.genln("")
	}
}

func (c *C2V) goto_stmt(node *Node) {
	label := c.labels[node.label_id]
	if label == "" {
		label = "_GOTO_PLACEHOLDER_" + node.label_id
	}
	c.genln("goto ${label} /* id: ${node.label_id} */")
}

func (c *C2V) return_st(node *Node) {
	c.gen("return ")
	// returning expression?
	if len(node.inner) > 0 && !c.inside_main {
		expr := node.try_get_next_child()
		if expr.kindof(implicit_cast_expr) {
			if expr.ast_type.qualified == "bool" {
				// Handle `return 1` which is actually `return true`
				c.returning_bool = true
			}
		}
		c.expr(expr)
		c.returning_bool = false
	}
}

func (c *C2V) if_statement(node *Node) {
	expr := node.try_get_next_child()
	c.gen("if ")
	c.gen_bool(expr)
	// Main if block
	child := node.try_get_next_child()
	if child.kindof(null_stmt) {
		// The if branch body can be empty (`if (foo) ;`)
		c.genln(" {/* empty if */}")
	} else {
		c.st_block(child)
	}
	// Optional else block
	else_st := node.try_get_next_child()
	if else_st.kindof(compound_stmt) || else_st.kindof(return_stmt) {
		c.genln("else {")
		c.st_block_no_start(else_st)
	} else if else_st.kindof(if_stmt) {
		c.gen("else ")
		c.if_statement(else_st)
	} else if !else_st.kindof(bad) && !else_st.kindof(null0) {
		// `else expr() ;` else statement in one line without {}
		c.genln("else { // 3")
		c.expr(else_st)
		c.genln("\n}")
	}
}

func (c *C2V) while_st(node *Node) {
	c.gen("for ")
	expr := node.try_get_next_child()
	c.gen_bool(expr)
	c.genln(" {")
	stmts := node.try_get_next_child()
	c.st_block_no_start(stmts)
}

func (c *C2V) for_st(node *Node) {
	c.inside_for = true
	c.gen("for ")
	// Can be "for (int i = ...)"
	if node.has_child_of_kind(decl_stmt) {
		decl_stmt := node.try_get_next_child_of_kind(decl_stmt)

		c.var_decl(decl_stmt)
	} else {
		// Or "for (i = ....)"
		expr := node.try_get_next_child()
		c.expr(expr)
	}
	c.gen(" ; ")
	expr2 := node.try_get_next_child()
	if expr2.kind_str == "" {
		// second cond can be Null
		expr2 = node.try_get_next_child()
	}
	c.expr(expr2)
	c.gen(" ; ")
	expr3 := node.try_get_next_child()
	c.expr(expr3)
	c.inside_for = false
	child := node.try_get_next_child()
	c.st_block(child)
}

func (c *C2V) do_st(node *Node) {
	c.genln("for {")
	child := node.try_get_next_child()
	c.statements_no_rcbr(child)
	// TODO condition
	c.genln("// while()")
	c.gen("if ! (")
	expr := node.try_get_next_child()
	c.expr(expr)
	c.genln(" ) { break }")
	c.genln("}")
}

func (c *C2V) case_st(child *Node, is_enum bool) bool {
	if child.kindof(case_stmt) {
		if is_enum {
			// Force short `.val {` enum syntax, but only in `case .val:`
			// Later on it"ll be set to false, so that full syntax is used (`Enum.val`)
			// Since enums are often used as ints, and V will need the full enum
			// value to convert it to ints correctly.
			c.inside_switch_enum = true
		}
		c.gen(" ")
		case_expr := child.try_get_next_child()
		c.expr(case_expr)
		a := child.try_get_next_child()
		if a.kindof(null0) {
			a = child.try_get_next_child()
		}
		vprintln("A TYP=${a.ast_type}")
		if a.kindof(compound_stmt) {
			c.genln("// case comp stmt")
			c.statements(a)
		} else if a.kindof(case_stmt) {
			// case 1:
			// case 2:
			// case 3:
			// ===>
			// case 1, 2, 3:
			for a.kindof(case_stmt) {
				e := a.try_get_next_child()
				c.gen(", ")
				c.expr(e) // this is `1` in `case 1:`
				tmp := a.try_get_next_child()
				if tmp.kindof(null0) {
					tmp = a.try_get_next_child()
				}
				a = tmp
			}
			c.genln("{")
			vprintln("!!!!!!!!caseexpr=")
			c.inside_switch_enum = false
			c.statement(a)
		} else if a.kindof(default_stmt) {
		} else {
			// case body
			c.inside_switch_enum = false
			c.genln("// case comp body kind=${a.kind} is_enum=${is_enum} ")
			c.genln("{")
			c.statement(a)
			if a.kindof(return_stmt) {
			} else if a.kindof(break_stmt) {
				return true
			}
			if is_enum {
				c.inside_switch_enum = true
			}
		}
	}
	return false
}

// Switch statements are a mess in C...
func (c *C2V) switch_st(switch_node *Node) {
	c.gen("match ")
	c.inside_switch++
	expr := switch_node.try_get_next_child()
	is_enum := false
	if len(expr.inner) > 0 {
		// 0
		x := expr.inner[0]
		if x.ast_type.qualified == "int" {
			// this is an int, not a C enum type
			c.inside_switch_enum = false
		} else {
			c.inside_switch_enum = true
			is_enum = true
		}
	}
	comp_stmt := switch_node.try_get_next_child()
	// Detect if this switch statement runs on an enum (have to look at the first
	// value being compared). This means that the integer will have to be cast to this enum
	// in V.
	// switch (x) { case enum_val: ... }   ==>
	// match MyEnum(x) { .enum_val { ... } }
	// Don't cast if it"s already an enum and not an int. Enum(enum) compiles, but still.
	second_par := false
	if len(comp_stmt.inner) > 0 {
		child := comp_stmt.inner[0]
		if child.kindof(case_stmt) {
			case_expr := child.try_get_next_child()
			if case_expr.kindof(constant_expr) {
				x := case_expr.try_get_next_child()
				vprintln("YEP")

				if x.ref_declaration.kind == enum_constant_decl {
					is_enum = true
					c.inside_switch_enum = true
					c.gen(c.enum_val_to_enum_name(x.ref_declaration.name))

					c.gen("(")
					second_par = true
				}
			}
		}
	}
	// Now the opposite. Detect if the switch runs on a C int which is an enum in V.
	// switch (x) { case enum_val: ... }   ==>
	// match (x) { int(.enum_val) { ... } }

	//
	c.expr(expr)
	if is_enum {
	}
	if second_par {
		c.gen(")")
	}
	// c.inside_switch_enum = false
	c.genln(" {")
	default_node := new(Node)
	got_else := false
	// Switch AST node is weird. First child is a CaseStmt that contains a single child
	// statement (the first in the block). All other statements in the block are siblings
	// of this CaseStmt:
	// switch (x) {
	//   case 1:
	//     line1(); // child of CaseStmt
	//     line2(); // CallExpr (sibling of CaseStmt)
	//     line3(); // CallExpr (sibling of CaseStmt)
	// }
	has_case := false
	for i, child := range comp_stmt.inner {
		if child.kindof(case_stmt) {
			if i > 0 && has_case {
				c.genln("}")
			}
			c.case_st(child, is_enum)
			has_case = true
		} else if child.kindof(default_stmt) {
			default_node = child.try_get_next_child()
			got_else = true
		} else {
			// handle weird children-siblings
			c.inside_switch_enum = false
			c.statement(child)
		}
	}
	if got_else {
		if default_node.kind != bad {
			if default_node.kindof(case_stmt) {
				c.case_st(default_node, is_enum)
				c.genln("}")
				c.genln("else {")
			} else {
				c.genln("}")
				c.genln("else {")
				c.statement(default_node)
			}
			c.genln("}")
		}
	} else {
		if has_case {
			c.genln("}")
		}
		c.genln("else{}")
	}
	c.genln("}")
	c.inside_switch--
	c.inside_switch_enum = false
}

func (c *C2V) st_block_no_start(node *Node) {
	c.st_block2(node, false)
}

func (c *C2V) st_block(node *Node) {
	c.st_block2(node, true)
}

// {} or just one statement if there is no {
func (c *C2V) st_block2(node *Node, insert_start bool) {
	if insert_start {
		c.genln(" {")
	}
	if node.kindof(compound_stmt) {
		c.statements(node)
	} else {
		// No {}, just one statement
		c.statement(node)
		c.genln("}")
	}
}

//
func (c *C2V) gen_bool(node *Node) {
	typ := c.expr(node)
	if typ == "int" {
	}
}

func (c *C2V) var_decl(decl_stmt *Node) {
	for i := 0; i < len(decl_stmt.inner); i++ {
		//for _, _ := range decl_stmt.inner {
		var_decl := decl_stmt.try_get_next_child()
		if var_decl.kindof(record_decl) || var_decl.kindof(enum_decl) {
			return
		}
		if var_decl.class_modifier == "extern" {
			vprintln("local extern vars are not supported yet: ")
			vprintln(var_decl.str())
			vprintln(c.cur_file + ":" + fmt.Sprintf("%d", c.line_i))
			panic(1)
			return
		}
		// cinit means we have an initialization together with var declaration:
		// `int a = 0;`
		cinit := var_decl.initialization_type == "c"
		name := to_lower(filter_name(var_decl.name))
		typ_ := convert_type(var_decl.ast_type.qualified)
		if typ_.is_static {
			c.gen("static ")
		}
		if cinit {
			expr := var_decl.try_get_next_child()
			c.gen(fmt.Sprintf("%s := ", name))
			c.expr(expr)
			if len(decl_stmt.inner) > 1 {
				c.gen("\n")
			}
		} else {
			oldtyp := var_decl.ast_type.qualified
			typ := typ_.name
			vprintln(`oldtyp="${oldtyp}" typ="${typ}"`)
			// set default zero value (V requires initialization)
			def := ""
			if starts_with(var_decl.ast_type.desugared_qualified, "struct ") {
				def = "${typ}{}" // `struct Foo foo;` => `foo := Foo{}` (empty struct init)
			} else if typ == "u8" {
				def = "u8(0)"
			} else if typ == "u16" {
				def = "u16(0)"
			} else if typ == "u32" {
				def = "u32(0)"
			} else if typ == "u64" {
				def = "u64(0)"
			} else if (typ == "size_t") || (typ == "usize") {
				def = "usize(0)"
			} else if typ == "i8" {
				def = "i8(0)"
			} else if typ == "i16" {
				def = "i16(0)"
			} else if typ == "int" {
				def = "0"
			} else if typ == "i64" {
				def = "i64(0)"
			} else if (typ == "ptrdiff_t") || (typ == "isize") {
				def = "isize(0)"
			} else if typ == "bool" {
				def = "false"
			} else if typ == "f32" {
				def = "f32(0.0)"
			} else if typ == "f64" {
				def = "0.0"
			} else if typ == "boolean" {
				def = "false"
			} else if ends_with(oldtyp, "*") {
				// *sqlite3_mutex ==>
				// &sqlite3_mutex{!}
				// println2("!!! $oldtyp $typ")
				// def = "&${typ.right(1)}{!}"
				var tt string
				if starts_with(typ, "&") {
					tt = typ[1:]
				} else {
					tt = typ
				}
				def = fmt.Sprintf("&%s(0)", tt)
			} else if starts_with(typ, "[") {
				// Empty array init
				def = "${typ}{}"
			} else {
				// We assume that everything else is a struct, because C AST doesn't
				// give us any info that typedef"ed structs are structs

				if contains_any_substr(oldtyp, []string{"dirtype_t", "angle_t"}) { // TODO DOOM handle int aliases
					def = "u32(0)"
				} else {
					def = "${typ}{}"
				}
			}
			// vector<int> => int => []int
			if starts_with(typ, "vector<") {
				def = sub_str(typ, len("vector<"), len(typ)-1)
				def = fmt.Sprintf("[]%s", def)
			}
			c.gen("${name} := ${def}")
			if len(decl_stmt.inner) > 1 {
				c.genln("")
			}
		}
	}
}

func (c *C2V) global_var_decl(var_decl *Node) {
	// if the global has children, that means it"s initialized, parse the expression
	is_inited := len(var_decl.inner) > 0

	vprintf("\nglobal name=%s typ=%v\n", var_decl.name, var_decl.ast_type.qualified)
	vprintln(var_decl.str())

	name := filter_name(var_decl.name)

	if starts_with(var_decl.ast_type.qualified, "[]") {
		return
	}
	typ := convert_type(var_decl.ast_type.qualified)
	existing, has := c.globals[var_decl.name]
	if has {
		if !types_are_equal(existing.typ, typ.name) {
			c.verror(`Duplicate global "${var_decl.name}" with different types:"${existing.typ}" and	"${typ.name}".
Since C projects do not use modules but header files, duplicate globals are allowed.
This will not compile in V, so you will have to modify one of the globals and come up with a
unique name`)
		}
		if !existing.is_extern {
			c.genln(`// skipping global dup "${var_decl.name}"`)
			return
		}
	}
	// Skip extern globals that are initialized later in the file.
	// We"ll have go thru all top level nodes, find a VarDecl with the same name
	// and make sure it"s inited (has a child expressinon).
	is_extern := var_decl.class_modifier == "extern"
	if is_extern && !is_inited {
		for _, x := range c.tree.inner {
			if x.kindof(var_decl.kind) && (x.name == var_decl.name) && x.id != var_decl.id {
				if len(x.inner) > 0 {
					c.genln("// skipped extern global ${x.name}")
					return
				}
			}
		}
	}
	// We assume that if the global"s type is `[N]array`, and it"s initialized,
	// then it"s constant
	is_fixed_array := contains(var_decl.ast_type.qualified, "]") &&
		contains(var_decl.ast_type.qualified, "]")
	is_const := is_inited && (typ.is_const || is_fixed_array)
	if true || !contains(typ.name, "[") {
	}
	if c.is_wrapper && starts_with(typ.name, "_") {
		return
	}
	if c.is_wrapper {
		return
	}
	if !c.is_dir && is_extern && var_decl.redeclarations_count > 0 {
		// This is an extern global, and it"s declared later in the file without `extern`.
		return
	}
	// Cut generated code from `c.out` to `c.globals_out`
	start := len(c.out.arr.inner)
	if is_const {
		c.consts = append(c.consts, name)
		c.gen(`[export:"${name}"]\nconst (\n${name}  `)
	} else {
		if !c.contains_word(name) && !contains(c.cur_file, "deh_") { // TODO deh_ hack remove
			vprintf("RRRR global %s not here, skipping\n", name)
			// This global is not found in current .c file, means that it was only
			// in the include file, so it"s declared and used in some other .c file,
			// no need to genenerate it here.
			// TODO perf right now this searches an entire .c file for each global.
			return
		}
		if in_builtin_global_names(name) {
			return
		}

		if is_inited {
			c.gen("/*!*/[weak] __global ( ${name} ")
		} else {
			if contains(typ.name, "anonymous enum") || contains(typ.name, "unnamed enum") {
				// Skip anon enums, they are declared as consts in V
				return
			}

			if is_extern && is_fixed_array && var_decl.redeclarations_count == 0 {
				c.gen("[c_extern]")
			} else {
				c.gen("[weak]")
			}
			c.gen("__global ( ${name} ${typ.name} ")
		}
		c.global_struct_init = typ.name
	}
	if is_fixed_array && contains(var_decl.ast_type.qualified, "[]") &&
		!contains(var_decl.ast_type.qualified, "*") && !is_inited {
		// Do not allow uninitialized fixed arrays for now, since they are not supported by V
		eprintln(`${c.cur_file}: uninitialized fixed array without the size "${name}" typ="${var_decl.ast_type.qualified}"`)
		panic(1)
	}

	// if the global has children, that means it"s initialized, parse the expression
	if is_inited {
		child := var_decl.try_get_next_child()
		c.gen(" = ")
		is_struct := child.kindof(init_list_expr) && !is_fixed_array
		needs_cast := !is_const && !is_struct // Don't generate `foo=Foo(Foo{` if it"s a struct init
		if needs_cast {
			c.gen(typ.name + " (") ///* typ=$typ   KIND= $child.kind isf=$is_fixed_array*/(")
		}
		c.expr(child)
		if needs_cast {
			c.gen(")")
		}
		c.genln("")
	} else {
		c.genln("\n")
	}
	if true {
		c.genln(")\n")
	}
	if c.is_dir {
		s := c.out.cut_to(start)
		c.globals_out[name] = s
	}
	c.global_struct_init = ""
	c.globals[name] = &Global{
		name:      name,
		is_extern: is_extern,
		typ:       typ.name,
	}
}

// `"red"` => `"Color"`
func (c *C2V) enum_val_to_enum_name(enum_val string) string {
	filtered_enum_val := filter_name(enum_val)
	for enum_name, vals := range c.enum_vals {
		if vals.contains(filtered_enum_val) {
			return enum_name
		}
	}
	return ""
}

// expr is a spcial one. we dont know what type node has.
// can be multiple.
func (c *C2V) expr(_node *Node) string {
	node := _node
	// Just gen a number
	if node.kindof(null0) {
		return ""
	}
	if node.kindof(integer_literal) {
		if c.returning_bool {
			if node.value == "1" {
				c.gen("true")
			} else {
				c.gen("false")
			}
		} else {
			c.gen(node.value)
		}
	} else if node.kindof(character_literal) {
		// "a"
		//c.gen("`" + rune(node.value_number).str() + "`")
		c.gen("`" + fmt.Sprintf("%c", node.value_number) + "`")
	} else if node.kindof(floating_literal) {
		// 1e80
		c.gen(node.value)
	} else if node.kindof(constant_expr) {
		n := node.try_get_next_child()
		c.expr(n)
	} else if node.kindof(null_stmt) {
		// null
		c.gen("0 /* null */")
	} else if node.kindof(cold_attr) {
	} else if node.kindof(binary_operator) {
		// = + - *
		op := node.opcode
		first_expr := node.try_get_next_child()
		c.expr(first_expr)
		c.gen(" ${op} ")
		second_expr := node.try_get_next_child()

		if second_expr.kindof(binary_operator) && second_expr.opcode == "=" {
			// handle `a = b = c` => `a = c; b = c;`
			second_child_expr := second_expr.try_get_next_child() // `b`
			third_expr := second_expr.try_get_next_child()        // `c`
			c.expr(third_expr)
			c.genln("")
			c.expr(second_child_expr)
			c.gen(" = ")
			first_expr.current_child_id = 0
			c.expr(first_expr)
			c.gen("")
			second_expr.current_child_id = 0
		} else {
			c.expr(second_expr)
		}
		vprintln("done!")
		if op == "<" || op == ">" || op == "==" {
			return "bool"
		}
	} else if node.kindof(compound_assign_operator) {
		// +=
		op := node.opcode // get_val(-3)
		first_expr := node.try_get_next_child()
		c.expr(first_expr)
		c.gen(fmt.Sprintf(" %v ", op))
		second_expr := node.try_get_next_child()
		c.expr(second_expr)
	} else if node.kindof(unary_operator) {
		// ++ --
		op := node.opcode
		expr := node.try_get_next_child()
		if (op == "--") || (op == "++") {
			c.expr(expr)
			c.gen(" ${op}")
			if !c.inside_for && !node.is_postfix {
				// prefix ++
				// but do not generate `++i` in for loops, it breaks in V for some reason
				c.gen("$")
			}
		} else if op == "-" || op == "&" || op == "*" || op == "!" || op == "~" {
			c.gen(op)
			c.expr(expr)
		}
	} else if node.kindof(paren_expr) {
		// ()
		if !c.skip_parens {
			c.gen("(")
		}
		child := node.try_get_next_child()
		c.expr(child)
		if !c.skip_parens {
			c.gen(")")
		}
	} else if node.kindof(implicit_cast_expr) {
		// This junk means go again for its child
		expr := node.try_get_next_child()
		c.expr(expr)
	} else if node.kindof(decl_ref_expr) {
		// var  name
		c.name_expr(node)
	} else if node.kindof(string_literal) {
		// "string literal"
		str := node.value
		// "a" => "a"
		no_quotes := sub_str(str, 1, len(str)-1)
		if contains(no_quotes, `"`) {
			// same quoting logic as in vfmt
			c.gen(`c"${no_quotes}"`)
		} else {
			c.gen(`c"${no_quotes}"`)
		}
	} else if node.kindof(call_expr) {
		// func call
		c.func_call(node)
	} else if node.kindof(member_expr) {
		// `user.age`
		field := node.name
		expr := node.try_get_next_child()
		c.expr(expr)
		field = replace_str(field, "->", "")
		if starts_with(field, ".") {
			field = filter_name(field[1:])
		} else {
			field = filter_name(field)
		}
		c.gen(".${field}")
	} else if node.kindof(unary_expr_or_type_trait_expr) {
		// sizeof
		c.gen("sizeof")
		// sizeof (expr) ?
		if len(node.inner) > 0 {
			expr := node.try_get_next_child()
			c.expr(expr)
		} else {
			// sizeof (Type) ?
			typ := convert_type(node.ast_argument_type.qualified)
			c.gen(fmt.Sprintf("(%v)", typ.name))
		}
	} else if node.kindof(array_subscript_expr) {
		// a[0]
		first_expr := node.try_get_next_child()
		c.expr(first_expr)
		c.gen(" [")

		second_expr := node.try_get_next_child()
		c.inside_array_index = true
		c.expr(second_expr)
		c.inside_array_index = false
		c.gen("] ")
	} else if node.kindof(init_list_expr) {
		// int a[] = {1,2,3};
		c.init_list_expr(node)
	} else if node.kindof(c_style_cast_expr) {
		// (int*)a  => (int*)(a)
		// CStyleCastExpr "const char **" <BitCast>
		expr := node.try_get_next_child()
		typ := convert_type(node.ast_type.qualified)
		cast := typ.name
		if contains(cast, "*") {
			cast = "(${cast})"
		}
		c.gen("${cast}(")
		c.expr(expr)
		c.gen(")")
	} else if node.kindof(conditional_operator) {
		// ? :
		c.gen("if ") // { } else { }")
		expr := node.try_get_next_child()
		case1 := node.try_get_next_child()
		case2 := node.try_get_next_child()
		c.expr(expr)
		c.gen("{ ")
		c.expr(case1)
		c.gen(" } else {")
		c.expr(case2)
		c.gen("}")
	} else if node.kindof(break_stmt) {
		if c.inside_switch == 0 {
			c.genln("break")
		}
	} else if node.kindof(continue_stmt) {
		c.genln("continue")
	} else if node.kindof(goto_stmt) {
		c.goto_stmt(node)
	} else if node.kindof(opaque_value_expr) {
		// TODO
	} else if node.kindof(paren_list_expr) {
	} else if node.kindof(va_arg_expr) {
	} else if node.kindof(compound_stmt) {
	} else if node.kindof(offset_of_expr) {
	} else if node.kindof(array_filler) {
		c.gen("/*AFFF*/")
	} else if node.kindof(goto_stmt) {
	} else if node.kindof(implicit_value_init_expr) {
	} else if c.cpp_expr(node) {
	} else if node.kindof(deprecated_attr) {
		c.gen("/*deprecated*/")
	} else if node.kindof(full_comment) {
		c.gen("/*full comment*/")
	} else if node.kindof(bad) {
		vprintln("BAD node in expr()")
		vprintln(node.str())
	} else {
		if node.is_builtin() {
			// TODO this check shouldn't be needed, all builtin nodes should be skipped
			// when handling top level nodes
			return node.value
		}
		eprintln(`\n\nUnhandled expr() node {${node.kind}} (ast line nr node.ast_line_nr "${c.cur_file}"):`)

		eprintln(node.str())

		//print_backtrace()
		panic(1)
	}
	return node.value // get_val(0)
}

func (c *C2V) name_expr(node *Node) {
	// `GREEN` => `Color.GREEN`
	// Find the enum that has this value
	// vals:
	// ["int", "EnumConstant", "MT_SPAWNFIRE", "int"]
	is_enum_val := node.ref_declaration.kind == enum_constant_decl

	if is_enum_val {
		enum_val := to_lower(node.ref_declaration.name)
		need_full_enum := true // need `Color.green` instead of just `.green`

		if c.inside_switch != 0 && c.inside_switch_enum {
			// generate just `match ... { .val { } }`, not `match ... { Enum.val { } }`
			need_full_enum = false
		}
		if c.inside_array_index {
			need_full_enum = true
		}
		enum_name := c.enum_val_to_enum_name(enum_val)
		if c.inside_array_index {
			// `foo[ENUM_VAL]` => `foo(int(ENUM_NAME.ENUM_VAL))`
			c.gen("int(")
		}
		if need_full_enum {
			c.gen(enum_name)
		}
		if (enum_val != "true") && (enum_val != "false") && enum_name != "" {
			// Don't add a `.` before "const" enum vals so that e.g. `tmbbox[BOXLEFT]`
			// won't get translated to `tmbbox[.boxleft]`
			// (empty enum name means its enum vals are consts)

			c.gen(".")
		}
	}

	name := node.ref_declaration.name

	if !ArrayContains(name, c.consts) && !c.global_contains(name) {
		// Functions and variables are all lowercase in V
		name = to_lower(name)
		if starts_with(name, "c.") {
			name = "C." + name[2:] // TODO why is this needed?
		}
	}

	c.gen(filter_name(name))
	if is_enum_val && c.inside_array_index {
		c.gen(")")
	}
}

func ArrayContains(s string, arr []string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}

	return false
}

func (c *C2V) init_list_expr(node *Node) {
	t := node.ast_type.qualified
	// c.gen(" /* list init $t */ ")
	// C list init can be an array (`numbers = {1,2,3}` => `numbers = [1,2,3]``)
	// or a struct init (`user = {"Bob", 20}` => `user = {"Bob", 20}`)
	is_arr := contains(t, "[")
	if !is_arr {
		c.genln(parse_c_struct_name(t) + " {")
	} else {
		c.gen("[")
	}
	if len(node.array_filler) > 0 {
		for i, child := range node.array_filler {
			// array_filler nodes were not handled by set_kind_enum
			child.initialize_node_and_children()

			if child.kindof(implicit_value_init_expr) {
			} else {
				c.expr(child)
				if i < len(node.array_filler)-1 {
					c.gen(", ")
				}
			}
		}
	} else {
		for i, child := range node.inner {
			if child.kind == bad {
				child.kind = convert_str_into_node_kind(child.kind_str) // array_filler nodes were not handled by set_kind_enum
			}

			// C allows not to set final fields (a = {1,2,,,,})
			// V requires all fields to be set
			if child.kindof(implicit_value_init_expr) {
				c.gen("0/*IMPLICIT*/")
			} else {
				c.expr(child)
				if i < len(node.inner)-1 {
					c.gen(", ")
				}
			}
		}
	}
	is_fixed := contains(node.ast_type.qualified, "[") && contains(node.ast_type.qualified, "]")
	if !is_arr {
		c.genln("}")
	} else {
		if is_fixed {
			c.genln("]!")
		} else {
			c.genln("]")
		}
	}
}

func filter_name(name string) string {
	if is_go_keyword(name) {
		return name + "_" // ??
	}
	if in_builtin_fn_names(name) {
		// ??
		return "C." + name
	}
	if name == "argv" {
		return "os.argv"
	}
	if name == "FILE" {
		return "C.FILE"
	}
	return name
}

func (c2v *C2V) translate_file(path string) {
	start_ticks := time.Now()
	print("  translating ${path:-15s} ... ")
	//flush_stdout()
	c2v.set_config_overrides_for_file(path)
	lines := []string{}
	ast_path := path
	ext := filepath.Ext(path)
	if contains(path, "/src/") {
		// Hack to fix "doomtype.h" file not found
		// TODO come up with a better solution
		work_path := before(path, "/src/") + "/src"
		vprintln(work_path)
		os.Chdir(work_path)
	}
	additional_clang_flags := c2v.get_additional_flags(path)
	cmd := fmt.Sprintf("clang %s -w -Xclang -ast-dump=json "+
		"-fsyntax-only -fno-diagnostics-color -c %s", additional_clang_flags, quoted_path(path))
	vprintln("DA CMD")
	vprintln(cmd)
	out_ast := ""

	// file.c => file.json
	out_ast = replace(path, ext, ".json")

	//out_ast_dir := os.dir(out_ast)

	vprintf("EXT=%v out_ast=%v\n", ext, out_ast)
	vprintf("out_ast=%v\n", out_ast)
	clang_cmd := exec.Command(fmt.Sprintf("%s > %s", cmd, out_ast))
	clang_result := clang_cmd.Run()
	if clang_result != nil {
		eprintln("\nThe file ${path} could not be parsed as a C source file.")
		// TODO exit or return?
		return
		//panic(1)
	}
	lines, _ = ReadLines(out_ast)
	ast_path = out_ast
	vprintf("lines.len=%d\n", len(lines))
	out_v := replace(out_ast, ".json", ".v")
	rootdir, _ := os.Getwd()
	short_output_path := replace(out_v, rootdir+"/", "")
	c_file := path
	c2v.add_file(ast_path, out_v, c_file)

	// preparation pass, fill in the Node redeclarations field:
	seen_ids := map[string]*Node{}
	for i, node := range c2v.tree.inner {
		c2v.node_i = i
		seen_ids[node.id] = node
		if node.previous_declaration != "" {
			pnode := seen_ids[node.previous_declaration]
			if pnode != nil {
				pnode.redeclarations_count++
			}
		}
	}
	// Main parse loop
	for i, node := range c2v.tree.inner {
		vprintf(`\ndoing top node %d %v name="%s" is_std=%v\n`, i,
			node.kind, node.name, node.is_builtin_type)
		c2v.node_i = i
		c2v.top_level(node)
	}
	/*
		if os.args.contains("-print_tree") {
			c2v.print_entire_tree()
		}
		if !os.args.contains("-keep_ast") {
			os.rm(out_ast)
		}
	*/
	vprintln("DONE!2")
	c2v.save()
	c2v.translations++
	delta_ticks := time.Now().Sub(start_ticks)
	fmt.Printf("took %d ms ; output .v file: %s\n", delta_ticks.Microseconds(), short_output_path)
	//println(" took ${delta_ticks:5} ms ; output .v file: ${short_output_path}")
}

func (c2v *C2V) print_entire_tree() {
	for _, node := range c2v.tree.inner {
		print_node_recursive(node, 0)
	}
}

func print_node_recursive(node *Node, ident int) {
	vprint(repeat("  ", ident))
	vprintf(`"%v n="%s"`, node.kind, node.name)
	for _, child := range node.inner {
		print_node_recursive(child, ident+1)
	}
	if len(node.array_filler) > 0 {
		for _, child := range node.array_filler {
			print_node_recursive(child, ident+1)
		}
	}
}

func (c *C2V) top_level(node *Node) {
	if node.is_builtin_type {
		vprintf(`is std, ret (name="%s")\n`, node.name)
		return
	}
	if node.kindof(typedef_decl) {
		c.typedef_decl(node)
	} else if node.kindof(function_decl) {
		c.func_decl(node, "")
	} else if node.kindof(record_decl) {
		c.record_decl(node)
	} else if node.kindof(var_decl) {
		c.global_var_decl(node)
	} else if node.kindof(enum_decl) {
		c.enum_decl(node)
	} else if !c.cpp_top_level(node) {
		vprintf("\n\nUnhandled non C++ top level node typ=%v:\n", node.ast_type)
		panic(1)
	}
}

func (node *Node) get_int_define() string {
	return "HEADER"
}

// ""struct Foo":"struct Foo""  => "Foo"
func parse_c_struct_name(typ string) string {
	res := all_before(typ, ":")
	res = replace(res, "struct ", "")
	res = capitalize(res) // lowercase structs are stored as is, but need to be capitalized during gen
	return res
}

func trim_underscores(s string) string {
	i := 0
	for i < len(s) {
		if s[i:i+1] != `_` {
			break
		}
		i++
	}
	return s[i:]
}

func capitalize_type(s string) string {
	name := s
	if starts_with(name, "_") {
		// Trim "_" from the start of the struct name
		// TODO this can result in conflicts
		name = trim_underscores(name)
	}
	if !starts_with(name, "func ") {
		name = capitalize(name)
	}
	return name
}

func (c *C2V) verror(msg string) {
	panic(1)
}

func (c *C2V) contains_word(word string) bool {
	return contains(c.c_file_contents, word)
}

func (c2v *C2V) save_globals() {
	globals_path := c2v.get_globals_path()
	lines := []string{}

	lines = append(lines, "[translated]\n")
	if c2v.has_cfile {
		lines = append(lines, "[typedef]\nstruct C.FILE {}")
	}
	for _, g := range c2v.globals_out {
		lines = append(lines, g)
	}

	WriteLines(lines, globals_path)
}

func types_are_equal(a string, b string) bool {
	if a == b {
		return true
	}
	if starts_with(a, "[") && starts_with(b, "[") {
		return after(a, "]") == after(b, "]")
	}
	return false
}

func (c *C2V) global_contains(s string) bool {
	for _, v := range c.globals {
		if v.name == s {
			return true
		}
	}

	return false
}
