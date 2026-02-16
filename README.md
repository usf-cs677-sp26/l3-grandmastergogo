# File Transfer Client/Server (Lab 2)

## Team
- **Satyansh** (Implementation)
- **Sufiyan Shaikh** (Partner)

## Implementation
A TCP-based file transfer system using Protocol Buffers for control messages and raw byte streams for file data.

### Features
- **PUT**: Upload files with MD5 checksum verification
- **GET**: Download files with integrity checking
- Server refuses overwrites and checks disk space
- No directory paths - filenames only
- Optional destination directory for downloads

## Usage

### Server
```bash
make
./bin/server <port> [storage-dir]
# Example: ./bin/server 9898 ./storage
```

### Client
```bash
./bin/client <host:port> <put|get> <filename> [dest-dir]
# PUT: ./bin/client localhost:9898 put file.txt
# GET: ./bin/client localhost:9898 get file.txt ./downloads
```

## Testing Results

### Performance (localhost)
| File Type | Size | PUT | GET | Status |
|-----------|------|-----|-----|--------|
| Text | 29B | ~20ms | ~20ms | ✓ |
| Binary | 512KB | 24ms | 25ms | ✓ |

### Validation
- ✓ Checksum verification (MD5)
- ✓ Overwrite protection
- ✓ Non-existent file handling
- ✓ Path basename extraction
- ✓ Concurrent client support

## Changes from Starter Code
1. **Protocol**: Added `checksum` field to `StorageRequest` and `RetrievalResponse`
2. **Client**: Calculate checksums upfront, send only basenames, use destination-dir
3. **Server**: Added disk space check, directory creation, path validation
4. **Message Handler**: Fixed type safety issues (uint64→int), used `io.ReadFull`

## Compatibility
Implementation follows lab spec for checksum placement in request/response metadata (not as separate trailing messages).
