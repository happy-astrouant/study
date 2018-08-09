package ws

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
)

type Requests interface {
	SendKey(to string)
	RevokeKey(to string)
	RequestKey(to string)
}

func (s *Storage) SendKey(to string) error {
	s.log.Debugf("WS:: Sending encryption key to %s", to)

	r := newReq("SendKey")
	r.append("to", []byte(to))
	// Encrypt key
	if _, ok := s.config.Requested[to]; !ok {
		return fmt.Errorf("No key from user %s found", to)
	}
	sha256 := sha256.New()
	encKey, err := rsa.EncryptOAEP(sha256, rand.Reader, s.config.Requested[to], s.config.EncryptionKeys[s.config.EosAccount], nil)
	if err != nil {
		return err
	}
	r.append("key", encKey)
	req, err := r.encode()
	if err != nil {
		return err
	}
	err = s.conn.WriteMessage(websocket.BinaryMessage, req)
	s.log.Debugf("Sent SendKey request")
	return err
}

func (s *Storage) RevokeKey(to string) error {
	s.log.Debugf("WS:: Revoking encryption key at %s", to)

	r := newReq("RevokeKey")
	r.append("to", []byte(to))
	req, err := r.encode()
	if err != nil {
		return err
	}
	err = s.conn.WriteMessage(websocket.BinaryMessage, req)
	return err
}

func (s *Storage) RequestsKey(to string) error {
	s.log.Debugf("WS:: Requesting encryption key from %s", to)

	r := newReq("RequestKey")
	r.append("to", []byte(to))

	reader := rand.Reader
	bitSize := 4096
	key, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return err
	}
	s.config.RequestKeys[to] = key
	r.append("key", marshalPKCS1PublicKey(&key.PublicKey))
	req, err := r.encode()
	err = s.conn.WriteMessage(websocket.BinaryMessage, req)

	return err
}

type request struct {
	Name   string
	Fields map[string][]byte
}
type field struct {
	Name string
	Data []byte
}

func newReq(name string) *request {
	return &request{name, make(map[string][]byte)}
}

func (r *request) append(name string, data []byte) {
	r.Fields[name] = data
}

func (r *request) encode() ([]byte, error) {
	return json.Marshal(r)
}

func decode(r []byte) (request, error) {
	req := request{}
	err := json.Unmarshal(r, &req)
	if err != nil {
		return req, err
	}
	return req, nil
}

func (r *request) getData(key string) ([]byte, error) {
	if d, ok := r.Fields[key]; ok {
		return d, nil
	}
	return []byte{}, fmt.Errorf("No data found for key %s", key)
}

func (r *request) getDataString(key string) (string, error) {
	if d, ok := r.Fields[key]; ok {
		return string(d), nil
	}
	return "", fmt.Errorf("No data found for key %s", key)
}
