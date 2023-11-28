package main

import (
"net"
"os"
"fmt"
"bufio"
"bytes"
)

const PACKET_HELLO = 0x50
const PACKET_SITE_VISIT = 0x58

type PacketReader struct {
  count uint
  sum byte
  ptype byte
  plen uint
  base *bufio.Reader
  started bool
}
func newPacketReader(r *bufio.Reader) *PacketReader {
  return &PacketReader{
    0,
    0,
    0,
    0,
    r,
    false,
  }
}

func (r *PacketReader) Read(b []byte) (int, error) {
  bytesLeft := r.plen - r.count
  if len(b) > int(bytesLeft) {
    return 0, fmt.Errorf("not enough bytes left in package")
  }
  if ! r.started {
    return 0, fmt.Errorf("read called but not started")
  }
  n, err := r.base.Read(b)
  if err != nil { return 0, err }
  for i := 0; i < n; i++ {
    r.sum += b[i]
  }
  r.count += uint(len(b))
  return n, err
}

func (r *PacketReader) start(ptype byte) error {
  if r.started {
    return fmt.Errorf("already started")
  }
  r.plen = 5
  r.sum = 0
  r.count = 0
  r.started = true
  r.ptype, _ = readU8(r)
  if r.ptype != ptype {
    return fmt.Errorf("unexpexted packet type")
  }
  plen, _ := readU32(r)
  r.plen = uint(plen)
  return nil
}

func (r *PacketReader) finish() error {
  if ! r.started {
    return fmt.Errorf("not started")
  }
  _, err := readU8(r) //checksum
  r.started = false
  if err != nil { return err }
  if r.count != r.plen {
    return fmt.Errorf("packet len mismatch exp %d read %d", r.plen, r.count)
  }
  if r.sum != 0 {
    return fmt.Errorf("checksum error: %d", r.sum)
  }
  
  return nil
}

type PacketWriter struct {
  count uint
  sum byte
  ptype byte
  buf *bytes.Buffer
  base *bufio.Writer
  started bool
}

func newPacketWriter(r *bufio.Writer) *PacketWriter {
  return &PacketWriter{
    0,
    0,
    0,
    nil,
    r,
    false,
  }
}

func (w *PacketWriter) start(ptype byte) error {
  if w.started {
    return fmt.Errorf("already started")
  }
  w.buf = bytes.NewBuffer([]byte{})
  w.count = 0
  w.sum = ptype
  w.started = true
  w.ptype = ptype
  return nil
}

func (w *PacketWriter) finish() error {
  if ! w.started {
    return fmt.Errorf("not started")
  }
  w.started = false
  plen := uint32(w.count + 6)
  c := byte(plen) + byte(plen >> 8) + byte(plen >> 16) + byte(plen >> 24) + w.sum
  c = byte(- c)
  err := writeU8(w.base, w.ptype)
  if err != nil { return err }
  err = writeU32(w.base, plen)
  if err != nil { return err }
  _,err = w.base.Write(w.buf.Bytes())
  if err != nil { return err }
  err = writeU8(w.base, c)
  if err != nil { return err }
  err = w.base.Flush()
  return err
}

func (w *PacketWriter) Write(b []byte) (int, error) {
  if ! w.started {
    return 0, fmt.Errorf("write called but not started")
  }
  n, err := w.buf.Write(b)
  w.count += uint(n)
  for i := 0; i < n; i++ {
    w.sum += b[i]
  }
  return n, err
}


type HelloPacket struct {
  protocol string
  version uint
}

func readHelloPacket(r *PacketReader) (HelloPacket, error) {
  var p HelloPacket
  var err error
  err = r.start(PACKET_HELLO)
  if err != nil { return p, err }
  p.protocol, err = readLString(r)
  if err != nil { return p, err }
  version, err := readU32(r)
  if err != nil { return p, err }
  p.version = uint(version)
  err = r.finish()
  return p, err
}

func writeHelloPacket(w *PacketWriter, p HelloPacket) error {
  err := w.start(PACKET_HELLO)
  if err != nil { return err }
  err = writeLString(w, p.protocol)
  if err != nil { return err }
  err = writeU32(w, uint32(p.version))
  if err != nil { return err }
  err = w.finish()
  return err
}

type Tally struct {
  species string
  count uint
}

type SiteVisitPacket struct {
  siteId uint
  populations []Tally
}

func readSiteVisitPacket(r *PacketReader) (SiteVisitPacket, error) {
  var p SiteVisitPacket
  var err error
  err = r.start(PACKET_SITE_VISIT)
  if err != nil { return p, err }
  siteId, err := readU32(r)
  if err != nil { return p, err }
  p.siteId = uint(siteId)
  talCount, err := readU32(r)
  if err != nil { return p, err }
  for i := 0 ; i < int(talCount) ; i++ {
    var t Tally
    var count uint32
    t.species, err = readLString(r)
    if err != nil { return p, err }
    count, err = readU32(r)
    if err != nil { return p, err }
    t.count = uint(count)
    p.populations = append(p.populations, t)
  }
  err = r.finish()
  return p, err
}

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
  r *PacketReader
  w *PacketWriter
}

func NewPestSession(con net.Conn) *PestSession {
	return &PestSession{
    con,
  newPacketReader(bufio.NewReader(con)),
    newPacketWriter(bufio.NewWriter(con)),
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

func (s *PestSession) readHello() error {
  hello, err := readHelloPacket(s.r)  
  if err != nil { return err }
  fmt.Printf("%+v\n", hello)
  if hello.protocol != "pestcontrol" { return fmt.Errorf("unknown protocol")}
  if hello.version != 1 {
    return fmt.Errorf("unknown version")
  }
  return nil
}

func (s *PestSession) sendHello() error {
  p := HelloPacket{"pestcontrol", 1}
  err := writeHelloPacket(s.w, p)
  return err
}

func (s *PestSession) sendError(err error) error {
  return nil
}

func (s *PestSession) pestHandler() error {
	err := s.readHello()
  if err != nil { return err }
  fmt.Println("got hello, sending back")
  err = s.sendHello()
  var v SiteVisitPacket
  for err == nil {
    v, err = readSiteVisitPacket(s.r)
    if err != nil { return err }
    fmt.Printf("%+v\n", v)
  }
  return err
}

func (s *PestSession) run() {
  err := s.pestHandler()
  fmt.Printf("%e\n", err)
  if err != nil {
    s.sendError(err)
  }
}
