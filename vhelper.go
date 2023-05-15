package main

import (
	"strings"
)

// https://modules.vlang.io/
func after(s, part string) string {
	pos := strings.LastIndex(s, part)
	if pos == -1 {
		return s // same as v lang
	}

	return s[pos+1:]
}

// https://modules.vlang.io/
func all_before(s, part string) string {
	pos := strings.LastIndex(s, part)
	if pos == -1 {
		return s // same as v lang
	}

	return s[:pos]
}

func all_after(s, part string) string {
	pos := strings.LastIndex(s, part)
	if pos == -1 {
		return s // same as v lang
	}

	return s[pos+len(part):]
}

func ends_with(tofind, keyword string) bool {
	return strings.HasSuffix(tofind, keyword)
}

func replace_str(s, findstr, repstr string) string {
	return strings.ReplaceAll(s, findstr, repstr)
}

func replace(s, findstr, repstr string) string {
	return strings.ReplaceAll(s, findstr, repstr)
}

func repeat(s string, number int) string {
	if number <= 0 {
		return ""
	}

	if number == 1 {
		return s
	}

	ret := ""
	for i := 0; i < number; i++ {
		ret += s
	}
	return ret
}

func find_between(s, start, end0 string) string {
	pos_start := strings.Index(s, start)
	pos_end := strings.Index(s, end0)

	if pos_start == -1 {
		return ""
	}

	if pos_end == -1 {
		return s
	}

	return s[pos_start+len(start) : pos_end]
}

func split(s, delimeter string) []string {
	return strings.Split(s, delimeter)
}

func contains_any_substr(tofind string, keywords []string) bool {
	for _, v := range keywords {
		if strings.Contains(tofind, v) {
			return true
		}
	}
	return false
}

func contains_substr(tofind string, keywords string) bool {
	return strings.Contains(tofind, keywords)
}

func contains(tofind string, keywords string) bool {
	return strings.Contains(tofind, keywords)
}

func starts_with(s, substr string) bool {
	return strings.HasPrefix(s, substr)
}

func sub_str(s string, startPos, endPos int) string {
	return s[startPos:endPos]
}

func quoted_path(path string) string {
	return `"` + path + `"`
}
