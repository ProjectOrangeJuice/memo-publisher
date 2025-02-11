package main

import (
	"bufio"
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

func main() {

	http.HandleFunc("/webhook", webhookHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))

}

type webhookData struct {
	Activity string `json:"activityType"`
	Memo     struct {
		UID       string `json:"uid"`
		Content   string `json:"content"`
		Resources []struct {
			Name     string `json:"name"`
			Filename string `json:"filename"`
		}
		Visibility string `json:"visibility"`
	} `json:"memo"`
}

func webhookHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("Web hook")

	// Read the body data
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("Failed to read body, error: %v", err)
		return
	}

	// Read the json data
	var data webhookData
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Printf("Failed to read json body, error: %v body(%s)", err, body)
		return
	}

	log.Printf("Body data: %s", body)
	log.Printf("Json data: %+v", data)
	log.Printf("Visibility: %s", data.Memo.Visibility)

	if data.Memo.Visibility != "PUBLIC" {
		log.Printf("Not public, skipping")
		return
	}

	if data.Activity == "memos.memo.deleted" {
		// Delete the file
		err = os.Remove(fmt.Sprintf("%s.md", data.Memo.UID))
		if err != nil {
			log.Printf("Failed to delete file, error: %v", err)
			return
		}
		return
	}

	// Now we generate the template

	// Find the first # element
	heading, text := getFirstHashLineAndRemove(data.Memo.Content)
	log.Printf("Heading: %s", heading)

	// Generate the template
	template := fmt.Sprintf(`
+++
date = '%s'
draft = false
title = "%s"
+++
	%s`, time.Now().Format("2006-01-02"), heading, text)

	log.Printf("Template: %s", template)

	// Write the template to a file
	err = os.WriteFile(fmt.Sprintf("%s.md", data.Memo.UID), []byte(template), 0644)
	if err != nil {
		log.Printf("Failed to write file, error: %v", err)
		return
	}

	doGit(data.Memo.UID)
}

func doGit(fileID string) {
	// check if the folder "project-orange" exists
	if _, err := os.Stat("project-orange"); os.IsNotExist(err) {
		// Get creds from env
		gitstuff := os.Getenv("GITSTUFF")
		// Clone the repo
		cmd := exec.Command("git", "clone", "-c http.sslVerify=false", fmt.Sprintf("https://%s@gitea.localdomain/oharris/project-orange.git", gitstuff))
		err := cmd.Run()
		if err != nil {
			log.Printf("Failed to clone repo, error: %v", err)
			return
		}
	}
	log.Printf("Moving file")
	// move file into folder
	cmd := exec.Command("mv", fmt.Sprintf("%s.md", fileID), "project-orange/content/posts/")
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to move file, error: %v", err)
		return
	}
	log.Printf("Adding file")
	// Add the file
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = "project-orange"
	err = cmd.Run()
	if err != nil {
		log.Printf("Failed to add file, error: %v", err)
		return
	}
	log.Printf("commiting")

	// Commit the file
	cmd = exec.Command("git", "commit", "-m", fmt.Sprintf("Added file %s", fileID))
	cmd.Dir = "project-orange"
	err = cmd.Run()
	if err != nil {
		log.Printf("Failed to commit file, error: %v", err)
		return
	}
	log.Printf("pushing")
	// Push the file
	cmd = exec.Command("git", "push")
	cmd.Dir = "project-orange"
	err = cmd.Run()
	if err != nil {
		log.Printf("Failed to push file, error: %v", err)
		return
	}
	log.Printf("Finished")
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
