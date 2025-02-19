package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
)

var gitRepoRegx = regexp.MustCompile(`([^\/]+).git$`)
var repoName string

func repoNameFromGit() string {
	if repoName != "" {
		return repoName
	}
	// use GitURL and extract the repo from it
	matches := gitRepoRegx.FindStringSubmatch(GitURL)
	if len(matches) < 2 {
		log.Fatalf("Could not read the repo name from the url, %s", GitURL)
	}
	log.Printf("Repo name set to %s", repoName)
	repoName = matches[1]
	return matches[1]
}

// prepGit clones into the repo if it doesn't exist
func prepGit() error {
	// check if the folder repoNameFromGit() exists
	if _, err := os.Stat(repoNameFromGit()); os.IsNotExist(err) {
		// Clone the repo
		cmd := exec.Command("git", "clone", "-c http.sslVerify=false", GitURL)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to clone repo, error: %v, output %s", err, output)
		}

		cmd = exec.Command("git", "config", "--global", "user.email", "bot@projectsync")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set user email, error: %v, output %s", err, output)
		}
		cmd = exec.Command("git", "config", "--global", "user.name", "Project Sync Bot")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set user name, error: %v, output %s", err, output)
		}

	} else {
		// Pull the repo
		cmd := exec.Command("git", "pull")
		cmd.Dir = repoNameFromGit()
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to pull repo, error: %v, output %s", err, output)
		}
	}
	return nil
}

func pushGit() error {
	log.Printf("Adding file")
	// Add the file
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repoNameFromGit()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to add file, error: %v, output %s", err, output)
	}
	log.Printf("commiting")

	// Commit the file
	cmd = exec.Command("git", "commit", "-m", fmt.Sprintf("Updated via webhook"))
	cmd.Dir = repoNameFromGit()
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to commit file, error: %v, output %s", err, output)
	}
	log.Printf("pushing")
	// Push the file
	cmd = exec.Command("git", "push")
	cmd.Dir = repoNameFromGit()
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to push file, error: %v, output %s", err, output)
	}
	log.Printf("Finished")
	return nil
}
