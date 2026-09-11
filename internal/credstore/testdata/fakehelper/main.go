// Command docker-credential-fake is a test double for the
// docker-credential-helpers binary protocol, backed by a JSON state file.
// The action is passed as argv[1]; the payload arrives on stdin.
//
// Environment:
//
//	FAKE_HELPER_STATE  path to the JSON state file (required)
//	FAKE_HELPER_PREFIX when set, list returns keys prefixed with
//	                  "Registry credentials for ", mimicking
//	                  docker-credential-secretservice
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const notFoundMessage = "credentials not found in native keychain"

type entry struct {
	Username string `json:"Username"`
	Secret   string `json:"Secret"`
}

func main() {
	state := os.Getenv("FAKE_HELPER_STATE")
	if state == "" {
		fmt.Fprintln(os.Stderr, "FAKE_HELPER_STATE not set")
		os.Exit(1)
	}

	creds := map[string]entry{}
	if data, err := os.ReadFile(state); err == nil {
		if err := json.Unmarshal(data, &creds); err != nil {
			fmt.Fprintf(os.Stderr, "corrupt state: %v\n", err)
			os.Exit(1)
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var rest strings.Builder
	for scanner.Scan() {
		rest.WriteString(scanner.Text())
		rest.WriteString("\n")
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing action argument")
		os.Exit(1)
	}
	action := os.Args[1]

	switch action {
	case "store":
		var c struct {
			ServerURL string `json:"ServerURL"`
			Username  string `json:"Username"`
			Secret    string `json:"Secret"`
		}
		if err := json.Unmarshal([]byte(rest.String()), &c); err != nil {
			fmt.Fprintf(os.Stderr, "invalid store payload: %v\n", err)
			os.Exit(1)
		}
		creds[c.ServerURL] = entry{Username: c.Username, Secret: c.Secret}
		save(state, creds)
	case "get":
		url := strings.TrimSpace(rest.String())
		e, ok := creds[url]
		if !ok {
			fmt.Println(notFoundMessage)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			ServerURL string `json:"ServerURL"`
			Username  string `json:"Username"`
			Secret    string `json:"Secret"`
		}{url, e.Username, e.Secret})
	case "erase":
		url := strings.TrimSpace(rest.String())
		delete(creds, url)
		save(state, creds)
	case "list":
		out := map[string]string{}
		for url, e := range creds {
			key := url
			if os.Getenv("FAKE_HELPER_PREFIX") != "" {
				key = "Registry credentials for " + url
			}
			out[key] = e.Username
		}
		_ = json.NewEncoder(os.Stdout).Encode(out)
	default:
		fmt.Fprintf(os.Stderr, "unknown action %q\n", action)
		os.Exit(1)
	}
}

func save(path string, creds map[string]entry) {
	data, err := json.Marshal(creds)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal state: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write state: %v\n", err)
		os.Exit(1)
	}
}
