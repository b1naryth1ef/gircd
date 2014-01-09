package gircd

import "sync"
import "time"
import "log"

// Etc
const (
	// How long it takes before login times out
	LOGIN_TIMEOUT_DUR = time.Second * 30
)

// ENUM: UpdateType
const (
	UPDATE_GENERIC       = iota
	UPDATE_LOGIN_TIMEOUT // Timeout the client on login failure
)

type Update struct {
	TYPE   int
	DATA   map[string]interface{}
	CLIENT *Client
	LOCK   *sync.RWMutex
	LAST   time.Time
	VALID  bool
}

func NewUpdate(t int, c *Client) *Update {
	return &Update{
		TYPE:   t,
		DATA:   make(map[string]interface{}, 0),
		CLIENT: c,
		LOCK:   new(sync.RWMutex),
		LAST:   time.Now().Add(time.Now().Sub(time.Now().Add(time.Second * 600))),
		VALID:  true,
	}
}

func (u *Update) Queue() *Update {
	u.CLIENT.Log("Queued New Update!")
	u.CLIENT.Updates = append(u.CLIENT.Updates, u)
	return u
}

func (u *Update) Set(s string, i interface{}) *Update {
	u.DATA[s] = i
	return u
}

func (u *Update) Get(s string) interface{} {
	v, _ := u.DATA[s]
	return v
}

func (u *Update) MarkValid(v bool) {
	u.LOCK.Lock()
	u.VALID = v
	u.LOCK.Unlock()
}

func (u *Update) MarkLast() {
	u.LOCK.Lock()
	u.LAST = time.Now()
	u.LOCK.Unlock()
}

func (u *Update) Update() bool {
	u.LOCK.RLock()
	defer u.LOCK.RUnlock()
	if !u.VALID {
		log.Printf("Update isnt valid, skipping...")
		return false
	}

	// We write-lock, and make sure no one else bothers updating this guy
	u.MarkValid(false)

	// Handle UPDATE_LOGIN_TIMEOUT
	if u.TYPE == UPDATE_LOGIN_TIMEOUT {
		// Has it been long enough
		if (time.Now().Sub(u.Get("start").(time.Time))) > LOGIN_TIMEOUT_DUR {
			// Is the user still in the correct state
			u.CLIENT.Lock.RLock()
			c := (u.CLIENT.State == STATE_WAIT_NICK || u.CLIENT.State == STATE_WAIT_USER)
			u.CLIENT.Lock.RUnlock()

			if c {
				u.CLIENT.Log("LOGIN_TIMEOUT was fired, forcing disconnect")
				u.CLIENT.ForceDC("Login timed out!")
				return true
			}
		}
	}

	// If we get here, it means no condition for firing the update was met, reset
	u.MarkValid(true)
	// Mark u.LAST to time.Now
	u.MarkLast()
	return false
}

func (u *Update) NeedSense(i time.Duration) bool {
	if (time.Now().Sub(u.LAST)) > i {
		return true
	}
	return false
}
