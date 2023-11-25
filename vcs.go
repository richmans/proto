package main

import (
"net"
"os"
"fmt"
"bufio"
"strings"
"strconv"
"io"
)

type VcsNode struct {
  children map[string]*VcsNode
  revisions []string
}

type PutRequest struct {
  file string
  content string
  callback chan PutResult
}

type PutResult struct {
  rev uint
  err error
}

type VcsStorage struct {
  q chan *PutRequest
  root *VcsNode
}

type VcsServer struct {
	port uint16
  storage VcsStorage
}
func NewPutRequest(file string, content string) *PutRequest {
  return &PutRequest{
    file,
    content,
    make(chan PutResult),
  }
}

func NewVcsNode() *VcsNode {
  return &VcsNode {
        make(map[string]*VcsNode),
        nil,
      }
}
func NewVcsServer(port uint16) *VcsServer {
	return &VcsServer{
		port,
    VcsStorage{
      make(chan *PutRequest),
      NewVcsNode(),
    },
	}
}

type VcsSession struct{
	storage *VcsStorage
	con net.Conn
}

func NewVcsSession(storage *VcsStorage, con net.Conn) *VcsSession {
	return &VcsSession{
		storage,
    con,
	}
}

func (n *VcsNode) get(path []string, create bool) (*VcsNode, error) {
  if len(path) == 0 { return n, nil }
  part := path[0]
  nx, ok := n.children[part]
  if ! ok {
    if create {
      nx = NewVcsNode()
      n.children[part] = nx
    }else{
      return nil, fmt.Errorf("no such file")
    }
  }
  return nx.get(path[1:], create)
}

func parsePath(strPath string, isDir bool) ([]string, error) {
  p := strings.Split(strPath, "/")
  fmt.Println(p)
  itype := "file"
  if isDir {
    itype = "dir"
  }
  if p[0] != "" {
    return []string{}, fmt.Errorf("invalid %s name", itype)
  }
  if p[len(p)-1] == "" {
    if !isDir {
      return []string{}, fmt.Errorf("invalid file name")
      }else{
        p = p[:len(p)-2]
      }
  }
  p = p[1:]
  return p, nil
}

func (s *VcsStorage) hdlPut(m *PutRequest) (uint, error) {
  path, err := parsePath(m.file, false )
  if err != nil {
    return 0, err
  }
  f, err := s.root.get(path, true)
  if err != nil {
    return 0, err
  }
  f.revisions = append(f.revisions, m.content)
  return uint(len(f.revisions)), nil
  
}

func (s *VcsStorage) central(){
	for m := range s.q {
		rev, err := s.hdlPut(m)
    m.callback <- PutResult{rev, err}
	}
}



func (s *VcsServer) Run() {
	go s.storage.central()
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
}

func (s *VcsServer) processClient(con net.Conn) {
	session := NewVcsSession(&s.storage, con)
	go session.vcsHandler()
}

func (s *VcsSession) hdlHelp() error {
  s.write("OK usage: HELP|GET|PUT|LIST\n")
  return nil
}



func (s *VcsSession) hdlPut(args []string, r *bufio.Reader) error {
  file := args[0]
  clen, err := strconv.Atoi(args[1])
  if err != nil { clen = 0 }
  buf := make([]byte, clen)
  _, err = io.ReadFull(r, buf)
  if err!= nil { return err }
  rq := NewPutRequest(file, string(buf))
  s.storage.q <- rq
  result := <- rq.callback 
  err = result.err
  if err == nil {
    s.write(fmt.Sprintf("OK r%d\n", result.rev))
  }
  return err
}

func (s *VcsSession) process(cmd string, r *bufio.Reader) error {
  fmt.Println(cmd)
  var err error
  parts := strings.Split(cmd, " ")
  if len(parts) == 0 { return fmt.Errorf("illegal method:")}
  method := strings.ToLower(parts[0])
  switch method {
    case "help": err = s.hdlHelp()
    case "put": err = s.hdlPut(parts[1:], r)
    default: err = fmt.Errorf("illegal method: %s", method)
  }
  return err
}

func (s *VcsSession) write(m string){
  s.con.Write([]byte(m))
}

func (s *VcsSession) commandMode(r *bufio.Reader) error {
  s.write("READY\n")
  msg, err := r.ReadString('\n')
  if err != nil { return err }
  msg = strings.TrimSpace(msg)
  perr := s.process(msg, r)
  if perr != nil {
    s.write(fmt.Sprintf("ERR %s\n", perr.Error()))
  }
	return nil
}

func (s *VcsSession) vcsHandler() {
	r := bufio.NewReaderSize(s.con, 102400)
	var err error
	for err == nil {
    err = s.commandMode(r)
	}
}

