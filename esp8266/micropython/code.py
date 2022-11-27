import socket
import gc
import random
import time

p1 = 0
p2 = 0
p3 = 0
def printp():
    global p1
    global p2
    global p3
    p1 = rand(0, 1023)
    p2 = rand(0, 1023)
    p3 = rand(0, 1023)
    par = [p1, p2, p3]
    return par.join('|')

def rand(a, b):
    return random.randint(a, b)

s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(('', 8089))
s.listen(5)

while True:
    if gc.mem_free() < 102000:
        gc.collect()
    try:
        conn, addr = s.accept()
        while True:
            conn.settimeout(None)
            conn.sendall(printp())
            time.sleep(1)
    except OSError as e:
        conn.close()
        print('Connection closed')
