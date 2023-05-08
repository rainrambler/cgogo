package main

import (
	"os"
)

var builtin_type_names = []string{"ldiv_t", "__float2", "__double2", "exception", "double_t"}

var builtin_global_names = []string{"sys_nerr", "sys_errlist", "suboptarg"}

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
	tree            Node
	is_dir          bool // when translating a directory (multiple C=>V files)
	c_file_contents string
	line_i          int
	node_i          int      // when parsing nodes
	unhandled_nodes []string // when coming across an unknown Clang AST node
	// out  stuff
	//out                 strings.Builder   // os.File
	globals_out         map[string]string // `globals_out["myglobal"] == "extern int myglobal = 0;"` // strings.Builder
	out_file            os.File
	out_line_empty      bool
	types               []string            // to avoid dups
	enums               []string            // to avoid dups
	enum_vals           map[string]*str_arr // enum_vals['Color'] = ['green', 'blue'], for converting C globals  to enum values
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
	labels              map[string]string // for goto stmts: `label_stmts[label_id] == 'labelname'`
	//
	project_folder string // the final folder passed on the CLI, or the folder of the last file, passed on the CLI. Will be used for searching for a c2v.toml file, containing project configuration overrides, when the C2V_CONFIG env variable is not set explicitly.
	//conf           toml.Doc = empty_toml_doc() // conf will be set by parsing the TOML configuration file
	//
	project_output_dirname   string // by default, 'c2v_out.dir'; override with `[project] output_dirname = "another"`
	project_additional_flags string // what to pass to clang, so that it could parse all the input files; mainly -I directives to find additional headers; override with `[project] additional_flags = "-I/some/folder"`
	project_uses_sdl         bool   // if a project uses sdl, then the additional flags will include the result of `sdl2-config --cflags` too; override with `[project] uses_sdl = true`
	file_additional_flags    string // can be added per file, appended to project_additional_flags ; override with `['info.c'] additional_flags = -I/xyz`
	//
	project_globals_path string // where to store the _globals.v file, that will contain all the globals/consts for the project folder; calculated using project_output_dirname and project_folder
	//
	translations            int    // how many translations were done so far
	translation_start_ticks uint64 // initialised before the loop calling .translate_file()
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
