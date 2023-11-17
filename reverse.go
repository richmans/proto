package main

import (
"net"
"fmt"
"os"
"bufio"
"bytes"
"time"
)

var startTime int64

func ts() int64 {
  if startTime == 0 {
    startTime = time.Now().Unix()
  }
  return time.Now().Unix() - startTime
}
const MAXPACKET = 999

func b(s string) []byte {
  return []byte(s)
}

func Min(a uint32, b uint32) uint32 {
  if a < b { return a}
  return b
}

func Max(a uint32, b uint32) uint32 {
  if a > b { return a}
  return b
}

func b2i(s []byte) (uint32, error) {
  var num uint32
  _, err := fmt.Sscanf(string(s), "%d", &num)
  return num, err
}

func reverse(s []byte) []byte{
  for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
    s[i], s[j] = s[j], s[i]
  }
  return s
}

func splitSlash(s []byte) ([]byte, []byte) {
  parts := bytes.SplitAfterN(s, b("/"), 2)
  if len(parts) != 2 {
    return b(""),b("")
  }
  p := parts[0]
  return p[:len(p)-1], parts[1]
}

func calcMaxEscaped(b []byte, limit uint32) uint32 {
  bLen := len(b)
  cnt := 0
  pos := 0
  for cnt + pos < int(limit) && pos < bLen {
     if b[pos] == '/' || b[pos] =='\\' { 
       cnt += 1
     }
     pos += 1
    
  }
  return uint32(pos)
}

func escape(s []byte) []byte {
  s = bytes.Replace(s, b("\\"), b("\\\\"), -1)
  return bytes.Replace(s, b("/"), b("\\/"), -1)
}

func unescape(s []byte) []byte {
  s = bytes.Replace(s, b("\\/"), b("/"), -1)
  return bytes.Replace(s, b("\\\\"), b("\\"), -1)
}

func checkscape(s []byte) bool {
  i := 0
  for i < len(s) - 1 {
    if s[i] == '/' { 
      return false
    }
    if s[i] == '\\' {
      i += 1 
    }
    i += 1
  }
  return true
}

type ReverseServer struct {
	port uint16
  
}

func NewReverseServer(port uint16) *ReverseServer {
	return &ReverseServer{
		port,
	}
}

func (s *ReverseServer) Run() {
  sock := NewLrcpServer(s.port)
  for {
    ses := sock.Accept()
    go s.handleSession(ses)
  }
}

func (s *ReverseServer) handleSession(ses *LrcpSession) {
  r := bufio.NewReaderSize(ses, 1024*1024)
  for {
    data, err := r.ReadBytes('\n')
    if err != nil { return }
    data = reverse(data[:len(data)-1])
    ses.Write(append(data, '\n'))
  }
}

type ReceivedData struct {
  data []byte
  pos uint32
}

type LrcpSession struct {
  sessionId uint32
  ack uint32
  seq uint32
  acked uint32
  readPtr uint32
  readBuf uint32
  pc net.PacketConn
  addr net.Addr
  recv map[uint32]*ReceivedData
  dataQ chan uint32 
  ackQ chan uint32
  closed bool
  last int64
}

func NewLrcpSession(sessionId uint32, pc net.PacketConn, addr net.Addr) *LrcpSession {
  return &LrcpSession{
    sessionId,
    0,
    0,
    0,
    0,
    0,
    pc,
    addr,
    make(map[uint32]*ReceivedData),
    make(chan uint32, 100),
    make(chan uint32, 100),
    false,
    0,
  }
}

func (s *LrcpSession) snd(buf []byte) error {
  var err error
  fmt.Printf("> %d %s\n", ts(), bytes.Replace(buf, b("\n"), b("\\n"), -1))
  _, err = s.pc.WriteTo(buf, s.addr) 
  return err
  
}
func (s *LrcpSession) Read(buf []byte) (int, error) {
  //fmt.Printf("reading len %d\n", len(buf))
  for s.readPtr >= s.ack {
    fmt.Printf("ack %d\n", s.ack)
    select {
      case <-s.dataQ: 
      case <-time.After(60 * time.Second): 
        s.sendClose()
        return 0, fmt.Errorf("timeout")
    }
  }
  //fmt.Println("read has data")
  r, ok := s.recv[s.readBuf]
  if ! ok {
    return 0, fmt.Errorf("recv buf %d not found", s.readBuf)
  }
  b := r.data
  posInBuf := s.readPtr - s.readBuf
  bytesLeft := uint32(len(b)) - posInBuf
  l := Min(uint32(len(buf)), bytesLeft)
  //fmt.Printf("Read returning %d bytes\n", l)
  copy(buf, b[posInBuf:posInBuf+l])
  bytesLeft -= l
  s.readPtr += l
  if bytesLeft == 0 {
    s.readBuf += uint32(len(b))
  } 
  return int(l), nil
}

func (s *LrcpSession) waitAck() bool {
  acked := false
  
  for ! acked {
    select {
       case <- s.ackQ: acked = s.seq <= s.acked
       case <-time.After(1 * time.Second): return false
     }
  }
  
  return acked
}

func (s *LrcpSession) WriteString(b string) (int, error) {
  return s.Write([]byte(b))
}

func (s *LrcpSession) Write(b []byte) (int, error) {
  bytesLeft := uint32(len(b))
  pos := uint32(0)
  for bytesLeft > 0 {
    // max packed len is 1k
    // 5 slashes + "data" + 2 decimal ints =~ 23, call it 30
    if s.closed {
      return int(pos), fmt.Errorf("closed")
    }
    l := calcMaxEscaped(b[pos:pos+bytesLeft], MAXPACKET-30)
    err := s.writeBlock(b[pos:pos+l])
    if err != nil { return int(pos), err}
    pos += l
    bytesLeft -= l
  }
  return int(pos), nil
}
  
func (s *LrcpSession) writeBlock(b []byte)  error {
  bLen := uint32(len(b))
  b = escape(b)
  hdr := fmt.Sprintf("/data/%d/%d/", s.sessionId, s.seq)
  
  packet := append([]byte(hdr), b...)
  packet = append(packet, '/')
  s.seq += bLen
  acked := false
  var err error
  retries := 30
  for ! acked && retries > 0{
    if s.closed {
      return fmt.Errorf("closed")
    }
    //fmt.Printf("waiting for seq %d acked %d\n", s.seq, s.acked)
    err = s.snd(packet)
    if err != nil { return err }
    acked = s.waitAck()
    retries -= 1
  }
  return err
}

func (s *LrcpSession) sendAck() {
  m := fmt.Sprintf("/ack/%d/%d/", s.sessionId, s.ack)
  s.snd([]byte(m))
}

func (s *LrcpSession) receiveData( pos uint32, data []byte ){
  // todo if pos in recv but len  bigger, make a separate entry with higher pos and smalle data
  //fmt.Printf("receiving %d at %d\n", len(data), pos)
  //fmt.Printf("%+v\n", s.recv)
  d, ok := s.recv[pos]
  for ok {
    dlen := len(d.data)
    if dlen >= len(data) { 
      //fmt.Printf("found %d bytes at %d which is more than %d so bailing out\n", dlen, pos, len(data))
      s.sendAck()
      return
    }
    pos += uint32(dlen)
    data = data[dlen:]
    //fmt.Printf("found %d bytes at pos, remaining %d at %d\n", dlen, len(data), pos)
    d, ok = s.recv[pos]
  }
  r := &ReceivedData{data, pos}
  s.recv[pos] = r
  s.processNewData()
}

func (s *LrcpSession) processNewData() {
  r, ok := s.recv[s.ack]
  newData := ok
  
  for ok {
    s.ack = r.pos + uint32(len(r.data))
    r, ok = s.recv[s.ack]
  }
  s.sendAck()
  if newData {
    s.dataQ <- s.ack
  }
}

func (s *LrcpSession) close() {
  s.closed = true
}

func (s *LrcpSession) sendClose() {
  m := fmt.Sprintf("/close/%d/", s.sessionId)
  s.snd([]byte(m))
}


type LrcpServer struct {
	port uint16
  pc net.PacketConn
  sessions map[uint32]*LrcpSession
  acceptQ chan *LrcpSession
}

func NewLrcpServer(port uint16) *LrcpServer {
	s := &LrcpServer{
		port,
    nil,
    make(map[uint32]*LrcpSession),
    make(chan *LrcpSession, 100),
	}
  go s.run()
  return s
}

func (s *LrcpServer) run() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
  var err error
	s.pc, err = net.ListenPacket("udp", addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer s.pc.Close()
	buf := make([]byte, 1000)
	fmt.Println("Reverse server listening for packets")
	for {
		n, addr, err := s.pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		s.handle(addr, buf[:n])
	}	
}

func (s *LrcpServer) Accept() *LrcpSession {
  ses := <- s.acceptQ
  //fmt.Printf("Started session %d\n", ses.sessionId)
  ses.sendAck()
  return ses
}

func (s *LrcpServer) handle(addr net.Addr, req []byte) {
  fmt.Printf("< %d %s\n", ts(), bytes.Replace(req, b("\n"), []byte("\\n"), -1))
  if ! bytes.HasPrefix(req, b("/")) || ! bytes.HasSuffix(req, b("/")) {
    return
  }
  _, req = splitSlash(req)
  cmd, args := splitSlash(req)
  var err error
  strSessionId, args := splitSlash(args)
  sessionId, err := b2i(strSessionId)
  if err != nil { return }
  switch string(cmd) {
    case "connect": err = s.hdlConnect(sessionId, addr)
    case "data": err = s.hdlData(sessionId, args, addr)
    case "ack": err = s.hdlAck(sessionId, args)
    case "close": err = s.hdlClose(sessionId)
    default: err = fmt.Errorf("unimplemented command %s\n", cmd)
  }
  if err != nil {
    fmt.Println(err)
    
  }
}

func (s *LrcpServer) hdlConnect(sessionId uint32, addr net.Addr) error {
  
  ses, ok := s.sessions[sessionId]
  if ok && ! ses.closed {
    ses.sendAck()
    return nil
  }
  ses = NewLrcpSession(sessionId, s.pc, addr)
  ses.last = time.Now().Unix()
  s.sessions[sessionId] = ses
  s.acceptQ <- ses
  return nil
}


func (s *LrcpServer) hdlData(sessionId uint32, req []byte, addr net.Addr) error {
  ses, ok := s.sessions[sessionId]
  if !ok { 
    s.sendClose(sessionId, addr)
    return fmt.Errorf("unknown session %d", sessionId)}
  ses.last = time.Now().Unix()
  strPos, req := splitSlash(req)
  pos, err:= b2i(strPos)
  if err != nil { return err }
  
  data := req[:len(req)-1]
  if !checkscape(data) {
    return fmt.Errorf("invalid data")
  }
  data = unescape(data)
  //fmt.Printf("Data len %d for session %d pos %d\n", len(data), sessionId, pos)
  if len(data) == 0 {
    ses.sendAck()
  } else {
    ses.receiveData(pos, data)
  }
  return nil
}

func (s *LrcpServer) hdlAck(sessionId uint32, req []byte) error {
  ses, ok := s.sessions[sessionId]
  if !ok { return fmt.Errorf("unknown session %d", sessionId)}
  strPos, req := splitSlash(req)
  pos, err:= b2i(strPos)
  if err != nil { return err }
  if pos > ses.seq {
    return s.hdlClose(sessionId)
  }
  ses.acked = Max(ses.acked, pos)
  ses.ackQ <- pos
  return nil
}

func (s *LrcpServer) hdlClose(sessionId uint32) error {
  ses, ok := s.sessions[sessionId]
  if !ok { return fmt.Errorf("unknown session %d", sessionId)}
  s.sendClose(ses.sessionId, ses.addr)
  ses.close()
  return nil
}

func (s *LrcpServer) sendClose(sessionId uint32, addr net.Addr) {
  m := fmt.Sprintf("/close/%d/", sessionId)
  fmt.Printf("> %s\n", m)
  
  s.pc.WriteTo([]byte(m), addr)
}
