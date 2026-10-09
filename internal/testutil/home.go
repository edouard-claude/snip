// Package testutil holds helpers shared by the tests of several packages.
package testutil

import "testing"

// SetHome makes dir the home directory for the duration of the test.
//
// os.UserHomeDir reads HOME on Unix and USERPROFILE on Windows, so both are
// set. Setting only HOME leaves a test on Windows reading and writing the real
// home directory of whoever runs it.
func SetHome(t testing.TB, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}
