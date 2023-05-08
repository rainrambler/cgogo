package main

import (
	"fmt"
)

func AppendComma(filename string) {
	lines, err := ReadLines(filename)
	if err != nil {
		fmt.Printf("WARN: Cannot read file: %s!\n", filename)
		return
	}

	arr := []string{}
	for _, line := range lines {
		arr = append(arr, line+",")
	}

	WriteLines(arr, filename+".txt")
}
