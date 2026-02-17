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

	// handle errors without crashing (return 1 instead of log.Fatalln).
	info, err := os.Stat(fileName)
	if err != nil {
		log.Println(err)
		return 1
	}

	// added error handling + defer close (safer cleanup).
	file, err := os.Open(fileName)
	if err != nil {
		log.Println(err)
		return 1
	}
	defer file.Close()

	// compute checksum first so it can be sent to server in the storage request.
	sum := md5.New()
	if _, err := io.Copy(sum, file); err != nil {
		log.Println(err)
		return 1
	}
	checksum := sum.Sum(nil)

	// reset file pointer after hashing so we can actually send the file contents.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		log.Println(err)
		return 1
	}

	// Tell the server we want to store this file
	// send base filename only + include checksum in the storage request + handle Send errors.
	if err := msgHandler.SendStorageRequest(filepath.Base(fileName), uint64(info.Size()), checksum); err != nil {
		log.Println(err)
		return 1
	}
	if ok, _ := msgHandler.ReceiveResponse(); !ok {
		return 1
	}

	// added error handling for the upload transfer.
	if _, err := io.CopyN(msgHandler, file, info.Size()); err != nil {
		log.Println(err)
		return 1
	}

	// removed explicit checksum-verification message; now server can validate using checksum already sent.
	if ok, _ := msgHandler.ReceiveResponse(); !ok {
		return 1
	}

	fmt.Println("Storage complete!")
	return 0
}

func get(msgHandler *messages.MessageHandler, fileName string, destinationDir string) int {
	fmt.Println("GET", fileName)

	// download to destinationDir and sanitize name using Base().
	targetPath := filepath.Join(destinationDir, filepath.Base(fileName))

	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		log.Println(err)
		return 1
	}
	defer file.Close()

	// send base filename only + handle Send errors.
	if err := msgHandler.SendRetrievalRequest(filepath.Base(fileName)); err != nil {
		log.Println(err)
		return 1
	}

	// server now returns checksum inside retrieval response (no extra Receive() needed).
	ok, _, size, serverCheck := msgHandler.ReceiveRetrievalResponse()
	if !ok {
		return 1
	}

	sum := md5.New()
	w := io.MultiWriter(file, sum)

	// added error handling for the download transfer.
	if _, err := io.CopyN(w, msgHandler, int64(size)); err != nil {
		log.Println(err)
		return 1
	}
	clientCheck := sum.Sum(nil)

	if util.VerifyChecksum(serverCheck, clientCheck) {
		log.Println("Successfully retrieved file.")
		return 0
	}

	// delete the file if checksum fails so we don’t keep corrupted output.
	log.Println("FAILED to retrieve file. Invalid checksum.")
	_ = os.Remove(targetPath)
	return 1
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
	}

	// GET now uses the destination directory parameter.
	os.Exit(get(msgHandler, fileName, dir))
}
