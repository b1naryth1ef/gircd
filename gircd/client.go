package gircd

import "net"
import "fmt"
import "log"
import "sync"
import "time"

//import "strings"

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
	Mode     string
	Unused   string
	RealName string
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

	ClientInfo
}

var UPDATE_TIME = time.Millisecond * 350

// Called to create a new client, need an id, the server, and the network
//  connection
func NewClient(id int, server *Server, c net.Conn) *Client {
	cli := &Client{
		Conn:       c,
		Server:     server,
		ID:         id,
		MsgQ:       make(chan *Msg, 1),
		Updates:    make([]*Update, 0),
		Lock:       new(sync.RWMutex),
		LastPing:   time.Now(),
		ClientInfo: ClientInfo{},
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

	// Queue a timeout update
	NewUpdate(UPDATE_LOGIN_TIMEOUT, cli).Set("start", time.Now()).Queue()

	// Return ourselves for chaining
	return cli
}

// Grabs a write lock and changes the state
func (c *Client) SetState(s int) {
	c.Lock.Lock()
	c.State = s
	c.Lock.Unlock()
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

// Resets our ping
func (c *Client) MarkPing() {
	c.Lock.Lock()
	c.LastPing = time.Now()
	c.Lock.Unlock()
}

// Called after the AUTH process is done
func (c *Client) Init() {

}

// Writes a string + LINE_TERM
func (c *Client) Write(l string) {
	l = l + LINE_TERM
}

// Formats and writes a string
func (c *Client) WriteF(l string, vars ...interface{}) {
	c.Write(fmt.Sprintf(l, vars...))
}

// Forces a user to disconnect from the server (e.g. kick)
func (c *Client) ForceDC(s string) {
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
