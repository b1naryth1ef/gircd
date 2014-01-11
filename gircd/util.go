package gircd

import "regexp"

var r, _ = regexp.Compile("(?:[a-zA-Z][a-zA-Z0-9\\-\\[\\]\\\\`^{}\\_]*)")

// Take a string s and return a sanitized version of it
func Sanatize(s string) (string, bool) {
	v := r.FindAllString(s, -1)
	if len(v) > 1 {
		return "", false
	}
	return v[0], true
}
