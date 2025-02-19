package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegex(t *testing.T) {
	gitUrl := "https://user:abctoken@git.localdomain/user/git-repo.git"
	matches := gitRepoRegx.FindStringSubmatch(gitUrl)
	require.Equal(t, 2, len(matches))
	require.Equal(t, "git-repo", matches[1])
}
