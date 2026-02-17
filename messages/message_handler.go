package messages

import (
	"encoding/binary"
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

func (m *MessageHandler) ReadN(buf []byte) error {
	bytesRead := uint64(0)
	for bytesRead < uint64(len(buf)) {
		n, err := m.conn.Read(buf[bytesRead:])
		if err != nil {
			return err
		}
		bytesRead += uint64(n)
	}
	return nil
}

func (m *MessageHandler) Read(p []byte) (n int, err error) {
	return m.conn.Read(p)
}

func (m *MessageHandler) Write(p []byte) (n int, err error) {
	return m.conn.Write(p)
}

func (m *MessageHandler) WriteN(buf []byte) error {
	bytesWritten := uint64(0)
	for bytesWritten < uint64(len(buf)) {
		n, err := m.conn.Write(buf[bytesWritten:])
		if err != nil {
			return err
		}
		bytesWritten += uint64(n)
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
	// error handling to allow callers to handle network errors correctly.
	if err := m.WriteN(prefix); err != nil {
		return err
	}

	// error propagation for payload write to prevent partial message sends going unnoticed.
	if err := m.WriteN(serialized); err != nil {
		return err
	}

	return nil
}

func (m *MessageHandler) Receive() (*Wrapper, error) {
	prefix := make([]byte, 8)

	// Propagate read failures when reading the length header.
	if err := m.ReadN(prefix); err != nil {
		return nil, err
	}

	payloadSize := binary.LittleEndian.Uint64(prefix)
	payload := make([]byte, payloadSize)

	// Propagate read failures when reading the payload bytes.
	if err := m.ReadN(payload); err != nil {
		return nil, err
	}

	wrapper := &Wrapper{}
	err := proto.Unmarshal(payload, wrapper)
	return wrapper, err
}

func (m *MessageHandler) Close() {
	m.conn.Close()
}

func (m *MessageHandler) SendStorageRequest(fileName string, size uint64, checksum []byte) error {
	// Include checksum in the storage request so the server can verify integrity after upload.
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
	// Include checksum in retrieval response so the client can verify downloaded file integrity.
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
		// Return nil checksum on failure to keep return values consistent/safe for callers.
		return false, "", 0, nil
	}

	rr := resp.GetRetrievalResp().GetResp()
	log.Println(rr.Message)

	// Return checksum along with status/message/size so callers can validate retrieved file contents.
	return rr.Ok, rr.Message, resp.GetRetrievalResp().Size, resp.GetRetrievalResp().Checksum
}
