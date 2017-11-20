package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger = StdoutLogger{}
	retCode := m.Run()
	os.Exit(retCode)
}

func TestWorks(t *testing.T) {
	assert := assert.New(t)
	assert.True(true)
}

func TestGithubHandlerNoEnv(t *testing.T) {
	assert := assert.New(t)
	gh := &GithubHandler{}
	assert.Equal(gh.GithubURL(), "https://github.com")

	token, err := gh.GithubAccessToken()
	assert.Empty(token)
	assert.NotNil(err)

	cli, err := gh.GithubClient()
	assert.Nil(cli)
	assert.NotNil(err)

	r, _ := http.NewRequest("GET", "http://localhost:41968/raw/org/repo/master/README.md", strings.NewReader(""))
	err = gh.ProbablyAuthenticate(r)
	assert.NotNil(err)
}

func TestGithubHandlerWithEnv(t *testing.T) {
	assert := assert.New(t)
	gh := &GithubHandler{}
	ghURL := "https://somegithub.enterprise.com"
	ghToken := "abcdefg"
	os.Setenv("GITHUB_URL", ghURL)
	os.Setenv("GITHUB_ACCESS_TOKEN", ghToken)

	assert.Equal(gh.GithubURL(), ghURL)

	token, err := gh.GithubAccessToken()
	assert.Equal(token, ghToken)
	assert.Nil(err)

	cli, err := gh.GithubClient()
	assert.Nil(err)
	assert.NotEmpty(cli)
	assert.Regexp(fmt.Sprintf(".*%s.*", ghURL), cli.BaseURL)
}

func TestProbablyAuthenticateRawAuth(t *testing.T) {
	assert := assert.New(t)
	gh := &GithubHandler{}
	ghURL := "https://somegithub.enterprise.com"
	ghToken := "abcdefg"
	os.Setenv("GITHUB_URL", ghURL)
	os.Setenv("GITHUB_ACCESS_TOKEN", ghToken)

	r, err := http.NewRequest("GET", "http://localhost:41968/raw/org/repo/master/README.md", strings.NewReader(""))
	assert.Nil(err)

	gh.ProbablyAuthenticate(r)
	assert.Equal(r.Header.Get("Authorization"), fmt.Sprintf("token %s", ghToken))
	assert.Equal(r.Header.Get("Accept"), "application/vnd.github.v3.raw")
}

func TestProbablyAuthenticateGitAuth(t *testing.T) {
	assert := assert.New(t)
	gh := &GithubHandler{}
	ghURL := "https://somegithub.enterprise.com"
	ghToken := "abcdefg"
	os.Setenv("GITHUB_URL", ghURL)
	os.Setenv("GITHUB_ACCESS_TOKEN", ghToken)

	r, err := http.NewRequest("GET", "http://localhost:41968/org/repo.git", strings.NewReader(""))
	assert.Nil(err)

	gh.ProbablyAuthenticate(r)
	user, pass, _ := r.BasicAuth()
	assert.Equal(user, ghToken)
	assert.Equal("x-oauth-basic", pass)
}

func TestReleaseAssets(t *testing.T) {
	assert := assert.New(t)
	gh := &GithubHandler{}
	ghURL := "https://somegithub.enterprise.com"
	ghToken := "abcdefg"
	os.Setenv("GITHUB_URL", ghURL)
	os.Setenv("GITHUB_ACCESS_TOKEN", ghToken)
	w := httptest.NewRecorder()

	r, _ := http.NewRequest("GET", "http://localhost:41968/org/repo.git", strings.NewReader(""))

	did, err := gh.HandleReleaseAssets(w, r)
	assert.False(did)
	assert.Nil(err)

	// todo: handle actually retrieving release
	// releaseURL := "https://github.build.ge.com/owner/repo/releases/download/1.0/artifact.tar.gz"
	// r, err = http.NewRequest("GET", "http://localhost:41968/org/repo.git", strings.NewReader(""))
}
