package main

import (
"net"
"os"
"fmt"
"bufio"
"regexp"
)

type ProxyServer struct {
	port uint16
}

func NewProxyServer(port uint16) *ProxyServer {
	return &ProxyServer{port}
}

func (s *ProxyServer) Run() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("ProxyServer waiting for client...")
    for {
      connection, err := server.Accept()
      if err != nil {
        fmt.Println("Error accepting: ", err.Error())
        os.Exit(1)
      }
      fmt.Println("client connected")
      s.start(connection)
		}
}

func (s *ProxyServer) start(down net.Conn) {
	addr := "chat.protohackers.com:16963"
	up, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	go s.stream(up, down)
	go s.stream(down, up)
}

func (s *ProxyServer) stream(input net.Conn, output net.Conn) {
	r := bufio.NewReader(input)
	var err error
	for err == nil {
	  msg, err := r.ReadString('\n')
	  if err != nil { break }
		msg = s.transform(msg)
		_, err = output.Write([]byte(msg))
	}
	input.Close()
	output.Close()
}

func (s *ProxyServer) transform(input string) string {
	fmt.Printf(">> %s", input)
	tony := "${1}7YWHMfk9JZe0LM0g1ZauHuiSxhI${3}"
	r := regexp.MustCompile(`(^|\s)(7[a-zA-Z0-9]{25,34})($|\s)`)
	output := r.ReplaceAllString(input, tony)
	fmt.Printf("<< %s", output)
	return output
}