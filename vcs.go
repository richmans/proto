package main

import (
"net"
"os"
"fmt"
"bufio"
)

type PutRequest struct {
  file string
  content string
  callback chan bool
}

type VcsServer struct {
	port uint16
	q chan PutRequest
}

func NewVcsServer(port uint16) *VcsServer {
	return &VcsServer{
		port,
		make(chan PutRequest),
	}
}

type VcsSession struct{
	server *VcsServer
	con net.Conn
}

func NewVcsSession(server *VcsServer, con net.Conn) *VcsSession {
	return &VcsSession{
		server,
    con,
	}
}



func (s *VcsServer) central(){
	for m := range s.q {
		fmt.Printf("I %+v\n", m)
	}
}


func (s *VcsServer) Run() {
	go s.central()
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("VcsServer waiting for client...")
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

func (s *VcsServer) processClient(con net.Conn) {
	session := NewVcsSession(s, con)
	go session.vcsHandler()
}


func (s *VcsSession) vcsHandler() {
	r := bufio.NewReaderSize(s.con, 102400)
	msg, err := r.ReadString('\n')
  
	for err == nil {
    fmt.Print(msg)
	  msg, err = r.ReadString('\n')
	}
}

