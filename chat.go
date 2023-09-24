package main

import (
"net"
"os"
"fmt"
"strings"
"bufio"
"regexp"
)

type ChatMessage struct {
	from uint
	mtype uint
	data string
}

type ChatServer struct {
	port uint16
	numSessions uint
	q chan ChatMessage
	clients map[uint]*ChatSession
}

func NewChatServer(port uint16) *ChatServer {
	return &ChatServer{
		port,
		0,
		make(chan ChatMessage),
		make(map[uint]*ChatSession),
	}
}

type ChatSession struct{
	sessionId uint
	username string
	q chan string
	backend chan ChatMessage
	con net.Conn
	isOnline bool
}

func NewChatSession(sessionId uint, con net.Conn, backend chan ChatMessage) *ChatSession {
	return &ChatSession{
		sessionId,
		"",
		make(chan string),
		backend,
		con,
		false,
	}
}

func (s *ChatServer) broadcaster(){
	
	for m := range s.q {
		client, ok := s.clients[m.from]
		if ok != true { continue }
		switch m.mtype {
			case 1: s.handleJoin(m, client)
			case 2: s.handleMsg(m, client)
			case 3: s.handleLeave(m, client)
		}
	}
}

func (s *ChatServer) chatNames(except uint) string {
	first := true
	result := ""
	for key, session := range s.clients {
		if key == except {
		  continue
		}
    if first {
			first = false
    } else {
			result += ", "
    }
		result += session.username
  }
	if first {
		result = "empty space"
	}
	return result
}
func (s *ChatServer) handleJoin(m ChatMessage, c *ChatSession) {
	msg := fmt.Sprintf("* You see %s floating around here\n", s.chatNames(m.from))
	c.q <- msg
	msg = fmt.Sprintf("* %s just docked!\n", c.username)
	fmt.Printf(msg)
	for key, session := range s.clients {
		if key == m.from || session.isOnline == false {
		  continue
		}
    session.q <- msg
  }
}

func (s *ChatServer) handleMsg(m ChatMessage, c *ChatSession) {
	msg := fmt.Sprintf("[%s] %s\n", c.username, m.data)
	fmt.Printf(msg)
	for key, session := range s.clients {
		if key == m.from || session.isOnline == false{
		  continue
		}
    session.q <- msg
  }
}

func (s *ChatServer) handleLeave(m ChatMessage, c *ChatSession) {
	msg := fmt.Sprintf("* %s just fired up his hyperdrive!\n", c.username)
	delete(s.clients, c.sessionId)
	fmt.Println(msg)
	for key, session := range s.clients {
		if key == m.from || session.isOnline == false {
		  continue
		}
    session.q <- msg
  }
}

func (s *ChatServer) Run() {
	go s.broadcaster()
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("ChatServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      fmt.Println("client connected")
      s.processClient(connection)
		}
		close(s.q)
}

func (s *ChatServer) processClient(con net.Conn) {
	sessionId := s.numSessions
	s.numSessions += 1
	session := NewChatSession( sessionId, con, s.q)
	s.clients[sessionId] = session
	go session.chatReceiver()
	go session.chatSender()
}

func chatUsernameValid(username string) bool {
	var invalidUsername = regexp.MustCompile(`[^0-9a-zA-Z]`)
	
	if len(username) < 1 {
		return false
	}
	return ! invalidUsername.MatchString(username)
}
func (s *ChatSession) logon(r *bufio.Reader) error {
	fmt.Fprintf(s.con, "Howdy! welcome to station 42!\n")
	username, err := r.ReadString('\n')
	if err != nil { return err }
	username = strings.TrimSpace(username)
	if ! chatUsernameValid(username) {
		return fmt.Errorf("Invalid username %s", username)
	}
	s.isOnline = true
	s.username = username
	m := ChatMessage{s.sessionId, 1, username}
	s.backend <- m
	return nil
}

func (s *ChatSession) chat(r *bufio.Reader) error {
	msg, err := r.ReadString('\n')
	if err != nil { return err }
	msg = strings.TrimSpace(msg)
	m := ChatMessage{s.sessionId, 2, msg}
	s.backend <- m
	return nil
}

func (s *ChatSession) close() {
	fmt.Printf("close %d\n", s.sessionId)
	if s.isOnline {
  	m := ChatMessage{s.sessionId, 3, s.username}
	  s.backend <- m
	}
	s.isOnline = false
	close(s.q)
  s.con.Close()
}

func (s *ChatSession) chatReceiver() {
	defer s.close()
	r := bufio.NewReader(s.con)
	err := s.logon(r)
	for err == nil {
		err = s.chat(r)
	}
}

func (s *ChatSession) chatSender() {
	for m := range s.q {
		s.con.Write([]byte(m))
	}
}

