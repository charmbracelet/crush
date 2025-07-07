// Copyright (c) 2017, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interp

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

func isBuiltin(name string) bool {
	switch name {
	case "true", ":", "false", "exit", "set", "shift", "unset",
		"echo", "printf", "break", "continue", "pwd", "cd",
		"wait", "builtin", "trap", "type", "source", ".", "command",
		"dirs", "pushd", "popd", "umask", "alias", "unalias",
		"fg", "bg", "getopts", "eval", "test", "[", "exec",
		"return", "read", "mapfile", "readarray", "shopt":
		return true
	}
	return false
}

// TODO: oneIf and atoi are duplicated in the expand package.

func oneIf(b bool) int {
	if b {
		return 1
	}
	return 0
}

// atoi is like [strconv.Atoi], but it ignores errors and trims whitespace.
func atoi(s string) int {
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}

func (r *Runner) builtinCode(ctx context.Context, pos syntax.Pos, name string, args []string) int {
	switch name {
	case "true", ":":
	case "false":
		return 1
	case "exit":
		exit := 0
		switch len(args) {
		case 0:
			exit = r.lastExit
		case 1:
			n, err := strconv.Atoi(args[0])
			if err != nil {
				r.errf("invalid exit status code: %q\n", args[0])
				return 2
			}
			exit = n
		default:
			r.errf("exit cannot take multiple arguments\n")
			return 1
		}
		r.exitShell(ctx, exit)
		return exit
	case "set":
		if err := Params(args...)(r); err != nil {
			r.errf("set: %v\n", err)
			return 2
		}
		r.updateExpandOpts()
	case "shift":
		n := 1
		switch len(args) {
		case 0:
		case 1:
			if n2, err := strconv.Atoi(args[0]); err == nil {
				n = n2
				break
			}
			fallthrough
		default:
			r.errf("usage: shift [n]\n")
			return 2
		}
		if n >= len(r.Params) {
			r.Params = nil
		} else {
			r.Params = r.Params[n:]
		}
	case "unset":
		vars := true
		funcs := true
	unsetOpts:
		for i, arg := range args {
			switch arg {
			case "-v":
				funcs = false
			case "-f":
				vars = false
			default:
				args = args[i:]
				break unsetOpts
			}
		}

		for _, arg := range args {
			if vars && r.lookupVar(arg).IsSet() {
				r.delVar(arg)
			} else if _, ok := r.Funcs[arg]; ok && funcs {
				delete(r.Funcs, arg)
			}
		}
	case "echo":
		newline, doExpand := true, false
	echoOpts:
		for len(args) > 0 {
			switch args[0] {
			case "-n":
				newline = false
			case "-e":
				doExpand = true
			case "-E": // default
			default:
				break echoOpts
			}
			args = args[1:]
		}
		for i, arg := range args {
			if i > 0 {
				r.out(" ")
			}
			if doExpand {
				arg, _, _ = expand.Format(r.ecfg, arg, nil)
			}
			r.out(arg)
		}
		if newline {
			r.out("\n")
		}
	case "printf":
		if len(args) == 0 {
			r.errf("usage: printf format [arguments]\n")
			return 2
		}
		format, args := args[0], args[1:]
		for {
			s, n, err := expand.Format(r.ecfg, format, args)
			if err != nil {
				r.errf("%v\n", err)
				return 1
			}
			r.out(s)
			args = args[n:]
			if n == 0 || len(args) == 0 {
				break
			}
		}
	case "break", "continue":
		if !r.inLoop {
			r.errf("%s is only useful in a loop\n", name)
			break
		}
		enclosing := &r.breakEnclosing
		if name == "continue" {
			enclosing = &r.contnEnclosing
		}
		switch len(args) {
		case 0:
			*enclosing = 1
		case 1:
			if n, err := strconv.Atoi(args[0]); err == nil {
				*enclosing = n
				break
			}
			fallthrough
		default:
			r.errf("usage: %s [n]\n", name)
			return 2
		}
	case "pwd":
		evalSymlinks := false
		for len(args) > 0 {
			switch args[0] {
			case "-L":
				evalSymlinks = false
			case "-P":
				evalSymlinks = true
			default:
				r.errf("invalid option: %q\n", args[0])
				return 2
			}
			args = args[1:]
		}
		pwd := r.envGet("PWD")
		if evalSymlinks {
			var err error
			pwd, err = filepath.EvalSymlinks(pwd)
			if err != nil {
				r.setErr(err)
				return 1
			}
		}
		r.outf("%s\n", pwd)
	case "cd":
		var path string
		switch len(args) {
		case 0:
			path = r.envGet("HOME")
		case 1:
			path = args[0]

			// replicate the commonly implemented behavior of `cd -`
			// ref: https://www.man7.org/linux/man-pages/man1/cd.1p.html#OPERANDS
			if path == "-" {
				path = r.envGet("OLDPWD")
				r.outf("%s\n", path)
			}
		default:
			r.errf("usage: cd [dir]\n")
			return 2
		}
		return r.changeDir(ctx, path)
	case "wait":
		if len(args) > 0 {
			panic("wait with args not handled yet")
		}
		// Note that "wait" without arguments always returns exit status zero.
		r.bgShells.Wait()
	case "builtin":
		if len(args) < 1 {
			break
		}
		if !isBuiltin(args[0]) {
			return 1
		}
		return r.builtinCode(ctx, pos, args[0], args[1:])
	case "type":
		anyNotFound := false
		mode := ""
		fp := flagParser{remaining: args}
		for fp.more() {
			switch flag := fp.flag(); flag {
			case "-a", "-f", "-P", "--help":
				r.errf("command: NOT IMPLEMENTED\n")
				return 3
			case "-p", "-t":
				mode = flag
			default:
				r.errf("command: invalid option %q\n", flag)
				return 2
			}
		}
		args := fp.args()
		for _, arg := range args {
			if mode == "-p" {
				if path, err := LookPathDir(r.Dir, r.writeEnv, arg); err == nil {
					r.outf("%s\n", path)
				} else {
					anyNotFound = true
				}
				continue
			}
			if syntax.IsKeyword(arg) {
				if mode == "-t" {
					r.out("keyword\n")
				} else {
					r.outf("%s is a shell keyword\n", arg)
				}
				continue
			}
			if als, ok := r.alias[arg]; ok && r.opts[optExpandAliases] {
				var buf bytes.Buffer
				if len(als.args) > 0 {
					printer := syntax.NewPrinter()
					printer.Print(&buf, &syntax.CallExpr{
						Args: als.args,
					})
				}
				if als.blank {
					buf.WriteByte(' ')
				}
				if mode == "-t" {
					r.out("alias\n")
				} else {
					r.outf("%s is aliased to `%s'\n", arg, &buf)
				}
				continue
			}
			if _, ok := r.Funcs[arg]; ok {
				if mode == "-t" {
					r.out("function\n")
				} else {
					r.outf("%s is a function\n", arg)
				}
				continue
			}
			if isBuiltin(arg) {
				if mode == "-t" {
					r.out("builtin\n")
				} else {
					r.outf("%s is a shell builtin\n", arg)
				}
				continue
			}
			if path, err := LookPathDir(r.Dir, r.writeEnv, arg); err == nil {
				if mode == "-t" {
					r.out("file\n")
				} else {
					r.outf("%s is %s\n", arg, path)
				}
				continue
			}
			if mode != "-t" {
				r.errf("type: %s: not found\n", arg)
			}
			anyNotFound = true
		}
		if anyNotFound {
			return 1
		}
	case "eval":
		src := strings.Join(args, " ")
		p := syntax.NewParser()
		file, err := p.Parse(strings.NewReader(src), "")
		if err != nil {
			r.errf("eval: %v\n", err)
			return 1
		}
		r.stmts(ctx, file.Stmts)
		return r.exit
	case "source", ".":
		if len(args) < 1 {
			r.errf("%v: source: need filename\n", pos)
			return 2
		}
		path, err := scriptFromPathDir(r.Dir, r.writeEnv, args[0])
		if err != nil {
			// If the script was not found in PATH or there was any error, pass
			// the source path to the open handler so it has a chance to look
			// at files it manages (eg: virtual filesystem), and also allow
			// it to look for the sourced script in the current directory.
			path = args[0]
		}
		f, err := r.open(ctx, path, os.O_RDONLY, 0, false)
		if err != nil {
			r.errf("source: %v\n", err)
			return 1
		}
		defer f.Close()
		p := syntax.NewParser()
		file, err := p.Parse(f, path)
		if err != nil {
			r.errf("source: %v\n", err)
			return 1
		}

		// Keep the current versions of some fields we might modify.
		oldParams := r.Params
		oldSourceSetParams := r.sourceSetParams
		oldInSource := r.inSource

		// If we run "source file args...", set said args as parameters.
		// Otherwise, keep the current parameters.
		sourceArgs := len(args[1:]) > 0
		if sourceArgs {
			r.Params = args[1:]
			r.sourceSetParams = false
		}
		// We want to track if the sourced file explicitly sets the
		// parameters.
		r.sourceSetParams = false
		r.inSource = true // know that we're inside a sourced script.
		r.stmts(ctx, file.Stmts)

		// If we modified the parameters and the sourced file didn't
		// explicitly set them, we restore the old ones.
		if sourceArgs && !r.sourceSetParams {
			r.Params = oldParams
		}
		r.sourceSetParams = oldSourceSetParams
		r.inSource = oldInSource

		if code, ok := r.err.(returnStatus); ok {
			r.err = nil
			return int(code)
		}
		return r.exit
	case "[":
		if len(args) == 0 || args[len(args)-1] != "]" {
			r.errf("%v: [: missing matching ]\n", pos)
			return 2
		}
		args = args[:len(args)-1]
		fallthrough
	case "test":
		parseErr := false
		p := testParser{
			rem: args,
			err: func(err error) {
				r.errf("%v: %v\n", pos, err)
				parseErr = true
			},
		}
		p.next()
		expr := p.classicTest("[", false)
		if parseErr {
			return 2
		}
		return oneIf(r.bashTest(ctx, expr, true) == "")
	case "exec":
		// TODO: Consider unix.Exec, i.e. actually replacing
		// the process. It's in theory what a shell should do,
		// but in practice it would kill the entire Go process
		// and it's not available on Windows.
		if len(args) == 0 {
			r.keepRedirs = true
			break
		}
		r.exitShell(ctx, 1)
		r.exec(ctx, args)
		return r.exit
	case "command":
		show := false
		fp := flagParser{remaining: args}
		for fp.more() {
			switch flag := fp.flag(); flag {
			case "-v":
				show = true
			default:
				r.errf("command: invalid option %q\n", flag)
				return 2
			}
		}
		args := fp.args()
		if len(args) == 0 {
			break
		}
		if !show {
			if isBuiltin(args[0]) {
				return r.builtinCode(ctx, pos, args[0], args[1:])
			}
			r.exec(ctx, args)
			return r.exit
		}
		last := 0
		for _, arg := range args {
			last = 0
			if r.Funcs[arg] != nil || isBuiltin(arg) {
				r.outf("%s\n", arg)
			} else if path, err := LookPathDir(r.Dir, r.writeEnv, arg); err == nil {
				r.outf("%s\n", path)
			} else {
				last = 1
			}
		}
		return last
	case "dirs":
		for i, dir := range slices.Backward(r.dirStack) {
			r.outf("%s", dir)
			if i > 0 {
				r.out(" ")
			}
		}
		r.out("\n")
	case "pushd":
		change := true
		if len(args) > 0 && args[0] == "-n" {
			change = false
			args = args[1:]
		}
		swap := func() string {
			oldtop := r.dirStack[len(r.dirStack)-1]
			top := r.dirStack[len(r.dirStack)-2]
			r.dirStack[len(r.dirStack)-1] = top
			r.dirStack[len(r.dirStack)-2] = oldtop
			return top
		}
		switch len(args) {
		case 0:
			if !change {
				break
			}
			if len(r.dirStack) < 2 {
				r.errf("pushd: no other directory\n")
				return 1
			}
			newtop := swap()
			if code := r.changeDir(ctx, newtop); code != 0 {
				return code
			}
			r.builtinCode(ctx, syntax.Pos{}, "dirs", nil)
		case 1:
			if change {
				if code := r.changeDir(ctx, args[0]); code != 0 {
					return code
				}
				r.dirStack = append(r.dirStack, r.Dir)
			} else {
				r.dirStack = append(r.dirStack, args[0])
				swap()
			}
			r.builtinCode(ctx, syntax.Pos{}, "dirs", nil)
		default:
			r.errf("pushd: too many arguments\n")
			return 2
		}
	case "popd":
		change := true
		if len(args) > 0 && args[0] == "-n" {
			change = false
			args = args[1:]
		}
		switch len(args) {
		case 0:
			if len(r.dirStack) < 2 {
				r.errf("popd: directory stack empty\n")
				return 1
			}
			oldtop := r.dirStack[len(r.dirStack)-1]
			r.dirStack = r.dirStack[:len(r.dirStack)-1]
			if change {
				newtop := r.dirStack[len(r.dirStack)-1]
				if code := r.changeDir(ctx, newtop); code != 0 {
					return code
				}
			} else {
				r.dirStack[len(r.dirStack)-1] = oldtop
			}
			r.builtinCode(ctx, syntax.Pos{}, "dirs", nil)
		default:
			r.errf("popd: invalid argument\n")
			return 2
		}
	case "return":
		if !r.inFunc && !r.inSource {
			r.errf("return: can only be done from a func or sourced script\n")
			return 1
		}
		code := 0
		switch len(args) {
		case 0:
		case 1:
			code = atoi(args[0])
		default:
			r.errf("return: too many arguments\n")
			return 2
		}
		r.setErr(returnStatus(code))
	case "read":
		var prompt string
		raw := false
		silent := false
		fp := flagParser{remaining: args}
		for fp.more() {
			switch flag := fp.flag(); flag {
			case "-s":
				silent = true
			case "-r":
				raw = true
			case "-p":
				prompt = fp.value()
				if prompt == "" {
					r.errf("read: -p: option requires an argument\n")
					return 2
				}
			default:
				r.errf("read: invalid option %q\n", flag)
				return 2
			}
		}

		args := fp.args()
		for _, name := range args {
			if !syntax.ValidName(name) {
				r.errf("read: invalid identifier %q\n", name)
				return 2
			}
		}

		if prompt != "" {
			r.out(prompt)
		}

		var line []byte
		var err error
		if silent {
			line, err = term.ReadPassword(int(syscall.Stdin))
		} else {
			line, err = r.readLine(ctx, raw)
		}
		if len(args) == 0 {
			args = append(args, shellReplyVar)
		}

		values := expand.ReadFields(r.ecfg, string(line), len(args), raw)
		for i, name := range args {
			val := ""
			if i < len(values) {
				val = values[i]
			}
			r.setVarString(name, val)
		}

		// We can get data back from readLine and an error at the same time, so
		// check err after we process the data.
		if err != nil {
			return 1
		}

		return 0

	case "getopts":
		if len(args) < 2 {
			r.errf("getopts: usage: getopts optstring name [arg ...]\n")
			return 2
		}
		optind, _ := strconv.Atoi(r.envGet("OPTIND"))
		if optind-1 != r.optState.argidx {
			if optind < 1 {
				optind = 1
			}
			r.optState = getopts{argidx: optind - 1}
		}
		optstr := args[0]
		name := args[1]
		if !syntax.ValidName(name) {
			r.errf("getopts: invalid identifier: %q\n", name)
			return 2
		}
		args = args[2:]
		if len(args) == 0 {
			args = r.Params
		}
		diagnostics := !strings.HasPrefix(optstr, ":")

		opt, optarg, done := r.optState.next(optstr, args)

		r.setVarString(name, string(opt))
		r.delVar("OPTARG")
		switch {
		case opt == '?' && diagnostics && !done:
			r.errf("getopts: illegal option -- %q\n", optarg)
		case opt == ':' && diagnostics:
			r.errf("getopts: option requires an argument -- %q\n", optarg)
		default:
			if optarg != "" {
				r.setVarString("OPTARG", optarg)
			}
		}
		if optind-1 != r.optState.argidx {
			r.setVarString("OPTIND", strconv.FormatInt(int64(r.optState.argidx+1), 10))
		}

		return oneIf(done)

	case "shopt":
		mode := ""
		posixOpts := false
		fp := flagParser{remaining: args}
		for fp.more() {
			switch flag := fp.flag(); flag {
			case "-s", "-u":
				mode = flag
			case "-o":
				posixOpts = true
			case "-p", "-q":
				panic(fmt.Sprintf("unhandled shopt flag: %s", flag))
			default:
				r.errf("shopt: invalid option %q\n", flag)
				return 2
			}
		}
		args := fp.args()
		bash := !posixOpts
		if len(args) == 0 {
			if bash {
				for i, opt := range bashOptsTable {
					r.printOptLine(opt.name, r.opts[len(shellOptsTable)+i], opt.supported)
				}
				break
			}
			for i, opt := range &shellOptsTable {
				r.printOptLine(opt.name, r.opts[i], true)
			}
			break
		}
		for _, arg := range args {
			i, opt := r.optByName(arg, bash)
			if opt == nil {
				r.errf("shopt: invalid option name %q\n", arg)
				return 1
			}

			var (
				bo        *bashOpt
				supported = true // default for shell options
			)
			if bash {
				bo = &bashOptsTable[i-len(shellOptsTable)]
				supported = bo.supported
			}

			switch mode {
			case "-s", "-u":
				if bash && !supported {
					r.errf("shopt: invalid option name %q %q (%q not supported)\n", arg, r.optStatusText(bo.defaultState), r.optStatusText(!bo.defaultState))
					return 1
				}
				*opt = mode == "-s"
			default: // ""
				r.printOptLine(arg, *opt, supported)
			}
		}
		r.updateExpandOpts()

	case "alias":
		show := func(name string, als alias) {
			var buf bytes.Buffer
			if len(als.args) > 0 {
				printer := syntax.NewPrinter()
				printer.Print(&buf, &syntax.CallExpr{
					Args: als.args,
				})
			}
			if als.blank {
				buf.WriteByte(' ')
			}
			r.outf("alias %s='%s'\n", name, &buf)
		}

		if len(args) == 0 {
			for name, als := range r.alias {
				show(name, als)
			}
		}
	argsLoop:
		for _, name := range args {
			i := strings.IndexByte(name, '=')
			if i < 1 { // don't save an empty name
				als, ok := r.alias[name]
				if !ok {
					r.errf("alias: %q not found\n", name)
					continue
				}
				show(name, als)
				continue
			}

			// TODO: parse any CallExpr perhaps, or even any Stmt
			parser := syntax.NewParser()
			var words []*syntax.Word
			src := name[i+1:]
			for w, err := range parser.WordsSeq(strings.NewReader(src)) {
				if err != nil {
					r.errf("alias: could not parse %q: %v\n", src, err)
					continue argsLoop
				}
				words = append(words, w)
			}

			name = name[:i]
			if r.alias == nil {
				r.alias = make(map[string]alias)
			}
			r.alias[name] = alias{
				args:  words,
				blank: strings.TrimRight(src, " \t") != src,
			}
		}
	case "unalias":
		for _, name := range args {
			delete(r.alias, name)
		}

	case "trap":
		fp := flagParser{remaining: args}
		callback := "-"
		for fp.more() {
			switch flag := fp.flag(); flag {
			case "-l", "-p":
				r.errf("trap: %q: NOT IMPLEMENTED flag\n", flag)
				return 2
			case "-":
				// default signal
			default:
				r.errf("trap: %q: invalid option\n", flag)
				r.errf("trap: usage: trap [-lp] [[arg] signal_spec ...]\n")
				return 2
			}
		}
		args := fp.args()
		switch len(args) {
		case 0:
			// Print non-default signals
			if r.callbackExit != "" {
				r.outf("trap -- %q EXIT\n", r.callbackExit)
			}
			if r.callbackErr != "" {
				r.outf("trap -- %q ERR\n", r.callbackErr)
			}
		case 1:
			// assume it's a signal, the default will be restored
		default:
			callback = args[0]
			args = args[1:]
		}
		// For now, treat both empty and - the same since ERR and EXIT have no
		// default callback.
		if callback == "-" {
			callback = ""
		}
		for _, arg := range args {
			switch arg {
			case "ERR":
				r.callbackErr = callback
			case "EXIT":
				r.callbackExit = callback
			default:
				r.errf("trap: %s: invalid signal specification\n", arg)
				return 2
			}
		}

	case "readarray", "mapfile":
		dropDelim := false
		delim := "\n"
		fp := flagParser{remaining: args}
		for fp.more() {
			switch flag := fp.flag(); flag {
			case "-t":
				// Remove the delim from each line read
				dropDelim = true
			case "-d":
				if len(fp.remaining) == 0 {
					r.errf("%s: -d: option requires an argument\n", name)
					return 2
				}
				delim = fp.value()
				if delim == "" {
					// Bash sets the delim to an ASCII NUL if provided with an empty
					// string.
					delim = "\x00"
				}
			default:
				r.errf("%s: invalid option %q\n", name, flag)
				return 2
			}
		}

		args := fp.args()
		var arrayName string
		switch len(args) {
		case 0:
			arrayName = "MAPFILE"
		case 1:
			if !syntax.ValidName(args[0]) {
				r.errf("%s: invalid identifier %q\n", name, args[0])
				return 2
			}
			arrayName = args[0]
		default:
			r.errf("%s: Only one array name may be specified, %v\n", name, args)
			return 2
		}

		var vr expand.Variable
		vr.Kind = expand.Indexed
		scanner := bufio.NewScanner(r.stdin)
		scanner.Split(mapfileSplit(delim[0], dropDelim))
		for scanner.Scan() {
			vr.List = append(vr.List, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			r.errf("%s: unable to read, %v\n", name, err)
			return 2
		}
		r.setVar(arrayName, vr)

		return 0

	default:
		// "umask", "fg", "bg",
		r.errf("%s: unimplemented builtin\n", name)
		return 2
	}
	return 0
}

// mapfileSplit returns a suitable Split function for a [bufio.Scanner];
// the code is mostly stolen from [bufio.ScanLines].
func mapfileSplit(delim byte, dropDelim bool) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, delim); i >= 0 {
			// We have a full newline-terminated line.
			if dropDelim {
				return i + 1, data[0:i], nil
			} else {
				return i + 1, data[0 : i+1], nil
			}
		}
		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			return len(data), data, nil
		}
		// Request more data.
		return 0, nil, nil
	}
}

func (r *Runner) printOptLine(name string, enabled, supported bool) {
	state := r.optStatusText(enabled)
	if supported {
		r.outf("%s\t%s\n", name, state)
		return
	}
	r.outf("%s\t%s\t(%q not supported)\n", name, state, r.optStatusText(!enabled))
}

func (r *Runner) readLine(ctx context.Context, raw bool) ([]byte, error) {
	if r.stdin == nil {
		return nil, errors.New("interp: can't read, there's no stdin")
	}

	var line []byte
	esc := false

	stopc := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		r.stdin.SetReadDeadline(time.Now())
		close(stopc)
	})
	defer func() {
		if !stop() {
			// The AfterFunc was started.
			// Wait for it to complete, and reset the file's deadline.
			<-stopc
			r.stdin.SetReadDeadline(time.Time{})
		}
	}()
	for {
		var buf [1]byte
		n, err := r.stdin.Read(buf[:])
		if n > 0 {
			b := buf[0]
			switch {
			case !raw && b == '\\':
				line = append(line, b)
				esc = !esc
			case !raw && b == '\n' && esc:
				// line continuation
				line = line[len(line)-1:]
				esc = false
			case b == '\n':
				return line, nil
			default:
				line = append(line, b)
				esc = false
			}
		}
		if err != nil {
			return line, err
		}
	}
}

func (r *Runner) changeDir(ctx context.Context, path string) int {
	path = cmp.Or(path, ".")
	path = r.absPath(path)
	info, err := r.stat(ctx, path)
	if err != nil || !info.IsDir() {
		return 1
	}
	if r.access(ctx, path, access_X_OK) != nil {
		return 1
	}
	r.Dir = path
	r.setVarString("OLDPWD", r.envGet("PWD"))
	r.setVarString("PWD", path)
	return 0
}

func absPath(dir, path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return filepath.Clean(path) // TODO: this clean is likely unnecessary
}

func (r *Runner) absPath(path string) string {
	return absPath(r.Dir, path)
}

// flagParser is used to parse builtin flags.
//
// It's similar to the getopts implementation, but with some key differences.
// First, the API is designed for Go loops, making it easier to use directly.
// Second, it doesn't require the awkward ":ab" syntax that getopts uses.
// Third, it supports "-a" flags as well as "+a".
type flagParser struct {
	current   string
	remaining []string
}

func (p *flagParser) more() bool {
	if p.current != "" {
		// We're still parsing part of "-ab".
		return true
	}
	if len(p.remaining) == 0 {
		// Nothing left.
		p.remaining = nil
		return false
	}
	arg := p.remaining[0]
	if arg == "--" {
		// We explicitly stop parsing flags.
		p.remaining = p.remaining[1:]
		return false
	}
	if len(arg) == 0 || (arg[0] != '-' && arg[0] != '+') {
		// The next argument is not a flag.
		return false
	}
	// More flags to come.
	return true
}

func (p *flagParser) flag() string {
	arg := p.current
	if arg == "" {
		arg = p.remaining[0]
		p.remaining = p.remaining[1:]
	} else {
		p.current = ""
	}
	if len(arg) > 2 {
		// We have "-ab", so return "-a" and keep "-b".
		p.current = arg[:1] + arg[2:]
		arg = arg[:2]
	}
	return arg
}

func (p *flagParser) value() string {
	if len(p.remaining) == 0 {
		return ""
	}
	arg := p.remaining[0]
	p.remaining = p.remaining[1:]
	return arg
}

func (p *flagParser) args() []string { return p.remaining }

type getopts struct {
	argidx  int
	runeidx int
}

func (g *getopts) next(optstr string, args []string) (opt rune, optarg string, done bool) {
	if len(args) == 0 || g.argidx >= len(args) {
		return '?', "", true
	}
	arg := []rune(args[g.argidx])
	if len(arg) < 2 || arg[0] != '-' || arg[1] == '-' {
		return '?', "", true
	}

	opts := arg[1:]
	opt = opts[g.runeidx]
	if g.runeidx+1 < len(opts) {
		g.runeidx++
	} else {
		g.argidx++
		g.runeidx = 0
	}

	i := strings.IndexRune(optstr, opt)
	if i < 0 {
		// invalid option
		return '?', string(opt), false
	}

	if i+1 < len(optstr) && optstr[i+1] == ':' {
		if g.argidx >= len(args) {
			// missing argument
			return ':', string(opt), false
		}
		optarg = args[g.argidx]
		g.argidx++
		g.runeidx = 0
	}

	return opt, optarg, false
}

// optStatusText returns a shell option's status text display
func (r *Runner) optStatusText(status bool) string {
	if status {
		return "on"
	}
	return "off"
}
