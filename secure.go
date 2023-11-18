package main

import (
"net"
"os"
"fmt"
"bufio"
"strings"
"bytes"
"strconv"
"math/bits"
)

type Op struct {
  op byte
  arg byte
}

func parseSpec(c []byte) ([]Op, error) {
  r := make([]Op, 0, 5)
  i := 0
  for i < len(c) {
    last := i == len(c) - 1
    opid := c[i]
    arg := byte(0)
    if opid == 0 { return r, nil}
    if opid == 2 || opid == 4 {
      if last { return r, fmt.Errorf("missing byte in cipherspec") }
        i += 1
        arg = c[i]
    }
    op := Op{opid, arg}
    r = append(r, op)
    i += 1
  }
  return r, nil
}

func crypt(c []Op, b byte, o int, enc bool) (byte, error) {
  i := 0
  for i < len(c) {
    x := i
    if ! enc {
      x = len(c) - i -1
    }
    oper := c[x]

    switch oper.op {
      case 0: return b, nil
      case 1:
        b = bits.Reverse8(b)
      case 2:
        b = b ^ oper.arg
      case 3:
        b = b ^ byte(o)
      case 4:
        if enc {
          b = b + oper.arg
        } else {
          b = b - oper.arg
        }
      case 5:
        if enc {
          b = b + byte(o)
        } else {
          b = b - byte(o)
        }
      default:
        return 0, fmt.Errorf("invalid cipherspec %d", oper.op)
    }
    i += 1
  }
  return b, nil
}

func readEncryptedLine(r *bufio.Reader, c []Op, offset int) (string, error) {
  off := 0
  buf := make([]byte, 0, 32)
  for {
    b, err := r.ReadByte()
    if err != nil {
      return "", fmt.Errorf("read error")
    }
    b, err = crypt(c, b, off+offset, false)
    if err != nil {
      return "", err
    }
    if b == 10 { break }
    buf = append(buf, b)
    off += 1
  } 
  return string(buf), nil
}

func cryptBytes(c []Op, dat []byte, offset int, encrypt bool) ([]byte, error) {
  buf := make([]byte, 0, 32)
  for off := range dat {
    b, err := crypt(c, dat[off], off+offset, encrypt)
    if err != nil {
      return []byte(""), err
    }
    buf = append(buf, b)
  }
  return buf, nil
}

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
  cipspec []Op
  sent int
  recv int
}

func NewSecureSession(con net.Conn) *SecureSession {
	return &SecureSession{
		con,
    make([]Op, 0),
    0,
    0,
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


func (s *SecureSession) setCipher(r *bufio.Reader) error {
	cipspecStr, err := r.ReadBytes('\x00')
	if err != nil { return err }
  fmt.Printf("cipspec: %x\n", cipspecStr)
  cipspec, err := parseSpec(cipspecStr)
  if err != nil { return err }
  s.cipspec = cipspec
  dummy := []byte("abcd")
  result, err := cryptBytes(cipspec, dummy, 0, true)
  if err != nil { return err }
  if bytes.Equal(result, dummy) {
    return fmt.Errorf("transparent cipspec")
  }
  decrypt, err := cryptBytes(cipspec, result, 0, false)
  if err != nil { return err }
  if ! bytes.Equal(dummy, decrypt) {
    return fmt.Errorf("Sanity decrypt failed: %s", decrypt)
  }
  fmt.Println("Spec approved")
	return nil
}

func (s *SecureSession) query(r *bufio.Reader) error {
  fmt.Printf("Recv %d sent %d \n", s.recv, s.sent)
	msg, err := readEncryptedLine(r, s.cipspec, s.recv)
  s.recv += len(msg) + 1
	if err != nil { return err }
	msg = strings.TrimSpace(msg)
  fmt.Printf("Q: %s\n", msg)
  a := largestOrder(msg)
  fmt.Printf("A: %s\n", a)
  resp := append([]byte(a), []byte("\n")...)
  resp, err = cryptBytes(s.cipspec, resp, s.sent, true)
  s.sent += len(resp)
  if err != nil { return err }
  s.con.Write(resp)
	return nil
}

func (s *SecureSession) close() {
	fmt.Printf("close\n")
  s.con.Close()
}

func (s *SecureSession) SecureReceiver() {
	defer s.close()
	r := bufio.NewReaderSize(s.con, 5000)
	err := s.setCipher(r)
	for err == nil {
		err = s.query(r)
	}
  fmt.Printf("%e\n", err)
}

