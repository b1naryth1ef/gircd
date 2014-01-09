package gircd

import "strings"
import "fmt"
import "log"

const (
	// Default channel prefix
	CHAN_PREFIX_DEFAULT = "#"

	// Channel prefix for "server" channels (butchered from spec)
	CHAN_PREFIX_SERVER = "&"
)

const (
	CHAN_MODE_ANON   = "a"
	CHAN_MODE_STICKY = "g"
)

const (
	CHAN_OP_PREFIX        = "@"
	CHAN_VOICE_PREFIX     = "+"
	CHAN_GLOBAL_OP_PREFIX = "&"
)

type ChannelInfo struct {
	Mode       string
	MaxMembers int
	Topic      ChannelTopic
}

type ChannelTopic struct {
	Topic string
	Time  int
	Nick  string
}

type MemberMode struct {
	Op    bool
	Voice bool
	Ghost bool
}

func NewChannelInfo() ChannelInfo {
	return ChannelInfo{
		Mode:       "",
		MaxMembers: 256,
		Topic:      ChannelTopic{},
	}
}

type Channel struct {
	Prefix string
	Name   string
	Key    string

	// Ref to server
	Server *Server

	// List of references to active members
	Members     []*Client
	MemberModes map[int]*MemberMode

	// Messages Queue
	Messages chan string

	Alive bool

	ChannelInfo
}

func NewChannel(prefix string, name string, server *Server) *Channel {
	return &Channel{
		Prefix:      prefix,
		Name:        name,
		Key:         "",
		Server:      server,
		Members:     make([]*Client, 0),
		MemberModes: make(map[int]*MemberMode, 0),
		Messages:    make(chan string, 1),
		ChannelInfo: NewChannelInfo(),
		Alive:       true,
	}
}

func (c *Channel) HasModes(cl *Client) bool {
	if _, c := c.MemberModes[cl.ID]; c {
		return true
	}
	return false
}

func (c *Channel) GetModes(cl *Client) *MemberMode {
	if !c.HasModes(cl) {
		c.MemberModes[cl.ID] = &MemberMode{
			Op:    false,
			Voice: false,
			Ghost: false,
		}
	}
	return c.MemberModes[cl.ID]
}

func (c *Channel) RmvModes(cl *Client) {
	delete(c.MemberModes, cl.ID)
}

func (c *Channel) SendLoop() {
	for msg := range c.Messages {
		if !c.Alive {
			return
		}
		c.LogF("Sending msg `%s` to `%d` members in `%s`", msg, len(c.Members), c.GetName())
		for _, v := range c.Members {
			v.Write(msg)
		}
	}
}

func (c *Channel) Send(msg string) {
	c.Messages <- msg
}

func (c *Channel) GetName() string {
	return c.Prefix + c.Name
}

func (c *Channel) GetMemberName(cl *Client) string {
	var prefix string = ""
	modes := c.GetModes(cl)
	if modes.Op {
		prefix = CHAN_OP_PREFIX
	} else if modes.Voice {
		prefix = CHAN_VOICE_PREFIX
	} else if cl.GlobalOp {
		prefix = CHAN_GLOBAL_OP_PREFIX
	}
	return prefix + cl.Nick
}

// Checks if the channel has mode `char` set
func (c *Channel) HasMode(char string) bool {
	return strings.Contains(c.Mode, char)
}

// Adds the chanmode flag `char` to the channel modes
func (c *Channel) AddMode(char string) {
	if c.HasMode(char) {
		return
	}
	c.Mode = c.Mode + char
}

// Removes the chanmode flag `char` from the channel modes
func (c *Channel) RmvMode(char string) {
	c.Mode = strings.Replace(c.Mode, char, "", -1)
}

func (c *Channel) ClientJoin(cl *Client) {
	if len(c.Members) > c.MaxMembers {
		cl.Resp(ERR_CHANNELISFULL).Set(c.GetName()).SetF(":Channel is full (%d/%d members)",
			len(c.Members), c.MaxMembers).Send()
		return
	}
	cl.Channels += 1
	c.Members = append(c.Members, cl)
	cl.Resp(CLIENT_JOIN).Set(c.GetName()).Chan(c)
	c.SendTopic(cl)
	c.SendNames(cl)
}

func (c *Channel) ClientPart(cl *Client, msg string) {
	cl.Resp(CLIENT_PART).Set(c.GetName()).SetF(":%s", msg).Chan(c)
	cl.Channels -= 1

	// r u fukin srs m8
	// fix ur fukin code ya scrub
	new_members := make([]*Client, 0)
	for _, v := range c.Members {
		if v != cl {
			new_members = append(new_members, v)
		}
	}
	c.Members = new_members

	// Some geneirc GC stuff
	c.RmvModes(cl)

	if len(c.Members) == 0 && !c.HasMode(CHAN_MODE_STICKY) {
		c.Log("Channel has 0 members and has no sticky mode set, GCing...")
		c.Server.RmvChannel(c)
	}
}

// Logs something wtih the channel name
func (c *Channel) Log(l string) {
	log.Printf("Channel <%s>: %s", c.GetName(), l)
}

// Formats and logs something with the channel name
func (c *Channel) LogF(l string, vars ...interface{}) {
	c.Log(fmt.Sprintf(l, vars...))
}

func (c *Channel) CheckPassword(pw string) bool {
	return (pw == c.Key)
}

func (c *Channel) IsMember(cl *Client) bool {
	for _, v := range c.Members {
		if v == cl {
			return true
		}
	}
	return false
}

func (c *Channel) SendTopic(cl *Client) {
	cl.Resp(RPL_TOPIC).Set(c.GetName()).Set(c.Topic.Topic)
	cl.Resp(RPL_TOPICWHOTIME).Set(c.GetName()).Set(c.Topic.Nick).Set(c.Topic.Time)
}

func (c *Channel) SendNames(cl *Client) {
	var base string = ""

	send_base := func() {
		cl.Resp(RPL_NAMREPLY).SetF("= %s", c.GetName()).Set(base).Send()
	}

	for _, v := range c.Members {
		name := c.GetMemberName(v)
		if len(base+name) > MAX_LINE_SIZE {
			send_base()
			base = ""
		}
		base = base + " " + name
	}
	if base != "" {
		send_base()
	}

	cl.Resp(RPL_ENDOFNAMES).Set(c.GetName()).Set(":End of /NAMES list.").Send()
}
