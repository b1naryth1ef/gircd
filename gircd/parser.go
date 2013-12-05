package gircd

import "strings"
import "log"

type Msg struct {
	Tag    string
	Values []string
}

func NewMsg(tag string, vals ...string) *Msg {
	return &Msg{
		Tag:    tag,
		Values: vals,
	}
}

func SplitMsg(s string) []string {
	base := make([]string, 0)
	if strings.Contains(s, ":") {
		a := strings.SplitN(s, ":", 2)
		if len(a) != 2 {
			log.Panicf("SplitMsg failure: %s\n", s)
		}
		for _, substr := range strings.Split(a[0], " ") {
			if substr == "" {
				continue
			}
			base = append(base, substr)
		}
		base = append(base, a[1])
		return base
	}
	return strings.Split(s, " ")
}

func NewMsgFrom(data string) *Msg {
	sd := strings.SplitN(data, " ", 2)
	if len(sd) != 2 {
		log.Panicf("NewMsgFrom failure: %s\n", data)
	}
	sf := SplitMsg(sd[1])
	return NewMsg(sd[0], sf...)
}

func (m *Msg) Debug(i *Client) {
	i.LogF("Tag: %s\n", m.Tag)
	i.LogF("Values: %s\n", m.Values)
}

func (m *Msg) Parse(i *Client) {
	er := func(txt string) {
		i.LogF("ParseError: %s (%s, %s, %s)\n", txt, m.Tag, len(m.Values), m.Values)
	}
	i.LogF("Attempting to parse line with tag: '%s'\n", m.Tag)
	if m.Tag == "NICK" {
		if len(m.Values) != 1 {
			er("Need 1 value for nick message")
			return
		}

		// TODO: sanatize
		i.Nick = m.Values[0]
	} else if m.Tag == "USER" {
		if len(m.Values) != 4 {
			er("Need at 4 values for user message")
			return
		}

		// TODO: sanatize
		i.User = m.Values[0]
		i.Mode = m.Values[1]
		i.Unused = m.Values[2]
		i.RealName = m.Values[3]
	} else {
		i.LogF("Failed to parse!\n")
	}
}
