package main

import (
"net"
"os"
"fmt"
)


type PestServer struct {
	port uint16
}

func NewPestServer(port uint16) *PestServer {
	return &PestServer{
		port,
	}
}

type PestSession struct{
	con net.Conn
}

func NewPestSession(con net.Conn) *PestSession {
	return &PestSession{
    con,
	}
}

func (s *PestServer) Run() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("PestServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      fmt.Println("client connected")
      s.processClient(connection)
		}
}

func (s *PestServer) processClient(con net.Conn) {
	session := NewPestSession(con)
	go session.pestHandler()
}

func (s *PestSession) pestHandler() {
	
}

