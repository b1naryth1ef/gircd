package gircd

import "net"
import "fmt"
import "log"
import "sync"
import "time"
import "strings"

// ENUM: ClientState
const (
	STATE_WAIT_PW = iota
	STATE_WAIT_NICK
	STATE_WAIT_USER
	STATE_ACTIVE
	STATE_DEAD
)

type ClientInfo struct {
	// NICK
	Nick string

	/// USER
	User     string
	Mode     *Mode
	Unused   string
	RealName string

	GlobalOp bool

	// Data to match PING and PONG
	PingCode string
}

type Client struct {
	// Internal (server)
	ID       int
	State    int
	Server   *Server
	Conn     net.Conn
	LastPing time.Time

	// Data-plexes
	Lock    *sync.RWMutex
	Updates []*Update
	MsgQ    chan *Msg

	// Used in rate-limiting
	Messages int

	// Marks how many channels a client is in
	Channels int

	ClientInfo
}

var UPDATE_TIME = time.Millisecond * 350

// Called to create a new client, need an id, the server, and the network
//  connection
func NewClient(id int, server *Server, c net.Conn) *Client {
	cli := &Client{
		Conn:     c,
		Server:   server,
		ID:       id,
		MsgQ:     make(chan *Msg, 1),
		Updates:  make([]*Update, 0),
		Lock:     new(sync.RWMutex),
		LastPing: time.Now(),
		ClientInfo: ClientInfo{
			Mode:     &Mode{""},
			Nick:     "",
			GlobalOp: true,
		},
	}

	// Set our timeout pretty high
	cli.Conn.SetReadDeadline(time.Now().Add(time.Millisecond * 1))

	// Change the state depending on if we need a password or nick
	if server.HasPassword() {
		cli.State = STATE_WAIT_PW
	} else {
		cli.State = STATE_WAIT_NICK
	}

	// Write empty line for lulz
	cli.Write("")

	// Return ourselves for chaining
	return cli
}

// Gets the user hash
func (c *Client) GetHash() string {
	return fmt.Sprintf("%s!%s@%s", c.Nick, c.User, c.GetAddr())
}

// Grabs a write lock and changes the state
func (c *Client) SetState(s int) {
	c.Lock.Lock()
	c.State = s
	c.Lock.Unlock()
}

// Get addr
func (c *Client) GetAddr() string {
	return strings.Split(c.Conn.RemoteAddr().String(), ":")[0]
}

// Grabs a read lock and checks whether we have timed out
func (c *Client) CheckPing() bool {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	if time.Now().Sub(c.LastPing) > PING_TIMEOUT {
		return false
	}
	return true
}

// Creates a new response towards the client
func (c *Client) Resp(tag string) *Response {
	r := NewResponse(tag, c, c.Server)
	r.Server = c.Server
	return r
}

// Resets our ping
func (c *Client) MarkPing() {
	c.Lock.Lock()
	c.LastPing = time.Now()
	c.Lock.Unlock()
}

// Called after the AUTH process is done
func (c *Client) Init() {
	c.Resp(RPL_WELCOME).SetF("Welcome to %s %s! %s@%s", c.Server.Name, c.Nick, c.User, c.GetAddr()).Send()
	c.Resp(RPL_MYINFO).Set(c.Server.Name).Send()
	c.SendMOTD()
}

// Send MOTD
func (c *Client) SendMOTD() {
	c.Resp(RPL_MOTDSTART).Set(":- MESSAGE OF THE DAY -")
	for _, v := range c.Server.MOTD {
		c.Resp(RPL_MOTD).SetF(":%s", v).Send()
	}
	c.Resp(RPL_ENDOFMOTD).Set(":End of /MOTD command.")
}

// Writes a string + LINE_TERM
func (c *Client) Write(l string) {
	l = l + LINE_TERM
	c.Conn.Write([]byte(l))
}

// Formats and writes a string
func (c *Client) WriteF(l string, vars ...interface{}) {
	c.Write(fmt.Sprintf(l, vars...))
}

// Forces a user to disconnect from the server (e.g. kick)
func (c *Client) ForceDC(s string) {
	// Part all channels
	for _, v := range c.Server.Channels {
		c.LogF("checking chan %s", v.GetName())
		if v.IsMember(c) {
			c.Log("yes!")
			v.ClientPart(c, s)
		}
	}

	c.SetState(STATE_DEAD)
	c.WriteF("QUIT :%s", s)
	c.Conn.Close()
	c.Server.RmvClient(c.ID)
}

// Writes a server error
func (c *Client) ErrorS(err string) {
	c.WriteF(":%s %s", c.Server.GetHash(), err)
}

// Logs something wtih the client id
func (c *Client) Log(l string) {
	log.Printf("Client <%d>: %s", c.ID, l)
}

// Formats and logs something with the client id
func (c *Client) LogF(l string, vars ...interface{}) {
	c.Log(fmt.Sprintf(l, vars...))
}

// Returns true if an update is required
func (c *Client) NeedUpdate() bool {
	return len(c.Updates) > 0
}

// Forces and update for each active update
func (c *Client) Update() {
	for _, i := range c.Updates {
		if i.NeedSense(UPDATE_TIME) {
			i.Update()
		}
	}
}
