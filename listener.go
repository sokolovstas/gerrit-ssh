package gerritssh

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"

	"golang.org/x/crypto/ssh"
)

// NewGerritSSH creates, and returns a new GerritESListener object. Its only argument
// is a channel that the worker can add itself to whenever it is done its
// work.
func NewGerritSSH(id int, url string, username string, sshKeyPath string) GerritSSH {
	// Create, and return the worker.
	worker := GerritSSH{
		ID:         id,
		StopChan:   make(chan bool),
		Username:   username,
		SSHKeyPath: sshKeyPath,
		URL:        url,
	}

	return worker
}

// GerritSSH agent
type GerritSSH struct {
	ID         int
	Username   string
	SSHKeyPath string
	URL        string
	StopChan   chan bool
}

// Start stream event routine
func (g *GerritSSH) Start() {
	go func() {
		buffer := bytes.Buffer{}
		go g.sshConnection("stream-events", &buffer)

		event := StreamEvent{}
		for {
			if buffer.Len() != 0 {
				err := json.Unmarshal(buffer.Bytes(), &event)
				if err == nil {
					buffer.Reset()
					log.Printf("Gerrit SSH: recived event: %v", event.Type)
				} else {
					log.Fatalf("Gerrit SSH: parse event error: %v", err.Error())
				}
			}

			if <-g.StopChan {
				return
			}
		}
	}()
}

// Stop stream event routine
func (g *GerritSSH) Stop() {
	go func() {
		g.StopChan <- true
	}()
}

// Send command over SSH to gerrit instance
func (g *GerritSSH) Send(command string) (string, error) {
	return g.sshConnection(command, nil)
}

// Internal ssh connection function
func (g *GerritSSH) sshConnection(command string, buffer *bytes.Buffer) (string, error) {
	// Read ssh key
	pemBytes, err := ioutil.ReadFile(g.SSHKeyPath)
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	// Parse ssh key
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		log.Fatalf("Gerrit SSH: parse key failed:%v", err)
		return "", err
	}

	// Create config
	config := &ssh.ClientConfig{
		User: g.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}
	// Dial TCP
	conn, err := ssh.Dial("tcp", g.URL, config)
	if err != nil {
		log.Fatalf("Gerrit SSH: dial failed:%v", err)
		return "", err
	}
	defer conn.Close()
	// Start new session
	session, err := conn.NewSession()
	if err != nil {
		log.Fatalf("Gerrit SSH: session failed:%v", err)
		return "", err
	}
	defer session.Close()

	// Read to buffer
	if buffer != nil {
		session.Stdout = buffer
	} else {
		buffer = &bytes.Buffer{}
		session.Stdout = buffer
	}

	// Run command
	err = session.Run("gerrit " + command)
	if err != nil {
		log.Fatalf("Gerrit SSH: run failed:%v", err)
		return "", err
	}
	// Return result
	return buffer.String(), nil
}
