// cgogo project main.go
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	//fmt.Println("Hello World!")
	//AppendComma(`D:\tmp\AST\c2v-master\src\aaa.txt`)
	if len(os.Args) < 2 {
		eprintln("Usage:")
		eprintln("  c2v file.c")
		eprintln("  c2v wrapper file.h")
		eprintln("  c2v folder/")
		return
	}

	path := os.Args[len(os.Args)-1] // last
	if _, err := os.Stat(path); err == nil {
		//fmt.Printf("File exists\n");
	} else {
		fmt.Printf(`"%s" does not exist\n`, path)
		return
	}

	c2v := new_c2v(os.Args)
	println("C to V translator ${version}")
	c2v.translation_start_ticks = time.Now().UnixMicro()

	fi, _ := os.Stat(path)
	if fi.IsDir() {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}

		for _, f := range files {
			if is_c_file(f.Name()) {
				c2v.translate_file(f.Name())
			}
		}
		c2v.save_globals()
	} else {
		c2v.translate_file(path)
	}
	delta_ticks := time.Now().UnixMicro() - c2v.translation_start_ticks
	fmt.Printf("Translated %v files in %v ms.\n", c2v.translations, delta_ticks)
}

func is_c_file(filename string) bool {
	return filepath.Ext(filename) == ".c"
}
