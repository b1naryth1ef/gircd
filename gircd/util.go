package gircd

import "regexp"
import "strings"

var r, _ = regexp.Compile("(?:[a-zA-Z][a-zA-Z0-9\\-\\[\\]\\\\`^{}\\_]*)")

// Take a string s and return a sanitized version of it
func Sanatize(s string) (string, bool) {
	v := r.FindAllString(s, -1)
	if len(v) > 1 {
		return "", false
	}
	return v[0], true
}

type Mode struct {
	Modes string
}

func (m *Mode) HasMode(c string) bool {
	return strings.Contains(m.Modes, c)
}

func (m *Mode) AddMode(c string) {
	if !m.HasMode(c) {
		m.Modes = m.Modes + c
	}
}

func (m *Mode) RmvMode(c string) {
	if m.HasMode(c) {
		m.Modes = strings.Replace(m.Modes, c, "", -1)
	}
}
