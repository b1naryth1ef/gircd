package gircd

import "net"
import "fmt"
import "log"
import "sync"
import "time"

// ENUM: ClientState
const (
	STATE_WAIT_PW = iota
	STATE_WAIT_NICK
	STATE_WAIT_USER
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

	cli.Conn.SetReadDeadline(time.Now().Add(time.Millisecond * 1))

	if server.HasPassword() {
		cli.State = STATE_WAIT_PW
	} else {
		cli.State = STATE_WAIT_NICK
	}

	cli.Write("")

	// Queue a timeout update
	NewUpdate(UPDATE_LOGIN_TIMEOUT, cli).Set("start", time.Now()).Queue()
	return cli
}

func (c *Client) SetState(s int) {
	c.Lock.Lock()
	c.State = s
	c.Lock.Unlock()
}

func (c *Client) CheckPing() bool {
	c.Lock.RLock()
	defer c.Lock.RUnlock()
	if time.Now().Sub(c.LastPing) > PING_TIMEOUT {
		return false
	}
	return true
}

func (c *Client) MarkPing() {
	c.Lock.Lock()
	c.LastPing = time.Now()
	c.Lock.Unlock()
}

func (c *Client) Init() {

}

func (c *Client) Write(l string) {
	l = l + LINE_TERM
}

func (c *Client) WriteF(l string, vars ...interface{}) {
	c.Write(fmt.Sprintf(l, vars...))
}

func (c *Client) ForceDC(s string) {
	c.SetState(STATE_DEAD)
	c.WriteF("QUIT :%s", s)
	c.Conn.Close()
	c.Server.RmvClient(c.ID)
}

func (c *Client) Log(l string) {
	log.Printf("Client <%d>: %s", c.ID, l)
}

func (c *Client) LogF(l string, vars ...interface{}) {
	c.Log(fmt.Sprintf(l, vars...))
}

func (c *Client) NeedUpdate() bool {
	return len(c.Updates) > 0
}

func (c *Client) Update() {
	for _, i := range c.Updates {
		if i.NeedSense(UPDATE_TIME) {
			i.Update()
		}
	}
}
