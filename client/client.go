package client

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/iryonetwork/network-poc/config"
	"github.com/iryonetwork/network-poc/logger"
	"github.com/iryonetwork/network-poc/storage/ehr"
	"github.com/iryonetwork/network-poc/storage/eos"
	"github.com/iryonetwork/network-poc/storage/ws"
)

const tokenKey = "token"

type Client struct {
	config *config.Config
	eos    *eos.Storage
	ehr    *ehr.Storage
	log    *logger.Log
	ws     *ws.Storage
	token  string
}

func New(config *config.Config, eos *eos.Storage, ehr *ehr.Storage, log *logger.Log) (*Client, error) {
	return &Client{
		config: config,
		eos:    eos,
		ehr:    ehr,
		log:    log,
	}, nil
}

func (c *Client) ConnectWs() error {
	wsstorage, err := ws.Connect(c.config, c.log, c.ehr, c.eos, c.token)
	if err != nil {
		return err
	}
	c.ws = wsstorage
	c.Subscribe()
	return nil
}

func (c *Client) CloseWs() {
	c.ws.Close()
	c.ws = nil
}

func (c *Client) Login() error {
	// generate random hash
	hash := make([]byte, 32)
	_, err := rand.Read(hash)
	if err != nil {
		return fmt.Errorf("failed to generate random hash; %v", err)
	}

	// sign hash with private key
	sig, err := c.eos.SignHash(hash)
	if err != nil {
		return fmt.Errorf("Failed to sign the login request; %v", err)
	}
	req := url.Values{"sign": {sig}, "key": {c.config.GetEosPublicKey()}, "hash": {string(hash)}}
	if account := c.config.EosAccount; account != "" {
		req["account"] = []string{account}
	}
	// send login request
	response, err := http.PostForm(fmt.Sprintf("http://%s/login", c.config.IryoAddr), req)
	if err != nil {
		return fmt.Errorf("failed to call login; %v", err)
	}
	if response.StatusCode != 201 {
		return fmt.Errorf("Code: %d", response.StatusCode)
	}
	data := make(map[string]string)
	err = json.NewDecoder(response.Body).Decode(&data)
	// Login again after token expires
	go c.loginWaiter(data["validUntil"])
	// save token to client
	c.token = data["token"]
	return nil
}
func (c *Client) loginWaiter(str string) {
	i, err := strconv.ParseInt(str, 10, 64)
	// make request 5 seconds before token expires
	i -= 5
	if err != nil {
		c.log.Fatalf("Error getting validUntil")
	}
	time.Sleep(time.Until(time.Unix(i, 0)))
	if err := c.Login(); err != nil {
		log.Fatalf("Error logging in")
	}
}

func (c *Client) CreateAccount(key string) (string, error) {
	c.log.Debugf("Client::createaccount(%s) called", key)
	r, err := http.NewRequest("GET", fmt.Sprintf("http://%s/account/%s", c.config.IryoAddr, key), nil)
	if err != nil {
		return "", err
	}
	r.Header.Add("Authorization", c.token)
	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return "", err
	}
	if res.StatusCode != 201 {
		return "", fmt.Errorf("Code: %d", res.StatusCode)
	}
	var a map[string]string
	err = json.NewDecoder(res.Body).Decode(&a)
	c.log.Debugf("Client:: createaccount returned: %v", a)
	if err != nil {
		return "", err
	}

	if _, ok := a["error"]; ok {
		return "", fmt.Errorf(a["error"])
	}
	return a["account"], nil

}

func (c *Client) Ls(owner string) ([]map[string]string, error) {

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/%s", c.config.IryoAddr, owner), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", c.token)
	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Code: %d", res.StatusCode)
	}
	var a map[string][]map[string]string
	err = json.NewDecoder(res.Body).Decode(&a)
	if err != nil {
		return nil, err
	}
	if _, ok := a["error"]; ok {
		return nil, fmt.Errorf("Could not list files")
	}
	return a["files"], nil

}
func (c *Client) Download(owner, fileID string) error {
	c.log.Debugf("Client::Download(%s, %s) called", owner, fileID)

	// download file from server
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/%s/%s", c.config.IryoAddr, owner, fileID), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", c.token)
	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("Code: %d", res.StatusCode)
	}

	a, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// save file to local storage
	c.ehr.Saveid(owner, fileID, []byte(a))

	return nil
}

// Update downloads files for user, if they do not exist. Remove them if access was removed
func (c *Client) Update(owner string) error {
	// First check if access is granted
	if owner != c.config.EosAccount {
		granted, err := c.eos.AccessGranted(owner, c.config.EosAccount)
		if err != nil {
			return err
		}
		if !granted {
			c.ehr.RemoveUser(owner)
			return fmt.Errorf("You don't have permission granted to access this data")
		}
	}
	// List files
	list, err := c.Ls(owner)
	if err != nil {
		return err
	}
	// Download missing
	for _, f := range list {
		if !c.ehr.Exists(owner, f["fileID"]) {
			err = c.Download(owner, f["fileID"])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) Upload(owner, id string, reencrypt bool) error {
	c.log.Debugf("Client::upload(%s) called", owner)
	// get data from local storage
	data := c.ehr.Getid(owner, id)
	if data == nil {
		return fmt.Errorf("Document for %s does not exist", owner)
	}
	// lets get a signature
	sign, err := c.eos.SignHash(data)
	if err != nil {
		return err
	}

	// Get body for the request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("account", c.config.EosAccount)
	writer.WriteField("key", c.config.GetEosPublicKey())
	writer.WriteField("sign", sign)
	if reencrypt {
		writer.WriteField("reencrypt", "true")
	}
	part, err := writer.CreateFormFile("data", id)
	part.Write(data)

	err = writer.Close()
	if err != nil {
		return err
	}

	// upload data to server
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/%s", c.config.IryoAddr, owner), body)
	if err != nil {
		return fmt.Errorf("failed to call Upload; %v", err)
	}
	req.Header.Add("Authorization", c.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Upload; %v", err)
	}
	b, _ := ioutil.ReadAll(res.Body)
	if string(b) == "unknown token" {
		return fmt.Errorf(string(b))
	}
	if res.StatusCode != 201 {
		return fmt.Errorf("Got code: %d", res.StatusCode)
	}
	a := make(map[string]string)
	json.Unmarshal(b, &a)
	if err != nil {
		return err
	}
	newid := a["fileID"]
	c.ehr.Rename(owner, id, newid)
	return nil
}

func (c *Client) Reencrypt(key []byte) error {
	c.log.Debugf("Client:: Reencrypting")
	err := c.Update(c.config.EosAccount)
	if err != nil {
		return err
	}
	err = c.ehr.Reencrypt(c.config.EosAccount, c.config.EncryptionKeys[c.config.EosAccount], key)
	if err != nil {
		return err
	}

	// Save the new key
	c.config.EncryptionKeys[c.config.EosAccount] = key

	// Send notification about reencrypting to api
	err = c.ws.ReencryptRequest()
	if err != nil {
		return err
	}

	// Take some time, to make sure that api folder no loger contains files
	time.Sleep(2 * time.Second)

	// Upload new files
	for id := range c.ehr.Get(c.config.EosAccount) {
		c.log.Debugf("Uploading %s", id)
		c.Upload(c.config.EosAccount, id, true)
	}
	return nil
}
func (c *Client) GrantAccess(to string) error {
	c.log.Debugf("Client::grantAccess(%s) called", to)

	// Make sure that reciever exists
	if !c.eos.CheckAccountExists(to) {
		return fmt.Errorf("User does not exists")
	}

	// Check that users are not yet connected
	if ok, err := c.eos.AccessGranted(c.config.EosAccount, to); ok {
		if err != nil {
			return err
		}
		// make sure doctor is on list of connected
		conn := false
		for _, v := range c.config.Connections {
			if v == to {
				conn = true
			}
		}
		if !conn {
			c.config.Connections = append(c.config.Connections, to)
		}
		return nil
	}

	// write access granted to blockchain
	err := c.eos.GrantAccess(to)
	if err != nil {
		return fmt.Errorf("failed to call grantAccess; %v", err)
	}

	// if request for key was already made send the key
	// else notify the doctor that request was granted
	if _, ok := c.config.Requested[to]; ok {
		// send key for storage encryption
		err = c.ws.SendKey(to)
		if err != nil {
			return fmt.Errorf("failed to send key; %v", err)
		}
	} else {
		err = c.ws.NotifyGranted(to)
		if err != nil {
			return fmt.Errorf("Failed to notify access granted; %v", err)
		}
	}

	// add doctor to list of connected
	conn := false
	for _, v := range c.config.Connections {
		if v == to {
			conn = true
		}
	}
	if !conn {
		c.config.Connections = append(c.config.Connections, to)
	}

	/// remove key from storage
	delete(c.config.Requested, to)

	return nil
}

func (c *Client) RevokeAccess(to string) error {
	c.log.Debugf("Client::revokeAccess(%s) called", to)

	// send empty key to doctor to revoke the access
	err := c.ws.RevokeKey(to)
	if err != nil {
		return fmt.Errorf("Error revoking key: %v", err)
	}

	// write revoke access to blockchain
	err = c.eos.RevokeAccess(to)
	if err != nil {
		return err
	}
	// remove doctor from our connections
	found := false
	for i, v := range c.config.Connections {
		if v == to {
			c.config.Connections = append(c.config.Connections[:i], c.config.Connections[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%s is not in connections", to)
	}
	return nil
}

func (c *Client) Subscribe() {
	c.log.Debugf("Client::subscribe() called")

	//subscribe to key sent event
	switch c.config.ClientType {
	default:
		c.log.Fatalf("Unknown client type")
	case "Patient":
		c.ws.SubscribePatient()
	case "Doctor":
		c.ws.SubscribeDoctor()
	}
}

func (c *Client) RequestAccess(to string) error {
	err := c.ws.RequestsKey(to)
	return err
}

func retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; i < attempts; i++ {
		if err = f(); err == nil {
			return nil
		}

		time.Sleep(sleep)

		log.Println("retrying after error:", err)
	}

	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}
