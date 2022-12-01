from machine import ADC, Pin
import socket
import gc
import random
import time

WIFI_SSID = 'ssid'
WIFI_PASS = 'key'

def do_connect():
    import network
    wlan = network.WLAN(network.STA_IF)
    wlan.active(True)
    if not wlan.isconnected():
        print(f"connecting to network {WIFI_SSID}...")
        wlan.connect(WIFI_SSID, WIFI_PASS)
        while not wlan.isconnected(): pass
    print('network config:', wlan.ifconfig())

adc = ADC(0)
dirty = False

POT_PINS = [Pin(21, Pin.OUT), Pin(3, Pin.OUT), Pin(2, Pin.OUT)]
prev_values = [0 for x in range(len(POT_PINS))]

def printp():
    return "{}\r\n".format("|".join([str(x) for x in prev_values])).encode()

s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(('', 8089))
s.listen(5)

def analog_read(pot):
    POT_PINS[pot].value(1)
    time.sleep_us(10)
    val = adc.read()
    POT_PINS[pot].value(0)
    return val

def read_values():
    global dirty
    dirty = False
    new_vals = [analog_read(x) for x in range(len(POT_PINS))]
    if any([new_vals[x] != prev_values[x] for x in range(len(POT_PINS))]):
        for x in range(len(POT_PINS)):
            prev_values[x] = new_vals[x]
        dirty = True

def rand(a, b):
    return random.randint(a, b)

def on_update(conn):
    read_values()
    if dirty:
        conn.send(printp())

do_connect()

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
