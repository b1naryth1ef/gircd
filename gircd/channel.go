package gircd

import "fmt"
import "log"

// Enum: Channel Name Prefixes
const (
	// Default channel prefix
	CHAN_PREFIX_DEFAULT = "#"

	// Channel prefix for "server" channels (butchered from spec)
	CHAN_PREFIX_SERVER = "&"
)

// Enum: Channel Modes
const (
	CHAN_MODE_ANON      = "a"
	CHAN_MODE_STICKY    = "g"
	CHAN_MODE_MODERATED = "m"
)

// Enum: Channel User Level Prefixes
const (
	CHAN_OP_PREFIX        = "@"
	CHAN_VOICE_PREFIX     = "+"
	CHAN_GLOBAL_OP_PREFIX = "&"
)

// Returns true if `char` is an acceptable channel prefix
func isChannelPrefix(char string) bool {
	if char == CHAN_PREFIX_DEFAULT || char == CHAN_PREFIX_SERVER {
		return true
	}
	return false
}

// Embedded struct containing channel details
type ChannelInfo struct {
	Mode       *Mode
	MaxMembers int
	Topic      ChannelTopic
}

// Embedded struct containing channel topic details
type ChannelTopic struct {
	Topic string
	Time  int
	Nick  string
}

// Struct used to store a users flags on a channel.
type MemberMode struct {
	Op    bool
	Voice bool
	// Ghost is set when a user is invisible (e.g. lurking)
	Ghost bool
}

// Struct containing data for a single channel
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

// Creates a new instance of Channel
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

// Creates a new instance of ChannelInfo
func NewChannelInfo() ChannelInfo {
	return ChannelInfo{
		Mode:       &Mode{""},
		MaxMembers: 256,
		Topic:      ChannelTopic{},
	}
}

// Checks if the client has a valid MemberMode stored yet
func (c *Channel) HasModesMap(cl *Client) bool {
	if _, c := c.MemberModes[cl.ID]; c {
		return true
	}
	return false
}

// Returns the MemberModes for a client, creating and returing a default
//  MemberMode if one does not already exist
//  NB: Do not call unless user IsMember, or we'll mem leak!
func (c *Channel) GetModes(cl *Client) *MemberMode {
	if !c.HasModesMap(cl) {
		c.MemberModes[cl.ID] = &MemberMode{
			Op:    false,
			Voice: false,
			Ghost: false,
		}
	}
	return c.MemberModes[cl.ID]
}

// Delete a MemberMode from the mapping (used when a clinet leaves)
func (c *Channel) RmvModes(cl *Client) {
	delete(c.MemberModes, cl.ID)
}

// A constant loop that is used to send channel-wide packets
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

// Utility function to queue a message for sending
func (c *Channel) Send(msg string) {
	c.Messages <- msg
}

// Returns the full channel name (e.g. #test)
func (c *Channel) GetName() string {
	return c.Prefix + c.Name
}

// Returns a members full name
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

// Called when client `cl` wants to join the channel
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

// Called when client `cl` wants to part the channel with message `msg`
func (c *Channel) ClientPart(cl *Client, msg string) {
	// r u fukin srs m8
	// fix ur fukin code ya scrub
	new_members := make([]*Client, 0)
	for _, v := range c.Members {
		if v != cl {
			new_members = append(new_members, v)
		}
	}
	c.Members = new_members

	if len(c.Members) > 0 {
		cl.Resp(CLIENT_PART).Set(c.GetName()).SetF(":%s", msg).Chan(c)
		cl.Channels -= 1
	}

	// Some geneirc GC stuff
	c.RmvModes(cl)

	if len(c.Members) == 0 && !c.Mode.HasMode(CHAN_MODE_STICKY) {
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

// Checks if `pw` is equal to the channel pw (default empty string)
func (c *Channel) CheckPassword(pw string) bool {
	return (pw == c.Key)
}

// Checks if client `cl` is a member of the channel
func (c *Channel) IsMember(cl *Client) bool {
	for _, v := range c.Members {
		if v == cl {
			return true
		}
	}
	return false
}

// Sends the channels topic to client `cl`
func (c *Channel) SendTopic(cl *Client) {
	cl.Resp(RPL_TOPIC).Set(c.GetName()).Set(c.Topic.Topic)
	cl.Resp(RPL_TOPICWHOTIME).Set(c.GetName()).Set(c.Topic.Nick).Set(c.Topic.Time)
}

// Sends the channels name listing to client `cl`
func (c *Channel) SendNames(cl *Client) {
	var base string = ""

	send_base := func() {
		cl.Resp(RPL_NAMREPLY).SetF("= %s", c.GetName()).Set(base).Send()
	}

	// This loop makes 510 character long messages
	// TODO: Bug, does not include the base message size in the max line
	//  length. get size of packet.build() and subtract it from MAX_LINE_SIZE
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

// Called when a client `cl` wants to send a message `msg` to the channel
func (c *Channel) Message(cl *Client, msg string) {
	// If the channel is in moderated mode, some clients cant chat
	if c.Mode.HasMode(CHAN_MODE_MODERATED) {
		if !(c.GetModes(cl).Voice || c.GetModes(cl).Op) {
			c.LogF("Client %d cannot chat to this channel, it's moderated!", cl.ID)
			return
		}
	}
	// TODO: do not send this to
	cl.Resp(CLIENT_PRIVMSG).Set(c.GetName()).SetF(":%s", msg).Chan(c)
}
