# File Transfer Client/Server (Lab 2)

## Team
- **Satyansh** (Implementation)
- **Sufiyan Shaikh** (Partner)

## Overview
A TCP-based file transfer system using **Protocol Buffers** for control-plane messages (metadata) and a **raw byte stream** for file data.

**Control-plane framing:** Each protobuf `Wrapper` is sent as an **8-byte little-endian length prefix** followed by the protobuf bytes.

## Features
- **PUT**: Upload files with MD5 checksum verification
- **GET**: Download files with integrity checking
- Server refuses overwrites and checks disk space
- No directory paths - filenames only
- Optional destination directory for downloads

## Usage

### Server
```bash
make
./bin/server <port> [download-dir]
# Example: ./bin/server 9898 ./storage
```

### Client
```bash
./bin/client <host:port> <put|get> <filename> [dest-dir]

# PUT:
./bin/client localhost:9898 put /path/to/file.txt

# GET:
./bin/client localhost:9898 get file.txt ./downloads
```

## Testing Results

### Performance (localhost)
| File Type | Size | PUT | GET | Status |
|-----------|------|-----|-----|:------:|
| Text | 29B | ~20ms | ~20ms | ✓ |
| Binary | 512KB | 24ms | 25ms | ✓ |
| Large Log | 5MB | ~45ms | ~48ms | ✓ |

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
Agreed upon a single `.proto` with teammate (Sufiyan Shaikh).

This implementation places checksums in request/response metadata (not as separate trailing messages) and uses **MD5** for integrity verification.

### Cross-compatibility
- Both implementations use the same `StorageRequest` and `RetrievalResponse` format
- Checksums sent upfront in control messages (per lab spec)
- Compatible with any client/server following the agreed protocol
