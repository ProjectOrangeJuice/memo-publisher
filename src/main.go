package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	GitURL  string
	MemoURL string
)

func main() {
	// fetch the environment variables
	GitURL = strings.TrimSpace(fetchEnv("GIT_URL"))
	MemoURL = strings.TrimSpace(fetchEnv("MEMO_URL"))

	log.Printf("Prepping git repo")
	err := prepGit()
	if err != nil {
		log.Fatalf("Could not setup git repo, %s", err)
	}

	log.Printf("Starting on port 8080")
	http.HandleFunc("/webhook", webhookHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// fetchEnv fetches the environment variable and will exit if it is not set
func fetchEnv(env string) string {
	if os.Getenv(env) == "" {
		log.Fatalf("Missing environment variable: %s", env)
	}
	return os.Getenv(env)
}

type webhookData struct {
	Activity string `json:"activityType"`
	Memo     struct {
		MemoID     string
		Name       string `json:"name"`
		Content    string `json:"content"`
		Resources  []resource
		Visibility string `json:"visibility"`
	} `json:"memo"`
}

type resource struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
}

func webhookHandler(w http.ResponseWriter, req *http.Request) {
	log.Print("Webhook called")

	// Read the body data
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("Failed to read body, error: %s", err)
		return
	}
	log.Printf("body.. %s", body)

	// Read the json data
	var data webhookData
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Printf("Failed to read json body, error: %s body(%s)", err, body)
		return
	}

	// Prep the git repo before working on it
	err = prepGit()
	if err != nil {
		log.Printf("Failed to setup git, error: %s", err)
		return
	}

	data.Memo.MemoID = strings.SplitN(data.Memo.Name, "/", 2)[1] // should check this

	// Delete the memo to keep the resources correct (we lazily download every time)
	deleteFile(data.Memo.MemoID)

	if data.Memo.Visibility == "PUBLIC" && data.Activity != "memos.memo.deleted" {
		log.Printf("Updating or creating post")
		handleUpdate(data)
	}

	err = pushGit()
	if err != nil {
		log.Printf("Failed to push git, error: %s", err)
		return
	}
}

func handleUpdate(data webhookData) error {
	log.Printf("webhook data [%+v]", data)
	// Find the first # element
	heading, text := getFirstHashLineAndRemove(data.Memo.Content)
	log.Printf("Heading: %s", heading)
	// Generate the template
	template := fmt.Sprintf(`
+++
date = '%s'
draft = false
title = "%s"
summary = "%s"
+++
%s`, time.Now().Format("2006-01-02"), heading, getFirstParagraph(text), text)

	// Add the resources
	for _, res := range data.Memo.Resources {
		template = fmt.Sprintf("%s\n\n![%s](%s)", template, res.Filename, getResourceNumber(res.Name)+res.Filename)
	}

	log.Printf("Template: %s", template)

	addFile(template, data.Memo.MemoID)
	updateResources(data.Memo.Resources, data.Memo.MemoID)
	return nil
}

func deleteFile(fileID string) {
	log.Printf("Deleting file")
	// Delete the file
	cmd := exec.Command("rm", "-r", fmt.Sprintf("%s/content/post/%s", repoNameFromGit(), fileID))
	err, output := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to delete file, error: %s, output %s", err, output)
		return
	}
}

func addFile(text string, fileID string) {
	log.Printf("Adding the file with ID %s", fileID)
	// Create the folder if needed
	cmd := exec.Command("mkdir", "-p", fmt.Sprintf("%s/content/post/%s", repoNameFromGit(), fileID))
	_ = cmd.Run() // We don't care for this error

	// Write the template to a file
	err := os.WriteFile(fmt.Sprintf("%s/content/post/%s/index.md", repoNameFromGit(), fileID), []byte(text), 0644)
	if err != nil {
		log.Printf("Failed to write file, error: %s", err)
		return
	}
}

func getFirstHashLineAndRemove(text string) (string, string) {
	var firstHashLine string
	var remainingText string
	scanner := bufio.NewScanner(strings.NewReader(text))
	var lines []string
	found := false

	for scanner.Scan() {
		line := scanner.Text()
		if !found && strings.HasPrefix(line, "#") {
			// Remove the #
			line = strings.TrimPrefix(line, "#")
			firstHashLine = line
			found = true
		} else {
			lines = append(lines, line)
		}
	}
	remainingText = strings.Join(lines, "\n")
	return firstHashLine, remainingText
}

func getFirstParagraph(text string) string {
	splits := strings.Split(text, "\n")
	for _, split := range splits {
		// first line that contains text
		if len(strings.TrimSpace(split)) > 0 {
			return split
		}
	}
	return ""
}

var (
	tr = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client = &http.Client{Transport: tr}
)

func updateResources(resources []resource, fileID string) {
	// Download the resource
	for _, res := range resources {
		log.Printf("Downloading resource: %s", res.Name)
		resp, err := client.Get(fmt.Sprintf("%s/file/%s/%s", MemoURL, res.Name, res.Filename))
		if err != nil {
			log.Printf("Failed to download resource, error: %s", err)
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read body, error: %s", err)
			return
		}

		// Write the resource to a file
		err = os.WriteFile(fmt.Sprintf("%s/content/post/%s/%s", repoNameFromGit(), fileID, getResourceNumber(res.Name)+res.Filename), body, 0644)
		if err != nil {
			log.Printf("Failed to write resource, error: %s", err)
			return
		}
	}
}

func getResourceNumber(resourceName string) string {
	var lastDigits string
	for _, r := range resourceName {
		if r >= '0' && r <= '9' {
			lastDigits = fmt.Sprintf("%s%c", lastDigits, r)
		} else {
			lastDigits = ""
		}
	}
	return lastDigits
}
