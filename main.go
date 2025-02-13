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

func main() {
	http.HandleFunc("/webhook", webhookHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))

}

type resource struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
}

type webhookData struct {
	Activity string `json:"activityType"`
	Memo     struct {
		UID        string `json:"uid"`
		Content    string `json:"content"`
		Resources  []resource
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

	err = prepGit()
	if err != nil {
		log.Printf("Failed to setup git, error: %v", err)
		return
	}

	if data.Memo.Visibility != "PUBLIC" {
		log.Printf("Not public, skipping")
		// if the file has changed to private
		deleteFile(data.Memo.UID)
		err = pushGit()
		if err != nil {
			log.Printf("Failed to push git, error: %v", err)
			return
		}
		return
	}

	if data.Activity == "memos.memo.deleted" {
		deleteFile(data.Memo.UID)
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

	// Add the resources
	for _, res := range data.Memo.Resources {
		template = fmt.Sprintf("%s\n\n![%s](%s)", template, res.Filename, getLastDigits(res.Name)+res.Filename)
	}

	log.Printf("Template: %s", template)

	addFile(template, data.Memo.UID)
	updateResources(data.Memo.Resources, data.Memo.UID)
	err = pushGit()
	if err != nil {
		log.Printf("Failed to push git, error: %v", err)
		return
	}
}

func deleteFile(fileID string) {
	log.Printf("Deleting file")
	// Delete the file
	cmd := exec.Command("rm", "-r", fmt.Sprintf("project-orange/content/post/%s", fileID))
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to delete file, error: %v", err)
		return
	}

	err = pushGit()
	if err != nil {
		log.Printf("Failed to push git, error: %v", err)
		return
	}

}

func addFile(text string, fileID string) {
	// Create the folder if needed
	cmd := exec.Command("mkdir", "-p", fmt.Sprintf("project-orange/content/post/%s", fileID))
	_ = cmd.Run()

	// Write the template to a file
	err := os.WriteFile(fmt.Sprintf("project-orange/content/post/%s/index.md", fileID), []byte(text), 0644)
	if err != nil {
		log.Printf("Failed to write file, error: %v", err)
		return
	}
}

func pushGit() error {
	log.Printf("Adding file")
	// Add the file
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = "project-orange"
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to add file, error: %v", err)
	}
	log.Printf("commiting")

	// Commit the file
	cmd = exec.Command("git", "commit", "-m", fmt.Sprintf("Updated via webhook"))
	cmd.Dir = "project-orange"
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to commit file, error: %v", err)
	}
	log.Printf("pushing")
	// Push the file
	cmd = exec.Command("git", "push")
	cmd.Dir = "project-orange"
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to push file, error: %v", err)
	}
	log.Printf("Finished")
	return nil
}

func prepGit() error {
	// check if the folder "project-orange" exists
	if _, err := os.Stat("project-orange"); os.IsNotExist(err) {
		// Get creds from env
		gitstuff := os.Getenv("GITSTUFF")
		// Clone the repo
		cmd := exec.Command("git", "clone", "-c http.sslVerify=false", fmt.Sprintf("https://%s@gitea.localdomain/oharris/project-orange.git", gitstuff))
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("Failed to clone repo, error: %v", err)
		}
	} else {
		// Pull the repo
		cmd := exec.Command("git", "pull")
		cmd.Dir = "project-orange"
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("Failed to pull repo, error: %v", err)
		}
	}
	return nil
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
		resp, err := client.Get(fmt.Sprintf("https://memo.projectkube.com/file/%s/%s", res.Name, res.Filename))
		if err != nil {
			log.Printf("Failed to download resource, error: %v", err)
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read body, error: %v", err)
			return
		}

		// Write the resource to a file
		err = os.WriteFile(fmt.Sprintf("project-orange/content/post/%s/%s", fileID, getLastDigits(res.Name)+res.Filename), body, 0644)
		if err != nil {
			log.Printf("Failed to write resource, error: %v", err)
			return
		}
	}
}

func getLastDigits(text string) string {
	var lastDigits string
	for _, r := range text {
		if r >= '0' && r <= '9' {
			lastDigits = fmt.Sprintf("%s%c", lastDigits, r)
		} else {
			lastDigits = ""
		}
	}
	return lastDigits
}
