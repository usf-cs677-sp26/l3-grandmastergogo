package messages

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"google.golang.org/protobuf/proto"
)

type MessageHandler struct {
	conn net.Conn
}

func NewMessageHandler(conn net.Conn) *MessageHandler {
	m := &MessageHandler{
		conn: conn,
	}

	return m
}

func (m *MessageHandler) readN(buf []byte) error {
	_, err := io.ReadFull(m.conn, buf)
	return err
}

func (m *MessageHandler) Read(p []byte) (n int, err error) {
	return m.conn.Read(p)
}

func (m *MessageHandler) Write(p []byte) (n int, err error) {
	return m.conn.Write(p)
}

func (m *MessageHandler) writeN(buf []byte) error {
	for len(buf) > 0 {
		n, err := m.conn.Write(buf)
		if err != nil {
			return err
		}
		buf = buf[n:]
	}
	return nil
}

func (m *MessageHandler) Send(wrapper *Wrapper) error {
	serialized, err := proto.Marshal(wrapper)
	if err != nil {
		return err
	}

	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(serialized)))
	if err := m.writeN(prefix); err != nil {
		return err
	}
	return m.writeN(serialized)
}

func (m *MessageHandler) Receive() (*Wrapper, error) {
	prefix := make([]byte, 8)
	if err := m.readN(prefix); err != nil {
		return nil, err
	}

	payloadSize := binary.LittleEndian.Uint64(prefix)
	if payloadSize > (1 << 30) { // sanity limit: 1GiB
		return nil, fmt.Errorf("message too large: %d bytes", payloadSize)
	}

	payload := make([]byte, int(payloadSize))
	if err := m.readN(payload); err != nil {
		return nil, err
	}

	wrapper := &Wrapper{}
	return wrapper, proto.Unmarshal(payload, wrapper)
}

func (m *MessageHandler) Close() {
	m.conn.Close()
}

func (m *MessageHandler) SendStorageRequest(fileName string, size uint64, checksum []byte) error {
	msg := StorageRequest{FileName: fileName, Size: size, Checksum: checksum}
	wrapper := &Wrapper{
		Msg: &Wrapper_StorageReq{StorageReq: &msg},
	}
	return m.Send(wrapper)
}

func (m *MessageHandler) SendRetrievalRequest(fileName string) error {
	msg := RetrievalRequest{FileName: fileName}
	wrapper := &Wrapper{
		Msg: &Wrapper_RetrievalReq{RetrievalReq: &msg},
	}
	return m.Send(wrapper)
}

func (m *MessageHandler) SendChecksumVerification(checksum []byte) error {
	checkMsg := ChecksumVerification{Checksum: checksum}
	checkWrapper := &Wrapper{
		Msg: &Wrapper_Checksum{Checksum: &checkMsg},
	}
	return m.Send(checkWrapper)
}

func (m *MessageHandler) SendResponse(ok bool, str string) error {
	msg := Response{Ok: ok, Message: str}
	wrapper := &Wrapper{
		Msg: &Wrapper_Response{Response: &msg},
	}

	return m.Send(wrapper)
}

func (m *MessageHandler) SendRetrievalResponse(ok bool, str string, size uint64, checksum []byte) error {
	resp := Response{Ok: ok, Message: str}
	msg := RetrievalResponse{Resp: &resp, Size: size, Checksum: checksum}
	wrapper := &Wrapper{
		Msg: &Wrapper_RetrievalResp{RetrievalResp: &msg},
	}

	return m.Send(wrapper)
}

func (m *MessageHandler) ReceiveResponse() (bool, string) {
	resp, err := m.Receive()
	if err != nil {
		return false, ""
	}

	log.Println(resp.GetResponse().Message)
	return resp.GetResponse().Ok, resp.GetResponse().Message
}

func (m *MessageHandler) ReceiveRetrievalResponse() (bool, string, uint64, []byte) {
	resp, err := m.Receive()
	if err != nil {
		return false, "", 0, nil
	}

	rr := resp.GetRetrievalResp().GetResp()
	log.Println(rr.Message)
	return rr.Ok, rr.Message, resp.GetRetrievalResp().Size, resp.GetRetrievalResp().Checksum
}
