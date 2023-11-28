import socket
from struct import pack, unpack
from time import time, sleep

def sock():
  addr = ("localhost", 13370)
  s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  s.connect(addr)
  return s

def recv(s):
  ptype, plen = unpack(">BI", s.recv(5))
  data = s.recv(plen-6)
  cs = s.recv(1)
  print(f"I {ptype:X} {data}")
  
def pstr(s):
  if type(s) == str:
    s = s.encode()
  return pack(">I", len(s)) + s

def snd(s, typ, dat):
  p = pack(">BI", typ, len(dat)+6) + dat
  csum = (-sum(p)) % 256
  c = bytes([ csum ])
  s.sendall(p + c)
  
def hello(s):
  h = pstr("pestcontrol")
  h += pack(">I", 1)
  snd(s, 0x50, h)

def sivi(s):
  h = pack(">II", 1337, 3)
  h += pstr("green starred rat")
  h += pack(">I", 765)
  h += pstr("red footed elephant")
  h += pack(">I", 6029)
  h += pstr("black tailed unicorn")
  h += pack(">I", 1234)
  snd(s, 0x58, h)
  
s = sock()
hello(s)
recv(s)
sivi(s)