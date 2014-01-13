package gircd

import "fmt"
import "strings"

const (
	R_NETWORK = "NETWORK"
)

const (
	RPL_WELCOME      = "001"
	RPL_YOURHOST     = "002"
	RPL_CREATED      = "003"
	RPL_MYINFO       = "004"
	RPL_TOPIC        = "332"
	RPL_TOPICWHOTIME = "333"
	RPL_NAMREPLY     = "353"
	RPL_ENDOFNAMES   = "366"
	RPL_MOTDSTART    = "375"
	RPL_MOTD         = "372"
	RPL_ENDOFMOTD    = "376"

	// Clients
	CLIENT_JOIN    = "JOIN"
	CLIENT_PART    = "PART"
	CLIENT_PING    = "PING"
	CLIENT_PONG    = "PONG"
	CLIENT_PRIVMSG = "PRIVMSG"

	// Errors
	ERR_UNKNOWNERROR     = "400"
	ERR_NOSUCHCHANNEL    = "403"
	ERR_TOOMANYCHANNELS  = "405"
	ERR_CANNOTSENDTOCHAN = "404"
	ERR_NORECIPIENT      = "411"
	ERR_ERRONEUSNICKNAME = "432"
	ERR_NICKNAMEINUSE    = "433"
	ERR_NOTONCHANNEL     = "442"
	ERR_CHANNELISFULL    = "471"
	ERR_BADCHANNELKEY    = "475"
	ERR_CHANOPRIVSNEEDED = "482"
	ERR_BADPING          = "513"
)

const (
	CHAN_GLOBAL = iota
	CHAN_USER
)

type Response struct {
	Tag     string
	Vars    []interface{}
	Channel int
	Server  *Server
	Client  *Client
}

func NewResponse(tg string, cl *Client, sl *Server) *Response {
	return &Response{
		Tag:     tg,
		Vars:    make([]interface{}, 0),
		Channel: CHAN_GLOBAL,
		Server:  sl,
		Client:  cl,
	}
}

func (r *Response) Set(v interface{}) *Response {
	r.Vars = append(r.Vars, v)
	return r
}

func (r *Response) SetF(s string, vals ...interface{}) *Response {
	r.Set(fmt.Sprintf(s, vals...))
	return r
}

func (r *Response) Build() string {
	base := fmt.Sprintf(strings.Repeat("%s ", len(r.Vars)), r.Vars...)
	if r.Channel == CHAN_GLOBAL {
		return fmt.Sprintf(":%s %s %s %s", r.Server.GetHash(), r.Tag, r.Client.Nick, base)
	} else if r.Channel == CHAN_USER {
		return fmt.Sprintf(":%s %s %s", r.Client.GetHash(), r.Tag, base)
	}
	return ""
}

func (r *Response) Send() {
	r.Channel = CHAN_GLOBAL
	r.Client.Write(r.Build())
}

func (r *Response) Chan(c *Channel) {
	r.Channel = CHAN_USER
	c.Send(r.Build())
}
