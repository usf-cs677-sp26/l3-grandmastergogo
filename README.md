# File Transfer (Lab 2)
TCP-based file transfer system in Go using protobuf-framed messages.
The branch documented here is `lab2-sufiyan`.

## Project overview
There are two executables:
- `server`: listens on a TCP port and stores/serves files from a target directory.
- `client`: connects to server and performs `put` (upload) / `get` (download).

Message framing and serialization are implemented in `messages/message_handler.go` using:
- protobuf payloads
- an 8-byte little-endian length prefix



### Changes made

#### A) End-to-end checksum integrity (core feature)
- Added `checksum` field to:
  - `StorageRequest`
  - `RetrievalResponse`
- Updated `proto/messages.proto` and regenerated `messages/messages.pb.go`.
- Upload (`put`) now computes MD5 on client and sends it with storage request.
- Server verifies uploaded bytes against request checksum.
- Download (`get`) now receives checksum in retrieval response and verifies after write.

Why this change:
- Detects silent corruption in transit or partial/incorrect transfer.
- Guarantees integrity for both upload and download paths.

#### B) Corruption handling improvements
- On checksum mismatch during upload, server removes the stored file.
- On checksum mismatch during download, client removes the downloaded file.

Why this change:
- Prevents keeping corrupted artifacts on disk.

#### C) Disk-space safety on server
- Added pre-upload free-space check (`checkAvailableSpace`) using `syscall.Statfs`.

Why this change:
- Avoids accepting uploads that cannot be fully written.

#### D) Better error handling and resilience
- Avoids process-killing behavior for many runtime errors; returns/logs errors cleanly.
- Propagates send/receive failures in message handler.
- Handles accept-loop errors without crashing server.
- Uses safer file handling (`defer Close`, explicit transfer error checks).

Why this change:
- Keeps server available and makes failures recoverable/debuggable.

#### E) Small but important starter-code behavior changes
- Client uses `filepath.Base(...)` for request file names.
  - reason: avoid directory path injection and comply with file-name-only semantics.
- `get` supports destination directory parameter and writes there.
  - reason: explicit, testable download location control.
- Server ensures target storage directory exists (`os.MkdirAll`) before serving.
  - reason: predictable startup and fewer runtime failures.


## Build instructions
From repository root:

```bash
make clean
make
```

This builds:
- `bin/server`
- `bin/client`

Alternative:

```bash
go build -o bin/server server/server.go
go build -o bin/client client/client.go
```

## Run instructions
Open two terminals.

### Terminal 1: start server
```bash
./bin/server <port> [server_storage_dir]
```

Example:
```bash
./bin/server 9000 ./server_files
```

### Terminal 2: run client
Client usage:
```bash
./bin/client <server_host:port> put|get <file_name> [download_dir]
```

Examples:
- Upload local file to server:
```bash
./bin/client 127.0.0.1:9000 put ./sample.txt
```
- Download file from server into local destination directory:
```bash
./bin/client 127.0.0.1:9000 get sample.txt ./downloads
```

## How to test client/server transfer
### 1) Positive upload test
1. Start server.
2. Create a file locally (e.g., `sample.txt`).
3. Run client `put`.
4. Verify file exists in server storage directory.

### 2) Positive download test
1. Ensure file exists on server side.
2. Run client `get` into `./downloads`.
3. Verify file appears in downloads and checksum logs report match.

### 3) Missing-file download test
1. Request a non-existent file with `get`.
2. Expect clean failure response without server crash.

### 4) Existing-local-file download conflict test
1. Keep a same-name file in destination.
2. Run `get`.
3. Expect client `os.O_EXCL` behavior (fails instead of overwrite).

### 5) Large-file transfer test
1. Upload a larger file.
2. Download it back.
3. Confirm checksum match logs and file integrity.

## Transfer flow details
### PUT (upload)
1. Client stats and opens file.
2. Client computes checksum.
3. Client sends `StorageRequest(file_name, size, checksum)`.
4. Server validates preconditions and replies ready/fail.
5. Client streams bytes to server.
6. Server computes checksum while writing and validates.
7. Server returns final success/failure response.

### GET (download)
1. Client sends `RetrievalRequest(file_name)`.
2. Server opens file, computes checksum, responds with `(ok, message, size, checksum)`.
3. Client streams `size` bytes into destination file while hashing.
4. Client compares computed checksum with server checksum.
5. Client keeps file on match, deletes it on mismatch.

## Notes
- `messages/messages.pb.go` is generated code. If `proto/messages.proto` changes, regenerate protobuf outputs before build.

## Tests (Different branch tests on Orion)
- Both the proto file and function signature are same of me and Satayansh (branched partner) so they are compatibile.
- tested it on orion different machines and different branch server client, it was able to PUT and GET files both binary and text

