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

func checkDiskSpace(requiredBytes uint64) bool {
	var stat syscall.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		log.Println("Error getting working directory:", err)
		return false
	}
	
	err = syscall.Statfs(wd, &stat)
	if err != nil {
		log.Println("Error checking disk space:", err)
		return false
	}
	
	// Available space = available blocks * block size
	availableSpace := stat.Bavail * uint64(stat.Bsize)
	log.Printf("Available disk space: %d bytes, Required: %d bytes\n", availableSpace, requiredBytes)
	
	return availableSpace >= requiredBytes
}

func handleStorage(msgHandler *messages.MessageHandler, request *messages.StorageRequest) {
	log.Println("Attempting to store", request.FileName)
	
	// Check if file already exists (refuse to overwrite)
	if _, err := os.Stat(request.FileName); err == nil {
		msgHandler.SendResponse(false, "File already exists")
		return
	}
	
	// Check available disk space
	if !checkDiskSpace(request.Size) {
		msgHandler.SendResponse(false, "Insufficient disk space")
		return
	}
	
	// Create file
	file, err := os.OpenFile(request.FileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		msgHandler.SendResponse(false, err.Error())
		return
	}

	msgHandler.SendResponse(true, "Ready for data")
	
	// Receive file data and calculate checksum
	md5 := md5.New()
	w := io.MultiWriter(file, md5)
	io.CopyN(w, msgHandler, int64(request.Size))
	file.Close()

	serverCheck := md5.Sum(nil)

	// Verify checksum against client's checksum from request
	if util.VerifyChecksum(serverCheck, request.Checksum) {
		log.Println("Successfully stored file.")
		msgHandler.SendResponse(true, "File stored successfully")
	} else {
		log.Println("FAILED to store file. Invalid checksum.")
		os.Remove(request.FileName) // Delete corrupted file
		msgHandler.SendResponse(false, "Checksum verification failed")
	}
}

func handleRetrieval(msgHandler *messages.MessageHandler, request *messages.RetrievalRequest) {
	log.Println("Attempting to retrieve", request.FileName)

	// Get file size and make sure it exists
	info, err := os.Stat(request.FileName)
	if err != nil {
		log.Println("File not found:", err)
		msgHandler.SendRetrievalResponse(false, "File not found", 0, nil)
		return
	}

	// Calculate checksum before sending
	file, err := os.Open(request.FileName)
	if err != nil {
		log.Println("Error opening file:", err)
		msgHandler.SendRetrievalResponse(false, "Error opening file", 0, nil)
		return
	}
	md5 := md5.New()
	io.Copy(md5, file)
	checksum := md5.Sum(nil)
	file.Close()

	// Send response with size and checksum
	msgHandler.SendRetrievalResponse(true, "Ready to send", uint64(info.Size()), checksum)

	// Send file data
	file, _ = os.Open(request.FileName)
	io.CopyN(msgHandler, file, info.Size())
	file.Close()
}

func handleClient(msgHandler *messages.MessageHandler) {
	defer msgHandler.Close()

	for {
		wrapper, err := msgHandler.Receive()
		if err != nil {
			log.Println(err)
		}

		switch msg := wrapper.Msg.(type) {
		case *messages.Wrapper_StorageReq:
			handleStorage(msgHandler, msg.StorageReq)
			continue
		case *messages.Wrapper_RetrievalReq:
			handleRetrieval(msgHandler, msg.RetrievalReq)
			continue
		case nil:
			log.Println("Received an empty message, terminating client")
			return
		default:
			log.Printf("Unexpected message type: %T", msg)
		}
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
	if err := os.Chdir(dir); err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Listening on port:", port)
	fmt.Println("Download directory:", dir)
	for {
		if conn, err := listener.Accept(); err == nil {
			log.Println("Accepted connection", conn.RemoteAddr())
			handler := messages.NewMessageHandler(conn)
			go handleClient(handler)
		}
	}
}
