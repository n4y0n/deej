import socket
import gc
import random
import time

p1 = 0
p2 = 0
p3 = 0
dirty = False

def printp():
    return "{}|{}|{}\r\n".format(p1, p2, p3).encode()

s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(('', 8089))
s.listen(5)

def read_values():
    global p1, p2, p3, dirty
    dirty = False
    new_p1 = rand(0, 1023)
    new_p2 = rand(0, 1023)
    new_p3 = rand(0, 1023)
    if new_p1 != p1 or new_p2 != p2 or new_p3 != p3:
        p1 = new_p1
        p2 = new_p2
        p3 = new_p3
        dirty = True

def rand(a, b):
    return random.randint(a, b)

def on_update(conn):
    read_values()
    if dirty:
        conn.send(printp())

while True:
    if gc.mem_free() < 102000:
        gc.collect()
    try:
        conn, addr = s.accept()
        conn.settimeout(None)
        print('Got a new connection from {}'.format(str(addr)))
        while True:
            on_update(conn)
            time.sleep(0.01)
    except OSError as e:
        conn.close()
        print('Connection closed')
    except Exception as e:
        print('Error: {}'.format(e))
