package gircd

import (
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

const (
	LINE_TERM     = "\r\n"
	PING_TIMEOUT  = time.Second * 120
	RECV_BUF_SIZE = 2048
	BREATH_TIME   = time.Millisecond * 5
	MAX_CHANNELS  = 64
	MAX_LINE_SIZE = 510

	// N packets per 5 seconds must be less than this
	MESSAGES_PER_5_SEC = 10
)

type ServerInfo struct {
	Name    string
	Version int
	MOTD    []string
}

type Server struct {
	Conn     net.Listener
	Clients  map[int]*Client
	Channels map[string]*Channel

	// Config
	Host     string
	Port     string
	Password string

	// Is the server running?
	running bool
	id_inc  int

	ServerInfo
}

func NewServer(host string, port string, password string) *Server {
	return &Server{
		Clients:  make(map[int]*Client, 0),
		Channels: make(map[string]*Channel, 0),
		running:  false,
		id_inc:   0,
		Host:     host,
		Port:     port,
		Password: password,
		ServerInfo: ServerInfo{
			Name:    "GIRCD",
			Version: 1,
			MOTD: []string{
				"Welcome to GIRCD Test server!",
				"Please enjoy your stay and be nice!",
			},
		},
	}
}

func (s *Server) FindUserByNick(nick string) *Client {
	for _, v := range s.Clients {
		if v.Nick == nick {
			return v
		}
	}
	return nil
}

func (s *Server) NewChannel(prefix string, name string) *Channel {
	channel := NewChannel(prefix, name, s)
	go channel.SendLoop()
	s.Channels[channel.GetName()] = channel
	return channel
}

func (s *Server) GetChannel(name string) *Channel {
	return s.Channels[name]
}

func (s *Server) RmvChannel(c *Channel) {
	// Force loop to stop
	c.Alive = false
	c.Send("")
	delete(s.Channels, c.GetName())
}

func (s *Server) HasChannel(name string) bool {
	if _, c := s.Channels[name]; c {
		return true
	}
	return false
}

func (s *Server) GetHash() string {
	return s.Host
}

// Returns the next availible ID, skips over used ID's
func (s *Server) NextID() int {
	for s.HasClient(s.id_inc) {
		s.id_inc += 1
	}
	return s.id_inc
}

// Checks if a client of ID i exists
func (s *Server) HasClient(i int) bool {
	if _, c := s.Clients[i]; c {
		return true
	}
	return false
}

// Returns a client (or nil) of ID i
func (s *Server) GetClient(i int) *Client {
	v, _ := s.Clients[i]
	return v
}

// Removes a client
func (s *Server) RmvClient(i int) bool {
	if s.HasClient(i) {
		delete(s.Clients, i)
		return true
	}
	return false
}

// Adds a client
func (s *Server) AddClient(c *Client) {
	if s.HasClient(c.ID) {
		if s.GetClient(c.ID) == c {
			log.Printf("Warning: AddClient is ignoring request, client was already added!")
			return
		}
		log.Printf("[WARN] AddClient failed, client w/ ID %s already exists and is not identical!", c.ID)
		return
	}
	s.Clients[c.ID] = c
}

// Loops over s.Conn.Accept()
func (s *Server) AcceptLoop() {
	for s.running {
		conn, err := s.Conn.Accept()
		if err != nil {
			log.Printf("Could not accept new connection: %s\n", err)
			continue
		}

		id := s.NextID()
		log.Printf("Accepting new connection: %d\n", id)
		cli := NewClient(id, s, conn)
		s.AddClient(cli)

		NewUpdate(UPDATE_LOGIN_TIMEOUT, cli).Set("start", time.Now()).Queue()
	}
}

// Loops over clients and pulls from the update queue
func (s *Server) UpdateLoop() {
	for s.running {
		for _, v := range s.Clients {
			if v.NeedUpdate() {
				v.Update()
			}
		}
		// Breath
		time.Sleep(time.Millisecond * 100)
	}
}

// Loops over clients and checks to see if they have timed out
func (s *Server) PingLoop() {
	for s.running {
		for _, v := range s.Clients {
			if !v.CheckPing() {
				v.Log("Client timed out on ping!")
				v.ForceDC("Ping Timeout")
			}

			if v.Messages > MESSAGES_PER_5_SEC {
				v.LogF("Client %s seems to be spamming... kicking!")
				v.ForceDC("Rate Limiting")
			}
			v.Lock.Lock()
			v.Messages = 0
			v.Lock.Unlock()
		}
		time.Sleep(time.Second * 5)
	}
}

// Reads
func (s *Server) ReadLoop() {
	for s.running {
		for _, v := range s.Clients {
			buff := make([]byte, RECV_BUF_SIZE)
			c, e := v.Conn.Read(buff)
			if e != nil {
				if nerr, ok := e.(net.Error); ok && nerr.Timeout() {
					continue
				}
				if e == io.EOF {
					v.LogF("Client Closed Connection...\n")
					v.ForceDC("Client Closed Connection")
					continue
				}
				v.LogF("Error reading: %s\n", e)
				continue
			}
			v.LogF("Read bytes: %s\n", c)
			if c > MAX_LINE_SIZE {
				v.Log("WARNING: Line size above MAX_LINE_SIZE, skipping...")
				v.ForceDC("Line size over maximum")
				continue
			}
			if c > 0 {
				data := string(buff[:c])
				for _, line := range strings.Split(data, LINE_TERM) {
					if len(line) == 0 {
						continue
					}
					msg := NewMsgFrom(line)
					if msg == nil {
						continue
					}
					v.Messages += 1
					v.MsgQ <- msg
					v.LogF("Added msg to buffer!\n")
				}
			}
		}
		time.Sleep(BREATH_TIME)
	}
}

func (s *Server) ParseLoop() {
	for s.running {
		for _, v := range s.Clients {
			if len(v.MsgQ) > 0 {
				v.LogF("Parsing Queue...\n")
				val := <-v.MsgQ
				val.Parse(v)
			}
		}
		time.Sleep(BREATH_TIME)
	}
}

func (s *Server) Start() {
	ln, err := net.Listen("tcp", ":"+s.Port)
	if err != nil {
		log.Panicf("Could not listen: %s\n", err)
	}
	s.running = true
	s.Conn = ln

	log.Printf("Loading Parser")
	InitParser()

	log.SetOutput(os.Stdout)
	log.Printf("Running!")
	// go s.PingLoop()
	go s.ReadLoop()
	go s.ParseLoop()
	s.AcceptLoop()
}

func (s *Server) HasPassword() bool {
	if s.Password != "" {
		return true
	}
	return false
}
