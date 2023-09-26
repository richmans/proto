package main

import (
"net"
"os"
"fmt"
"time"
"encoding/binary"
)

var BE = binary.BigEndian

type SpeedMessage interface {
	serialize() []byte
}

type ErrorMessage struct {
	err string
}

func (m ErrorMessage) serialize() []byte {
	return []byte{0x10}
}

type HeartbeatMessage struct {}

func (m HeartbeatMessage) serialize() []byte {
	return []byte{0x41}
}

type SpeedServer struct {
	port uint16
	tickQ chan bool
	clients map[uint]*SpeedClient
	numClients uint
}

func NewSpeedServer(port uint16) *SpeedServer {
	s := &SpeedServer{
		port,
		make(chan bool),
		make(map[uint]*SpeedClient),
		0,
	}
	
	return s
}

type SpeedClient struct {
	clientId uint
	con net.Conn
	q chan SpeedMessage
	ctype uint8
	lastHeartbeat int64
	heartbeat int64
}

func NewSpeedClient(clientId uint, con net.Conn) *SpeedClient {
	return &SpeedClient{
		clientId,
		con,
		make(chan SpeedMessage),
		0,
		0,
		0,
	}
}

func (s *SpeedServer) tick() {
	cur := time.Now().UnixMilli()
	m := HeartbeatMessage{}
	for _, c := range s.clients {
		if c.heartbeat == 0 { continue }
	
		if c.lastHeartbeat + c.heartbeat < cur {
			c.q <- m
			c.lastHeartbeat = cur
		}
	}
}

func (s *SpeedServer) ticker() {
	for {
		s.tickQ <- true
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *SpeedServer) main() {
	for {
		select {
			case _ = <- s.tickQ: s.tick()
		}
	}
}

func (s *SpeedServer) Run() {
	go s.listen()
	go s.ticker()
	s.main()
}
	
func (s *SpeedServer) listen() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("SpeedServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      
      s.start(connection)
		}
}

func (s *SpeedServer) start(con net.Conn) {
	c := NewSpeedClient(s.numClients, con)
	s.clients[s.numClients] = c
	s.numClients += 1
	go c.sender()
	go c.receiver()
}

func (c *SpeedClient) sender() {
	for m := range c.q {
	  c.con.Write(m.serialize())
	}
	
	c.con.Close()
	fmt.Printf("Client %d closed\n", c.clientId)
}

func (c *SpeedClient) receiver() {
	var err error
	fmt.Printf("client %d connected\n", c.clientId)
	for {
		var mType uint8
		err = binary.Read(c.con, BE, &mType);
		if err != nil { break }
		switch mType {
			case 0x40: err = c.hdlWantHeartbeat()
			
			default: err = fmt.Errorf("Unknown message type 0x%x", mType)
		}
		if err != nil { break }
	}
	if err != nil {
		fmt.Printf("Client %d error %s\n", c.clientId, err)
	} else {
		fmt.Printf("Client %d closing\n", c.clientId)
	}
	close(c.q)
}

func (c *SpeedClient) hdlWantHeartbeat() error {
	var wantedHb uint32
	err := binary.Read(c.con, BE, &wantedHb)
	if err != nil { return err }
	c.heartbeat = int64(wantedHb) * 100
	fmt.Printf("heartbeat for %d set to %d\n", c.clientId, wantedHb)
	return nil
}


