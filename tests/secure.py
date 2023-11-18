import socket
from struct import pack, unpack
from time import time, sleep
from binascii import unhexlify

def sock():
  addr = ("localhost", 13370)
  s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  s.connect(addr)
  return s
  
def test(s):
  s.sendall(b"\x04\x01\x0010x snoeperfloep,123x sapperflap\x11")
  print(s.recv(1024))
  s.close()

def insane(s): s.sendall(unhexlify("03050105050500"))

insane(sock())