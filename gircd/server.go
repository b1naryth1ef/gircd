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
)

type Server struct {
	Conn    net.Listener
	Clients map[int]*Client

	// Config
	Port     string
	Password string

	// Is the server running?
	running bool
	id_inc  int
}

func NewServer(port string, password string) *Server {
	return &Server{
		Clients:  make(map[int]*Client, 0),
		running:  false,
		id_inc:   0,
		Port:     port,
		Password: password,
	}
}

func (s *Server) GetHash() string {
	return "localhost:" + string(s.Port)
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
		log.Panicf("AddClient failed, client w/ ID %s already exists and is not identical!", c.ID)
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
		cli.Init()
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
				}
				v.LogF("Error reading: %s\n", e)
				continue
			}
			v.LogF("Read bytes: %s\n", c)
			if c > 0 {
				data := string(buff[:c])
				for _, line := range strings.Split(data, LINE_TERM) {
					if len(line) == 0 {
						continue
					}
					v.MsgQ <- NewMsgFrom(line)
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
	//go s.UpdateLoop()
	//go s.PingLoop()
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
