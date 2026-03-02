package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Cmd represents a GOCACHEPROG protocol command.
type Cmd string

const (
	CmdGet   Cmd = "get"
	CmdClose Cmd = "close"
)

// Request is a JSON message sent from the go command.
type Request struct {
	ID       int64  `json:"ID"`
	Command  Cmd    `json:"Command"`
	ActionID []byte `json:"ActionID,omitempty"`
}

// Response is a JSON message sent back to the go command.
type Response struct {
	ID            int64      `json:"ID"`
	Err           string     `json:"Err,omitempty"`
	KnownCommands []Cmd      `json:"KnownCommands,omitempty"`
	Miss          bool       `json:"Miss,omitempty"`
	OutputID      []byte     `json:"OutputID,omitempty"`
	Size          int64      `json:"Size,omitempty"`
	Time          *time.Time `json:"Time,omitempty"`
	DiskPath      string     `json:"DiskPath,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: gocacheprog <cache-dir>\n")
		os.Exit(1)
	}
	cacheDir := os.Args[1]

	if fi, err := os.Stat(cacheDir); err != nil {
		fmt.Fprintf(os.Stderr, "gocacheprog: warning: cache directory %s: %v\n", cacheDir, err)
	} else if !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "gocacheprog: warning: %s is not a directory\n", cacheDir)
	}

	enc := json.NewEncoder(os.Stdout)
	dec := json.NewDecoder(os.Stdin)

	if err := enc.Encode(Response{KnownCommands: []Cmd{CmdGet, CmdClose}}); err != nil {
		fmt.Fprintf(os.Stderr, "gocacheprog: failed to write init response: %v\n", err)
		os.Exit(1)
	}

	for {
		var req Request
		if err := dec.Decode(&req); err != nil {
			return
		}

		switch req.Command {
		case CmdGet:
			resp := handleGet(cacheDir, &req)
			if err := enc.Encode(resp); err != nil {
				return
			}
		case CmdClose:
			_ = enc.Encode(Response{ID: req.ID})
			return
		}
	}
}

// actionEntryLen is the fixed length of a Go cache action entry.
// Format: "v1 <actionID-hex(64)> <outputID-hex(64)> <size(20)> <timestamp(20)>\n"
const actionEntryLen = 175

func handleGet(cacheDir string, req *Request) *Response {
	miss := &Response{ID: req.ID, Miss: true}

	if len(req.ActionID) != 32 {
		return miss
	}

	actionHex := hex.EncodeToString(req.ActionID)
	actionPath := filepath.Join(cacheDir, actionHex[:2], actionHex+"-a")

	data, err := os.ReadFile(actionPath)
	if err != nil {
		return miss
	}

	if len(data) != actionEntryLen {
		return miss
	}
	if !strings.HasPrefix(string(data), "v1 ") {
		return miss
	}
	if data[actionEntryLen-1] != '\n' {
		return miss
	}

	entryActionHex := string(data[3:67])
	if entryActionHex != actionHex {
		return miss
	}

	outputHex := string(data[68:132])
	outputID, err := hex.DecodeString(outputHex)
	if err != nil {
		return miss
	}

	sizeStr := strings.TrimSpace(string(data[133:153]))
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return miss
	}

	tsStr := strings.TrimSpace(string(data[154:174]))
	tsNano, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return miss
	}
	t := time.Unix(0, tsNano)

	diskPath := filepath.Join(cacheDir, outputHex[:2], outputHex+"-d")
	if _, err := os.Stat(diskPath); err != nil {
		return miss
	}

	return &Response{
		ID:       req.ID,
		OutputID: outputID,
		Size:     size,
		Time:     &t,
		DiskPath: diskPath,
	}
}
