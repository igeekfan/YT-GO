package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func main() {
	if slices.Contains(os.Args, "--version") {
		if strings.HasPrefix(filepath.Base(os.Args[0]), "deno") {
			fmt.Println("deno 2.0.0")
		} else {
			fmt.Println("2026.01.01")
		}
		return
	}
	if slices.Contains(os.Args, "--dump-single-json") {
		data, err := os.ReadFile(os.Getenv("YTGO_TEST_PLAYLIST_FILE"))
		if err != nil {
			fail(err)
		}
		fmt.Println(string(data))
		return
	}
	if slices.Contains(os.Args, "--dump-json") {
		fail("the fixture URL points to a collection")
	}
	downloadURL, err := url.Parse(os.Args[len(os.Args)-1])
	if err != nil {
		fail(err)
	}
	id := downloadURL.Query().Get("v")
	if downloadURL.Hostname() == "youtu.be" {
		id = strings.TrimPrefix(downloadURL.Path, "/")
	}
	outputArg := slices.Index(os.Args, "--output")
	if outputArg < 0 || id == "" {
		fail("expected download arguments")
	}
	output := filepath.Join(filepath.Dir(os.Args[outputArg+1]), id+".mp4")
	switch id {
	case "FailVideo01":
		fail("fixture download failed")
	case "SilentFail1":
		if err := os.WriteFile(output, []byte("incomplete"), 0600); err != nil {
			fail(err)
		}
		fmt.Printf("[download] Destination: %s\n", output)
		fmt.Fprintln(os.Stderr, "ERROR: download failed with --ignore-errors")
		return
	case "CancelVid01":
		fmt.Println("[fixture] ready to cancel")
		time.Sleep(30 * time.Second)
		fail("expected cancellation")
	}
	if !slices.Contains(os.Args, "after_move:[YT-GO-YOUTUBE]%(.{id,extractor_key,filepath})j") {
		fail("expected completion metadata template")
	}
	if err := os.WriteFile(output, []byte("fixture media"), 0600); err != nil {
		fail(err)
	}
	metadata, err := json.Marshal(map[string]string{"id": id, "extractor_key": "Youtube", "filepath": output})
	if err != nil {
		fail(err)
	}
	fmt.Printf("[YT-GO-YOUTUBE]%s\n", metadata)
}

func fail(reason any) {
	fmt.Fprintln(os.Stderr, reason)
	os.Exit(1)
}
