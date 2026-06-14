package cmd

import "strings"

// shellQuote returns s wrapped so it is a single, literal argument when the
// string is interpreted by a POSIX shell (which is how srv.ExecuteCommand runs
// remote commands — via the login shell). Every embedded single quote is
// rewritten as '\” and the whole value is wrapped in single quotes, so shell
// metacharacters in user-supplied values (secret keys/values, app names, commit
// refs) cannot break out and inject commands.
//
//	shellQuote(`a'b; rm -rf /`) => `'a'\''b; rm -rf /'`
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
