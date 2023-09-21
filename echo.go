package main

import (
"net"
"os"
"fmt"
)

type EchoServer struct {
	port uint16
}

func NewEchoServer(port uint16) *EchoServer {
	return &EchoServer{port}
}

func (s *EchoServer) Run() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("EchoServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      fmt.Println("client connected")
      go s.processClient(connection)
		}
}

func (s *EchoServer) processClient(connection net.Conn) {
	buffer := make([]byte, 1024)
	for {
	  mLen, err := connection.Read(buffer)
	  if err != nil {
	    fmt.Println("Error reading:", err.Error())
			break
	  }
	  fmt.Printf("Received %d bytes\n", mLen)
	  _, err = connection.Write([]byte(buffer[:mLen]))
	}
  connection.Close()
}