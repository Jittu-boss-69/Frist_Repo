import os
import sys
import time
import argparse
import requests
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

class DevSyncHandler(FileSystemEventHandler):
    def __init__(self, host, device_name):
        self.host = host
        self.device_name = device_name

    def on_created(self, event):
        if event.is_directory:
            return
        
        filepath = event.src_path
        filename = os.path.basename(filepath)
        
        # Skip temporary files/system metadata
        if filename.startswith('.') or filename.endswith('.tmp') or filename.startswith('~'):
            return

        print(f"[Watcher] New file detected: {filename}. Uploading to DevSync...")
        
        # Wait a moment for the write stream to finish
        time.sleep(1.0)
        
        try:
            url = f"{self.host.rstrip('/')}/api/files/upload"
            with open(filepath, 'rb') as f:
                files = {'file': (filename, f)}
                data = {'sender_device': self.device_name}
                response = requests.post(url, files=files, data=data)
                
            if response.status_code == 200:
                print(f"[Watcher] SUCCESS: {filename} uploaded successfully!")
            else:
                print(f"[Watcher] FAILED: {filename} (Status {response.status_code}): {response.text}")
        except Exception as e:
            print(f"[Watcher] ERROR uploading {filename}: {e}")

def main():
    parser = argparse.ArgumentParser(description="DevSync Folder Watcher Utility")
    parser.add_argument("--host", default="http://localhost:8080", help="DevSync server address (e.g. http://192.168.1.15:8080)")
    parser.add_argument("--name", default="PythonWatcher", help="Custom name for this sender device")
    parser.add_argument("--folder", default="./DevSync_Folder", help="Path to local folder to watch")
    args = parser.parse_args()

    watch_dir = os.path.abspath(args.folder)
    
    # Create the watched folder if it doesn't exist
    if not os.path.exists(watch_dir):
        os.makedirs(watch_dir)
        print(f"[Watcher] Created folder: {watch_dir}")

    print("==================================================")
    print("      DevSync Folder Watcher Utility Active       ")
    print("==================================================")
    print(f" Watching Folder : {watch_dir}")
    print(f" Target Host     : {args.host}")
    print(f" Device Name     : {args.name}")
    print(" Press Ctrl+C to terminate...")
    print("==================================================")

    event_handler = DevSyncHandler(args.host, args.name)
    observer = Observer()
    observer.schedule(event_handler, path=watch_dir, recursive=False)
    observer.start()

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("\n[Watcher] Terminating folder watcher...")
        observer.stop()
    observer.join()

if __name__ == "__main__":
    main()
