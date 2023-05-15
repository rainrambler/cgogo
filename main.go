// cgogo project main.go
package main

import (
	"fmt"
)

func main() {
	//fmt.Println("Hello World!")
	//AppendComma(`D:\tmp\AST\c2v-master\src\aaa.txt`)
	if os.args.len < 2 {
		eprintln("Usage:")
		eprintln("  c2v file.c")
		eprintln("  c2v wrapper file.h")
		eprintln("  c2v folder/")
		exit(1)
	}
	vprintln(os.args.str())
	is_wrapper := os.args[1] == "wrapper"
	path := os.args.last()
	if !os.exists(path) {
		eprintln(`"${path}" does not exist`)
		exit(1)
	}
	c2v := new_c2v(os.args)
	println("C to V translator ${version}")
	c2v.translation_start_ticks = time.ticks()
	if os.is_dir(path) {
		os.chdir(path)
		println(`"${path}" is a directory, processing all C files in it recursively...\n`)
		files := os.walk_ext(".", ".c")

		if is_wrapper {
		} else {
			if files.len > 0 {
				for _, file := range files {
					c2v.translate_file(file)
				}
				c2v.save_globals()
			}
		}
	} else {
		c2v.translate_file(path)
	}
	delta_ticks := time.ticks() - c2v.translation_start_ticks
	println("Translated ${c2v.translations:3} files in ${delta_ticks:5} ms.")
}
