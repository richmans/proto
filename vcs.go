package main

import (
"net"
"os"
"fmt"
"bufio"
"strings"
"strconv"
"io"
"sort"
"regexp"
)

type VcsNode struct {
  children map[string]*VcsNode
  revisions []string
}

func (n *VcsNode) getRevision(rev string) (string, error) {
  var err error
  
  r := len(n.revisions)
  if rev != "" {
    if rev[0] == 'r' { rev = rev[1:]}
    r, err = strconv.Atoi(rev)
    if err != nil { return "", fmt.Errorf("no such revision")}
  }
  if r <1 || r > len(n.revisions) {
    return "", fmt.Errorf("no such revision")
  }
  return n.revisions[r-1], nil
}

func (n *VcsNode) list() []string {
  keys := make([]string, len(n.children))

  i := 0
  for k := range n.children {
    keys[i] = k
    i++
  }
  sort.Strings(keys)
  return keys
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

func isPrintable(p string) bool {
  for _, c:= range []rune(p) {
    if c > 126 {
      return false
    }
  }
  return true
}
func parsePath(strPath string, isDir bool) ([]string, error) {
  p := strings.Split(strPath, "/")
  itype := "file"
  match, _ := regexp.MatchString(`[^a-zA-Z0-9.\-_/]`, strPath)
  if isDir {
    itype = "dir"
  }
  printable := isPrintable(strPath)
  if p[0] != "" || match || !printable {
    return []string{}, fmt.Errorf("invalid %s name", itype)
  }
  if p[len(p)-1] == "" {
    if !isDir {
      return []string{}, fmt.Errorf("invalid file name")
      }else{
        p = p[:len(p)-1]
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
  if len(f.revisions) < 1 || f.revisions[len(f.revisions)-1] != m.content {
     f.revisions = append(f.revisions, m.content)
  }
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
  if len(args) != 2 { return fmt.Errorf("usage: PUT file length newline data")}
  file := args[0]
  clen, err := strconv.Atoi(args[1])
  if err != nil { clen = 0 }
  buf := make([]byte, clen)
  _, err = io.ReadFull(r, buf)
  if err!= nil { return err }
  content := string(buf)
  if ! isPrintable(content) {
    return fmt.Errorf("text files only")
  }
  rq := NewPutRequest(file, content)
  s.storage.q <- rq
  result := <- rq.callback 
  err = result.err
  if err == nil {
    s.write(fmt.Sprintf("OK r%d\n", result.rev))
  }
  return err
}

func (s *VcsSession) hdlList(args []string) error {
  if len(args) != 1 { return fmt.Errorf("usage: LIST dir")}
  dir := args[0]
  path, err := parsePath(dir, true )
  if err != nil {
    return err
  }
  n, err := s.storage.root.get(path, false)
  if err != nil {
    s.write("OK 0\n")
    return nil
  }
  s.write(fmt.Sprintf("OK %d\n", len(n.children)))
  for _, nm := range n.list() {
    child := n.children[nm]
    meta := "DIR"
    if len(child.revisions) > 0{
      meta = fmt.Sprintf("r%d", len(child.revisions))
    }else{
      nm += "/"
    }
    s.write(fmt.Sprintf("%s %s\n", nm, meta))
  }
  return nil
}

func (s *VcsSession) hdlGet(args []string) error {
  if len(args) < 1 || len(args) > 2{ return fmt.Errorf("usage: GET file [revision]")}
  file := args[0]
  rev := ""
  if len(args) > 1 {
    rev = args[1]
  }
  path, err := parsePath(file, false )
  if err != nil {
    return err
  }
  n, err := s.storage.root.get(path, false) 
  if err != nil { return err }
  data, err := n.getRevision(rev)
  if err != nil { return err}
  s.write(fmt.Sprintf("OK %d\n", len(data)))
  s.write(data)
  return nil
}


func (s *VcsSession) process(cmd string, r *bufio.Reader) error {
  var err error
  parts := strings.Split(cmd, " ")
  if len(parts) == 0 { return fmt.Errorf("illegal method:")}
  method := strings.ToLower(parts[0])
  args := parts[1:]
  switch method {
    case "help": err = s.hdlHelp()
    case "put": err = s.hdlPut(args, r)
    case "list": err = s.hdlList(args)
    case "get": err = s.hdlGet(args)
    default: err = fmt.Errorf("illegal method: %s", method)
  }
  return err
}

func (s *VcsSession) write(m string){
  fmt.Printf("O %s", m)
  s.con.Write([]byte(m))
}

func (s *VcsSession) commandMode(r *bufio.Reader) error {
  s.write("READY\n")
  msg, err := r.ReadString('\n')
  if err != nil { return err }
  fmt.Printf("I %q\n",msg)
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

