//go:build solaris || illumos

package shell

// coreUtilsExecHandler is nil on illumos and solaris: mvdan.cc/sh's coreutils
// package does not compile there, so the Go coreutils middleware is simply
// unavailable. standardHandlers skips it when the handler is nil.
var coreUtilsExecHandler execMiddleware
