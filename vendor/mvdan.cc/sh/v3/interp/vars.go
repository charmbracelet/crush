// Copyright (c) 2017, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interp

import (
	"fmt"
	"maps"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

type overlayEnviron struct {
	parent expand.Environ
	values map[string]expand.Variable

	// We need to know if the current scope is a function's scope, because
	// functions can modify global variables.
	funcScope bool
}

func (o *overlayEnviron) Get(name string) expand.Variable {
	if vr, ok := o.values[name]; ok {
		return vr
	}
	return o.parent.Get(name)
}

func (o *overlayEnviron) Set(name string, vr expand.Variable) error {
	// Manipulation of a global var inside a function.
	if o.funcScope && !vr.Local && !o.values[name].Local {
		// In a function, the parent environment is ours, so it's always read-write.
		return o.parent.(expand.WriteEnviron).Set(name, vr)
	}

	if o.values == nil {
		o.values = make(map[string]expand.Variable)
	}
	prev := o.Get(name)
	if vr.Kind == expand.KeepValue {
		vr.Kind = prev.Kind
		vr.Str = prev.Str
		vr.List = prev.List
		vr.Map = prev.Map
	} else if prev.ReadOnly {
		return fmt.Errorf("readonly variable")
	}
	if !vr.IsSet() { // unsetting
		if prev.Local {
			vr.Local = true
			o.values[name] = vr
			return nil
		}
		delete(o.values, name)
	}
	// modifying the entire variable
	vr.Local = prev.Local || vr.Local
	o.values[name] = vr
	return nil
}

func (o *overlayEnviron) Each(f func(name string, vr expand.Variable) bool) {
	o.parent.Each(f)
	for name, vr := range o.values {
		if !f(name, vr) {
			return
		}
	}
}

func execEnv(env expand.Environ) []string {
	list := make([]string, 0, 64)
	for name, vr := range env.Each {
		if !vr.IsSet() {
			// If a variable is set globally but unset in the
			// runner, we need to ensure it's not part of the final
			// list. Seems like zeroing the element is enough.
			// This is a linear search, but this scenario should be
			// rare, and the number of variables shouldn't be large.
			for i, kv := range list {
				if strings.HasPrefix(kv, name+"=") {
					list[i] = ""
				}
			}
		}
		if vr.Exported && vr.Kind == expand.String {
			list = append(list, name+"="+vr.String())
		}
	}
	return list
}

func (r *Runner) lookupVar(name string) expand.Variable {
	if name == "" {
		panic("variable name must not be empty")
	}
	var vr expand.Variable
	switch name {
	case "#":
		vr.Kind, vr.Str = expand.String, strconv.Itoa(len(r.Params))
	case "@", "*":
		vr.Kind = expand.Indexed
		if r.Params == nil {
			// r.Params may be nil but positional parameters always exist
			vr.List = []string{}
		} else {
			vr.List = r.Params
		}
	case "?":
		vr.Kind, vr.Str = expand.String, strconv.Itoa(r.lastExit)
	case "$":
		vr.Kind, vr.Str = expand.String, strconv.Itoa(os.Getpid())
	case "PPID":
		vr.Kind, vr.Str = expand.String, strconv.Itoa(os.Getppid())
	case "DIRSTACK":
		vr.Kind, vr.List = expand.Indexed, r.dirStack
	case "0":
		vr.Kind = expand.String
		if r.filename != "" {
			vr.Str = r.filename
		} else {
			vr.Str = "gosh"
		}
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		vr.Kind = expand.String
		i := int(name[0] - '1')
		if i < len(r.Params) {
			vr.Str = r.Params[i]
		} else {
			vr.Str = ""
		}
	}
	if vr.Kind != expand.Unknown {
		vr.Set = true
		return vr
	}
	if vr := r.writeEnv.Get(name); vr.Declared() {
		return vr
	}
	if runtime.GOOS == "windows" {
		upper := strings.ToUpper(name)
		if vr := r.writeEnv.Get(upper); vr.Declared() {
			return vr
		}
	}
	return expand.Variable{}
}

func (r *Runner) envGet(name string) string {
	return r.lookupVar(name).String()
}

func (r *Runner) delVar(name string) {
	if err := r.writeEnv.Set(name, expand.Variable{}); err != nil {
		r.errf("%s: %v\n", name, err)
		r.exit = 1
		return
	}
}

func (r *Runner) setVarString(name, value string) {
	r.setVar(name, expand.Variable{Set: true, Kind: expand.String, Str: value})
}

func (r *Runner) setVar(name string, vr expand.Variable) {
	if r.opts[optAllExport] {
		vr.Exported = true
	}
	if err := r.writeEnv.Set(name, vr); err != nil {
		r.errf("%s: %v\n", name, err)
		r.exit = 1
		return
	}
}

func (r *Runner) setVarWithIndex(prev expand.Variable, name string, index syntax.ArithmExpr, vr expand.Variable) {
	prev.Set = true
	if name2, var2 := prev.Resolve(r.writeEnv); name2 != "" {
		name = name2
		prev = var2
	}

	if vr.Kind == expand.String && index == nil {
		// When assigning a string to an array, fall back to the
		// zero value for the index.
		switch prev.Kind {
		case expand.Indexed:
			index = &syntax.Word{Parts: []syntax.WordPart{
				&syntax.Lit{Value: "0"},
			}}
		case expand.Associative:
			index = &syntax.Word{Parts: []syntax.WordPart{
				&syntax.DblQuoted{},
			}}
		}
	}
	if index == nil {
		r.setVar(name, vr)
		return
	}

	// from the syntax package, we know that value must be a string if index
	// is non-nil; nested arrays are forbidden.
	valStr := vr.Str

	var list []string
	switch prev.Kind {
	case expand.String:
		list = append(list, prev.Str)
	case expand.Indexed:
		// TODO: only clone when inside a subshell and getting a var from outside for the first time
		list = slices.Clone(prev.List)
	case expand.Associative:
		// if the existing variable is already an AssocArray, try our
		// best to convert the key to a string
		w, ok := index.(*syntax.Word)
		if !ok {
			return
		}
		k := r.literal(w)

		// TODO: only clone when inside a subshell and getting a var from outside for the first time
		prev.Map = maps.Clone(prev.Map)
		if prev.Map == nil {
			prev.Map = make(map[string]string)
		}
		prev.Map[k] = valStr
		r.setVar(name, prev)
		return
	}
	k := r.arithm(index)
	for len(list) < k+1 {
		list = append(list, "")
	}
	list[k] = valStr
	prev.Kind = expand.Indexed
	prev.List = list
	r.setVar(name, prev)
}

func (r *Runner) setFunc(name string, body *syntax.Stmt) {
	if r.Funcs == nil {
		r.Funcs = make(map[string]*syntax.Stmt, 4)
	}
	r.Funcs[name] = body
}

func stringIndex(index syntax.ArithmExpr) bool {
	w, ok := index.(*syntax.Word)
	if !ok || len(w.Parts) != 1 {
		return false
	}
	switch w.Parts[0].(type) {
	case *syntax.DblQuoted, *syntax.SglQuoted:
		return true
	}
	return false
}

// TODO: make assignVal and [setVar] consistent with the [expand.WriteEnviron] interface

func (r *Runner) assignVal(prev expand.Variable, as *syntax.Assign, valType string) expand.Variable {
	prev.Set = true
	if as.Value != nil {
		s := r.literal(as.Value)
		if !as.Append {
			prev.Kind = expand.String
			if valType == "-n" {
				prev.Kind = expand.NameRef
			}
			prev.Str = s
			return prev
		}
		switch prev.Kind {
		case expand.String, expand.Unknown:
			prev.Kind = expand.String
			prev.Str += s
		case expand.Indexed:
			if len(prev.List) == 0 {
				prev.List = append(prev.List, "")
			}
			prev.List[0] += s
		case expand.Associative:
			// TODO
		}
		return prev
	}
	if as.Array == nil {
		// don't return the zero value, as that's an unset variable
		prev.Kind = expand.String
		if valType == "-n" {
			prev.Kind = expand.NameRef
		}
		prev.Str = ""
		return prev
	}
	// Array assignment.
	elems := as.Array.Elems
	if valType == "" {
		valType = "-a" // indexed
		if len(elems) > 0 && stringIndex(elems[0].Index) {
			valType = "-A" // associative
		}
	}
	if valType == "-A" {
		amap := make(map[string]string, len(elems))
		for _, elem := range elems {
			k := r.literal(elem.Index.(*syntax.Word))
			amap[k] = r.literal(elem.Value)
		}
		if !as.Append {
			prev.Kind = expand.Associative
			prev.Map = amap
			return prev
		}
		// TODO
		return prev
	}
	// Evaluate values for each array element.
	elemValues := make([]struct {
		index  int
		values []string
	}, len(elems))
	var index, maxIndex int
	for i, elem := range elems {
		if elem.Index != nil {
			// Index resets our index with a literal value.
			index = r.arithm(elem.Index)
			elemValues[i].values = []string{r.literal(elem.Value)}
		} else {
			// Implicit index, advancing for every word.
			elemValues[i].values = r.fields(elem.Value)
		}
		elemValues[i].index = index
		index += len(elemValues[i].values)
		maxIndex = max(maxIndex, index)
	}
	// Flatten down the values.
	strs := make([]string, maxIndex)
	for _, ev := range elemValues {
		for i, str := range ev.values {
			strs[ev.index+i] = str
		}
	}
	if !as.Append {
		prev.Kind = expand.Indexed
		prev.List = strs
		return prev
	}
	switch prev.Kind {
	case expand.Unknown:
		prev.Kind = expand.Indexed
		prev.List = strs
	case expand.String:
		prev.Kind = expand.Indexed
		prev.List = append([]string{prev.Str}, strs...)
	case expand.Indexed:
		prev.List = append(prev.List, strs...)
	case expand.Associative:
		// TODO
	default:
		panic(fmt.Sprintf("unhandled conversion of kind %d", prev.Kind))
	}
	return prev
}
