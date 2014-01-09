package gircd

import "strings"
import "log"

type Msg struct {
	Tag    string
	Values []string
	Client *Client
}

// Function definition for a parser function
type ParserF func(i *Client, m *Msg)

// Create a map of parser functions
var PARSERS map[string]ParserF = make(map[string]ParserF)

// Add a function f with tag s to the parsers
func PF(s string, f ParserF) {
	PARSERS[s] = f
}

func HasParserFunc(s string) bool {
	if _, has := PARSERS[s]; has {
		return true
	}
	return false
}

func GetParserFunc(s string) ParserF {
	val, _ := PARSERS[s]
	return val
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
			log.Printf("[WARN] SplitMsg failure: %s\n", s)
			return nil
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
		log.Printf("[WARN] NewMsgFrom failure: %s\n", data)
		return nil
	}
	sf := SplitMsg(sd[1])
	return NewMsg(sd[0], sf...)
}

func (m *Msg) Debug(i *Client) {
	i.LogF("Tag: %s\n", m.Tag)
	i.LogF("Values: %s\n", m.Values)
}

func (m *Msg) Parse(i *Client) {
	m.Client = i

	er := func(txt string) {
		i.LogF("ParseError: %s (%s, %s, %s)\n", txt, m.Tag, len(m.Values), m.Values)
	}
	i.LogF("Attempting to parse line with tag: '%s'\n", m.Tag)

	// Do we have a parser to handle this?
	if !HasParserFunc(m.Tag) {
		er("Could not find parser function for tag!")
		return
	}

	// Actually parse the message
	GetParserFunc(m.Tag)(i, m)
}

func (m *Msg) Error(s string) {
	m.Client.LogF("ParseError: %s (%s, %s, %s)\n", s, m.Tag, len(m.Values), m.Values)
}

func InitParser() {
	PF("NICK", func(i *Client, m *Msg) {
		// The user can only change nick for first auth, or after auth is complete
		if i.State != STATE_WAIT_NICK && i.State != STATE_ACTIVE {
			m.Error("Not waiting for nick (not WAIT_NICK or ACTIVE)")
			return
		}

		// Require exactly one value
		if len(m.Values) != 1 {
			m.Error("Need 1 value for NICK message")
			return
		}

		n, e := Sanatize(m.Values[0])
		if !e {
			m.Error("Nickname is invalid for NICK message")
			// TODO: limit size of nickname
			i.ErrorS(BuildError(ERR_ERRONEUSNICKNAME, m.Values[0]))
		}

		// Check if the nick is already in use
		if m.Client.Server.FindUserByNick(n) != nil {
			m.Error("Nick in use")
			i.Resp(ERR_NICKNAMEINUSE).Set(n).Set(":That nickname is already in use!")
		}

		i.Nick = n
		i.SetState(STATE_WAIT_USER)
	})

	PF("USER", func(i *Client, m *Msg) {
		// The user can only send USER messages on auth
		if i.State != STATE_WAIT_USER {
			m.Error("Not waiting for user (not WAIT_USER)")
			return
		}

		// Require exactly four values
		if len(m.Values) != 4 {
			m.Error("Need exactly 4 values for user message")
			return
		}

		// TODO: Sanatize, find a good way to do it
		i.User = m.Values[0]
		i.Mode = m.Values[1]
		i.Unused = m.Values[2]
		i.RealName = m.Values[3]

		// The user is authed up and ready to go
		i.SetState(STATE_ACTIVE)
		i.Init()
	})

	PF("JOIN", func(i *Client, m *Msg) {
		// User must be authed and active
		if i.State != STATE_ACTIVE {
			m.Error("Active state required for JOIN")
			return
		}

		// Require 1 to 2 values for JOIN
		if len(m.Values) != 2 && len(m.Values) != 1 {
			m.Error("JOIN requires 1-2 values")
			return
		}

		// A user can join up to MAX_CHANNELS, and otherwise is denied
		if i.Channels > MAX_CHANNELS {
			i.Resp(ERR_TOOMANYCHANNELS).Set(m.Values[0]).SetF(":You may join a maximum of %d channels!", MAX_CHANNELS)
			return
		}

		// Null password by default
		var password string = ""

		// Set password if provided
		if len(m.Values) == 2 {
			password = m.Values[1]
		}

		// Get a channel (either by creating it, or grabbing it)
		var c *Channel
		if i.Server.HasChannel(m.Values[0]) {
			c = i.Server.GetChannel(m.Values[0])
		} else {
			c = i.Server.NewChannel(string(m.Values[0][0]), m.Values[0][1:])
		}

		// This is a weird edge case in the RFC, there is no valid reply to a user trying to join
		//  a channel they are already on. In this case we just ignore it. Maybe NOTICE?
		if c.IsMember(i) {
			m.Error("Client is already a member of the channel")
			return
		}

		if !c.CheckPassword(password) {
			i.Resp(ERR_BADCHANNELKEY).Set(c.GetName()).Set(":Invalid Key For Channel!")
			return
		}

		c.ClientJoin(i)
	})

	PF("QUIT", func(i *Client, m *Msg) {
		i.ForceDC("Client Quit")
	})
}
