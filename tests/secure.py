import socket
from struct import pack, unpack
from time import time, sleep

def sock():
  addr = ("localhost", 13370)
  s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  s.connect(addr)
  return s
  
def test(s):
  s.sendall(b"\x0010x snoeperfloep,123x sapperflap\n")
  print(s.recv(1024))
  s.close()

test(sock())