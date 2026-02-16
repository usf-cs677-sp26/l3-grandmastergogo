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
	"path/filepath"
	"strings"
)

func put(msgHandler *messages.MessageHandler, fileName string) int {
	fmt.Println("PUT", fileName)

	// Get file size and make sure it exists
	info, err := os.Stat(fileName)
	if err != nil {
		log.Fatalln(err)
	}

	// Calculate checksum before sending
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalln(err)
	}
	md5 := md5.New()
	io.Copy(md5, file)
	checksum := md5.Sum(nil)
	file.Close()

	// Tell the server we want to store this file (with checksum)
	// Send only the base filename (no directories allowed per spec)
	baseName := filepath.Base(fileName)
	msgHandler.SendStorageRequest(baseName, uint64(info.Size()), checksum)
	if ok, _ := msgHandler.ReceiveResponse(); !ok {
		return 1
	}

	// Send the file data
	file, _ = os.Open(fileName)
	io.CopyN(msgHandler, file, info.Size())
	file.Close()

	// Wait for final acknowledgement from server
	if ok, msg := msgHandler.ReceiveResponse(); !ok {
		log.Println("Storage failed:", msg)
		return 1
	}

	fmt.Println("Storage complete!")
	return 0
}

func get(msgHandler *messages.MessageHandler, fileName string, destDir string) int {
	fmt.Println("GET", fileName)

	// Create file in destination directory with base filename
	baseName := filepath.Base(fileName)
	outPath := filepath.Join(destDir, baseName)
	file, err := os.OpenFile(outPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		log.Println(err)
		return 1
	}

	// Send only basename to server (no directories allowed per spec)
	msgHandler.SendRetrievalRequest(baseName)
	ok, _, size, serverCheck := msgHandler.ReceiveRetrievalResponse()
	if !ok {
		file.Close()
		os.Remove(outPath) // Clean up file if retrieval failed
		return 1
	}

	// Receive file data and calculate checksum
	md5 := md5.New()
	w := io.MultiWriter(file, md5)
	io.CopyN(w, msgHandler, int64(size))
	file.Close()

	// Verify checksum against the one received in response
	clientCheck := md5.Sum(nil)
	if util.VerifyChecksum(serverCheck, clientCheck) {
		log.Println("Successfully retrieved file.")
		return 0
	} else {
		log.Println("FAILED to retrieve file. Invalid checksum.")
		os.Remove(outPath) // Remove corrupted file
		return 1
	}
}

func main() {
	if len(os.Args) < 4 {
		fmt.Printf("Not enough arguments. Usage: %s server:port put|get file-name [download-dir]\n", os.Args[0])
		os.Exit(1)
	}

	host := os.Args[1]
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Fatalln(err.Error())
		return
	}
	msgHandler := messages.NewMessageHandler(conn)
	defer conn.Close()

	action := strings.ToLower(os.Args[2])
	if action != "put" && action != "get" {
		log.Fatalln("Invalid action", action)
	}

	fileName := os.Args[3]

	dir := "."
	if len(os.Args) >= 5 {
		dir = os.Args[4]
	}
	openDir, err := os.Open(dir)
	if err != nil {
		log.Fatalln(err)
	}
	openDir.Close()

	if action == "put" {
		os.Exit(put(msgHandler, fileName))
	} else if action == "get" {
		os.Exit(get(msgHandler, fileName, dir))
	}
}
