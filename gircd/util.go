package gircd

import "regexp"
import "strings"
import "fmt"

var r, _ = regexp.Compile("(?:[a-zA-Z][a-zA-Z0-9\\-\\[\\]\\\\`^{}\\_]*)")

// Take a string s and return a sanitized version of it
func Sanatize(s string) (string, bool) {
	v := r.FindAllString(s, -1)
	if len(v) > 1 {
		return "", false
	}
	return v[0], true
}

type IError struct {
	ID  int
	Msg string
}

//:port80a.se.quakenet.org 432 * 8adsfhj````9345 :Erroneous Nickname
func BuildError(i IError, vars ...interface{}) string {
	res := fmt.Sprintf(strings.Repeat("%s ", len(vars)), vars...)
	return fmt.Sprintf("%s * %s :%s", i.ID, res, i.Msg)
}

var (
	ERR_NOSUCHNICK       = IError{401, ""}
	ERR_NOSUCHSERVER     = IError{402, ""}
	ERR_NOSUCHCHANNEL    = IError{403, ""}
	ERR_ERRONEUSNICKNAME = IError{432, "Erroneous Nickname"}
)
