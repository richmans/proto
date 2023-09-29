import socket
from struct import pack, unpack
from time import time, sleep

def sock():
  addr = ("localhost", 13370)
  s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  s.connect(addr)
  return s
  
def heartbeat(s):
  s.sendall(pack(">BI", 0x40, 2))
  beat1 = s.recv(1024)
  start = time()
  beat2 = s.recv(1024)
  dur = time() - start
  print(f"heartbeat {dur:.2f}")

def camera(s):
  s.sendall(pack(">BHHH", 0x80,1,0,80))
  s.sendall(b"\x20\x05snack" +pack(">I", 100000))

def camera2(s):
  s.sendall(pack(">BHHH", 0x80,1,85,80))
  s.sendall(b"\x20\x05snack" +pack(">I", 103600))
  
def dispatch(s):
  s.sendall(pack(">BBHH", 0x81,2,2,1))
  return s

def readTicket(d):
  l = unpack("BB", d.recv(2))[1]
  plt = d.recv(l)
  dt = unpack(">HHIHIH", d.recv(16))
  print(f"Ticket {plt} speed {dt[5]}")


heartbeat(sock())
camera(sock())
camera2(sock())
sleep(1)
dsp = dispatch(sock())
readTicket(dsp)
