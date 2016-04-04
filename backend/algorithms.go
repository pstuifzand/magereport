package backend

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
)

var pathRegexp *regexp.Regexp

func init() {
	pathRegexp = regexp.MustCompile("^(.*)-(default|websites|stores)-(\\d+)$")
}

func makeDiffLine(path, oldval, newval string) DiffLine {
	isAdded := newval != "" && oldval == ""
	isRemoved := newval == "" && oldval != ""
	isChanged := newval != "" && oldval != "" && oldval != newval
	parts := pathRegexp.FindStringSubmatch(path)
	scope := parts[2]
	scopeId, _ := strconv.ParseInt(parts[3], 10, 64)
	return DiffLine{parts[1], oldval, newval, isAdded, isRemoved, isChanged, scope, scopeId}
}

func Diff(oldSnapshot, newSnapshot SnapshotVars, from, to int) (DiffResult, error) {
	missing := make(map[string]bool)
	for k, _ := range oldSnapshot.Vars {
		missing[k] = true
	}

	paths := []string{}
	for k, _ := range newSnapshot.Vars {
		paths = append(paths, k)
	}

	sort.Strings(paths)

	result := DiffResult{}
	result.Lines = []DiffLine{}
	result.From = from + 1
	result.To = to + 1

	count := DiffResultCount{0, 0, 0}

	for _, path := range paths {
		val := newSnapshot.Vars[path]
		missing[path] = false
		if oldVal, e := oldSnapshot.Vars[path]; e {
			if oldVal != val {
				result.Lines = append(result.Lines, makeDiffLine(path, oldVal, val))
				count.Changed += 1
			}
		} else {
			result.Lines = append(result.Lines, makeDiffLine(path, "", val))
			count.Added += 1
		}
	}

	for k, v := range missing {
		if v {
			result.Lines = append(result.Lines, makeDiffLine(k, oldSnapshot.Vars[k], ""))
			count.Removed += 1
		}
	}

	result.Count = count

	return result, nil
}

func Export(w io.Writer, oldSnapshot, newSnapshot SnapshotVars, from, to int) error {
	diff, err := Diff(oldSnapshot, newSnapshot, from, to)
	if err != nil {
		return err
	}

	var newlinesRegexp *regexp.Regexp
	newlinesRegexp = regexp.MustCompile("\r?\n")

	for _, r := range diff.Lines {
		value := newlinesRegexp.ReplaceAllString(r.NewValue, "\\n")
		fmt.Fprintf(w, `config:set --scope="%s" --scope-id="%d" "%s" "%s"`,
			r.Scope, r.ScopeId, r.Path, value)
		fmt.Fprintln(w, "")
	}
	return nil
}
