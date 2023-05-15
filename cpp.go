package main

import (
	"fmt"
	"strings"
)

func (c *C2V) for_range(node *Node) {
	//  node := unsafe { _node }
	// decl := node.get(DeclStmt)
	stmt := node.inner.last()
	// decls := node.find_children(DeclStmt)
	// decl:=decls.last()
	// var_name :=  j
	c.genln(`for val in vals {`)
	c.st_block_no_start(stmt)
}

func (c *C2V) cpp_expr(_node *Node) bool {
	node := _node
	vprintln(`C++ expr check`)
	// println(node.vals)
	vprintln(node.ast_type.str())
	// std::vector<int> a;    OR
	// User u(34);
	if node.kindof(cxx_construct_expr) {
		// println(node.vals)
		// c.genln(node.vals.str())
		c.genln(`// cxx cons`)
		typ := node.ast_type.qualified // get_val(-2)
		if typ.contains(`<int>`) {
			c.gen(`int`)
		}
	} else if node.kindof(cxx_member_call_expr) {
		// c.gen(`[CXX MEMBER] `)
		member_expr := node.try_get_next_child_of_kind(member_expr)

		method_name := member_expr.name // get_val(-2)
		child := member_expr.try_get_next_child()
		c.expr(child)
		add_par := false
		switch method_name {
		case `.push_back`:
			c.gen(` << `)
		case `.size`:
			c.gen(`.len`)
		default:
			{
				add_par = true
				method := replace_str(method_name, `->`, `.`)
				c.gen(`${method}(`)
			}
		}
		mat_tmp_expr := node.try_get_next_child()
		if mat_tmp_expr.kindof(materialize_temporary_expr) {
			expr := mat_tmp_expr.try_get_next_child()
			c.expr(expr)
		}
		if add_par {
			c.gen(`)`)
		}
	} else if node.kindof(cxx_operator_call_expr) {
		// operator call (std::cout << etc)
		c.operator_call(node)
	} else if node.kindof(expr_with_cleanups) {
		// std::string s = "HI";
		vprintln(`expr with cle`)
		typ := node.ast_type.qualified // get_val(-1)
		vprintln(`TYP=${typ}`)
		if typ.contains(`basic_string<`) {
			// All this for a simple std::string = "hello";
			construct_expr := node.try_get_next_child_of_kind(cxx_construct_expr)

			mat_tmp_expr := construct_expr.try_get_next_child_of_kind(materialize_temporary_expr)

			// cast_expr := mat_tmp_expr.get(ImplicitCastExpr)
			cast_expr := mat_tmp_expr.try_get_next_child()
			if !cast_expr.kindof(implicit_cast_expr) {
				return true
			}
			bind_tmp_expr := cast_expr.try_get_next_child_of_kind(cxx_bind_temporary_expr)

			cast_expr2 := bind_tmp_expr.try_get_next_child_of_kind(implicit_cast_expr)

			construct_expr2 := cast_expr2.try_get_next_child_of_kind(cxx_construct_expr)

			cast_expr3 := construct_expr2.try_get_next_child_of_kind(implicit_cast_expr)

			str_lit := cast_expr3.try_get_next_child_of_kind(string_literal)

			c.gen(str_lit.value) // get_val(-1))
		}
	} else if node.kindof(unresolved_lookup_expr) {
	} else if node.kindof(cxx_try_stmt) {
	} else if node.kindof(cxx_throw_expr) {
	} else if node.kindof(cxx_dynamic_cast_expr) {
		typ_ := convert_type(node.ast_type.qualified) // get_val(2))
		dtyp := typ_.name
		dtyp = dtyp.replace(`* `, `&`)
		c.gen(`${dtyp}( `)
		child := node.try_get_next_child()
		c.expr(child)
		c.gen(`)`)
	} else if node.kindof(cxx_reinterpret_cast_expr) {
	} else if node.kindof(cxx_unresolved_construct_expr) {
	} else if node.kindof(cxx_dependent_scope_member_expr) {
	} else if node.kindof(cxx_this_expr) {
		c.gen(`this`)
	} else if node.kindof(cxx_bool_literal_expr) {
		val := node.value // get_val(-1)
		c.gen(val)
	} else if node.kindof(cxx_null_ptr_literal_expr) {
		c.gen(`nullptr`)
	} else if node.kindof(cxx_functional_cast_expr) {
	} else if node.kindof(cxx_delete_expr) {
	} else if node.kindof(cxx_static_cast_expr) {
		// static_cast<int>(a)
		typ := node.ast_type.qualified // get_val(0)
		// v := node.vals.join(` `)
		c.gen(`(${typ})(`)
		expr := node.try_get_next_child()
		c.expr(expr)
		c.gen(`)`)
	} else if node.kindof(materialize_temporary_expr) {
	} else if node.kindof(cxx_temporary_object_expr) {
	} else if node.kindof(decl_stmt) {
		// TODO WTF
	} else if node.kindof(cxx_new_expr) {
	} else {
		return false
	}
	return true
}
