package main

import (
	"crypto/md5"
	"file-transfer/messages"
	"file-transfer/util"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"syscall"
)

// check disk space before accepting an upload (avoids writing a partial file when the disk is full).
func checkAvailableSpace(requiredBytes uint64) bool {
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(".", &statfs); err != nil {
		log.Printf("disk space check skipped: %v", err)
		return true
	}
	available := statfs.Bavail * uint64(statfs.Bsize)
	return available >= requiredBytes
}

func handleStorage(msgHandler *messages.MessageHandler, request *messages.StorageRequest) {
	log.Println("Attempting to store", request.FileName)

	// reject upload early if there isn’t enough free space.
	if !checkAvailableSpace(request.Size) {
		_ = msgHandler.SendResponse(false, "insufficient disk space")
		return
	}

	// removed msgHandler.Close() on errors so server doesn’t kill the connection unexpectedly.
	file, err := os.OpenFile(request.FileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		_ = msgHandler.SendResponse(false, err.Error())
		return
	}
	// use defer to guarantee the file closes even on early returns.
	defer file.Close()

	// added error handling when sending “ready for data”.
	if err := msgHandler.SendResponse(true, "ready for data"); err != nil {
		log.Println(err)
		return
	}

	// added error handling while receiving file data (CopyN can fail mid-transfer).
	sum := md5.New()
	w := io.MultiWriter(file, sum)
	if _, err := io.CopyN(w, msgHandler, int64(request.Size)); err != nil {
		log.Println(err)
		_ = msgHandler.SendResponse(false, "failed while receiving file data")
		return
	}

	// checksum now comes inside the StorageRequest (no extra checksum message needed).
	serverCheck := sum.Sum(nil)
	if util.VerifyChecksum(serverCheck, request.Checksum) {
		// send a final success response after verifying checksum.
		_ = msgHandler.SendResponse(true, "storage complete")
		log.Println("Successfully stored file.")
		return
	}

	// delete the file if checksum fails so we don’t keep corrupted uploads.
	_ = os.Remove(request.FileName)
	_ = msgHandler.SendResponse(false, "checksum mismatch; file discarded")
	log.Println("FAILED to store file. Invalid checksum.")
}

func handleRetrieval(msgHandler *messages.MessageHandler, request *messages.RetrievalRequest) {
	log.Println("Attempting to retrieve", request.FileName)

	// replaced log.Fatalln with a clean failure response (server keeps running).
	info, err := os.Stat(request.FileName)
	if err != nil {
		_ = msgHandler.SendRetrievalResponse(false, err.Error(), 0, nil)
		return
	}

	// handle errors when opening the file.
	file, err := os.Open(request.FileName)
	if err != nil {
		_ = msgHandler.SendRetrievalResponse(false, err.Error(), 0, nil)
		return
	}
	// defer close for safe cleanup.
	defer file.Close()

	// compute checksum first so it can be included in the retrieval response.
	sum := md5.New()
	if _, err := io.Copy(sum, file); err != nil {
		_ = msgHandler.SendRetrievalResponse(false, "failed to compute checksum", 0, nil)
		return
	}
	checksum := sum.Sum(nil)

	// rewind file back to start after hashing so we can send the bytes.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = msgHandler.SendRetrievalResponse(false, "failed to rewind file", 0, nil)
		return
	}

	// retrieval response now includes checksum (no separate checksum message needed).
	if err := msgHandler.SendRetrievalResponse(true, "ready to send", uint64(info.Size()), checksum); err != nil {
		log.Println(err)
		return
	}

	// added error logging for the actual send.
	if _, err := io.CopyN(msgHandler, file, info.Size()); err != nil {
		log.Println(err)
	}
}

func handleClient(msgHandler *messages.MessageHandler) {
	defer msgHandler.Close()

	// handle only one request per connection instead of looping forever.
	wrapper, err := msgHandler.Receive()
	if err != nil {
		log.Println(err)
		return
	}

	switch msg := wrapper.Msg.(type) {
	case *messages.Wrapper_StorageReq:
		handleStorage(msgHandler, msg.StorageReq)
	case *messages.Wrapper_RetrievalReq:
		handleRetrieval(msgHandler, msg.RetrievalReq)
	case nil:
		log.Println("Received an empty message, terminating client")
	default:
		log.Printf("Unexpected message type: %T", msg)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Not enough arguments. Usage: %s port [download-dir]\n", os.Args[0])
		os.Exit(1)
	}

	port := os.Args[1]
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalln(err.Error())
		os.Exit(1)
	}
	defer listener.Close()

	dir := "."
	if len(os.Args) >= 3 {
		dir = os.Args[2]
	}

	// create the download directory if it doesn’t exist.
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalln(err)
	}
	if err := os.Chdir(dir); err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Listening on port:", port)
	fmt.Println("Download directory:", dir)
	for {
		// handle accept errors without crashing the server.
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("Accepted connection", conn.RemoteAddr())
		handler := messages.NewMessageHandler(conn)
		go handleClient(handler)
	}
}
