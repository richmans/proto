package main

import (
"net"
"os"
"fmt"
"bufio"
"strings"
"strconv"
)

func largestOrder(msg string) string {
  orders := strings.Split(msg, ",")
  largest := 0
  result := ""
  for i := range orders {
    l := orders[i]
    parts := strings.Split(l, "x")
    amount, err := strconv.Atoi(parts[0])
    if err == nil && amount > largest {
      largest = amount
      result = l
    }
  }
  return result
}

type SecureServer struct {
	port uint16
}

func NewSecureServer(port uint16) *SecureServer {
	return &SecureServer{
		port,
	}
}

type SecureSession struct{
	con net.Conn
}

func NewSecureSession(con net.Conn) *SecureSession {
	return &SecureSession{
		con,
	}
}


func (s *SecureServer) Run() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	server, err := net.Listen("tcp", addr)
    if err != nil {
      fmt.Println("Error listening:", err.Error())
        os.Exit(1)
    }
    defer server.Close()
    fmt.Println("SecureServer waiting for client...")
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

func (s *SecureServer) processClient(con net.Conn) {
	
	session := NewSecureSession( con)
	go session.SecureReceiver()
}


func (s *SecureSession) setEncryption(r *bufio.Reader) error {
	secdef, err := r.ReadBytes('\x00')
	if err != nil { return err }
  fmt.Printf("secdef: %s\n", secdef)
	return nil
}

func (s *SecureSession) query(r *bufio.Reader) error {
	msg, err := r.ReadString('\n')
	if err != nil { return err }
	msg = strings.TrimSpace(msg)
  fmt.Printf("Q: %s", msg)
  a := largestOrder(msg)
  fmt.Printf("A: %s\n", a)
  s.con.Write(append([]byte(a), []byte("\n")...))
	return nil
}

func (s *SecureSession) close() {
	fmt.Printf("close\n")
  s.con.Close()
}

func (s *SecureSession) SecureReceiver() {
	defer s.close()
	r := bufio.NewReaderSize(s.con, 5000)
	err := s.setEncryption(r)
	for err == nil {
		err = s.query(r)
	}
}

