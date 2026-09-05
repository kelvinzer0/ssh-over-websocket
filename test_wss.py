import asyncio
import websockets
import urllib.parse
import time

async def test_wss():
    host = "127.0.0.1"
    port = "22"
    user = "root"
    password = "@Kelvin123"
    
    url = f"wss://gatewayssh.warunglakku.com/ssh?host={urllib.parse.quote(host)}&port={port}&user={urllib.parse.quote(user)}&pass={urllib.parse.quote(password)}"
    
    print(f"Connecting to {url} ...")
    try:
        async with websockets.connect(url) as ws:
            print("Connected! Waiting to see if it closes...")
            
            # Send a basic command
            await ws.send("ls -la\r")
            
            # Listen for messages in background, send keep-alive in foreground
            async def listen():
                start_time = time.time()
                try:
                    while True:
                        msg = await asyncio.wait_for(ws.recv(), timeout=120.0)
                        print(f"[{time.time() - start_time:.1f}s] Received: {repr(msg)}")
                except asyncio.TimeoutError:
                    print("Timeout reached.")
                except websockets.exceptions.ConnectionClosed as e:
                    print(f"Connection closed at {time.time() - start_time:.1f}s! Reason: {e}")
                    raise e
                    
            listen_task = asyncio.create_task(listen())
            
            try:
                for i in range(10):
                    await asyncio.sleep(2)
                    await ws.ping() # Send standard WebSocket ping
                await listen_task
            except websockets.exceptions.ConnectionClosed:
                pass

    except Exception as e:
        print(f"Failed to connect or error: {e}")

if __name__ == "__main__":
    asyncio.run(test_wss())
