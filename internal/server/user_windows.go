//go:build windows

package server

import "golang.org/x/sys/windows"

// currentUserID returns the SID of the user that owns the current
// process, the same string [os/user.Current] reports as Uid, or "" if it
// cannot be read. Unlike user.Current it does not also look up the user's
// profile directory and account name, so it keeps working when only those
// lookups fail.
func currentUserID() string {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return ""
	}
	return tu.User.Sid.String()
}
