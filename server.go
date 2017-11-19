package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/cssivision/reverseproxy"
	github "github.com/google/go-github/github"
	"github.com/kardianos/service"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

var logger service.Logger

type GithubHandler struct {
	githubURL         string
	githubAccessToken string
	githubClient      *github.Client
}

func (h *GithubHandler) GithubClient() (*github.Client, error) {
	if h.githubClient == nil {
		accessToken, err := h.GithubAccessToken()
		if err != nil {
			return nil, err
		}
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: accessToken},
		)
		tc := oauth2.NewClient(ctx, ts)
		ghapiurl := fmt.Sprintf("%s/api/v3", h.GithubURL())
		cli, err := github.NewEnterpriseClient(ghapiurl, ghapiurl, tc)
		if err != nil {
			return nil, err
		}
		h.githubClient = cli
	}
	return h.githubClient, nil
}

func (h *GithubHandler) GithubURL() string {
	if len(h.githubURL) == 0 {
		gh := os.Getenv("GITHUB_URL")
		if len(gh) == 0 {
			gh = "https://github.com"
		}
		h.githubURL = gh
	}
	return h.githubURL
}

func (h *GithubHandler) GithubAccessToken() (string, error) {
	if len(h.githubAccessToken) == 0 {
		ghat := os.Getenv("GITHUB_ACCESS_TOKEN")
		if len(ghat) == 0 {
			return "", errors.New("couldn't find an access token")
		}
		h.githubAccessToken = ghat
	}
	return h.githubAccessToken, nil
}

func authenticateRawRequest(r *http.Request, accessToken string) error {
	r.Header.Add("Authorization", fmt.Sprintf("token %s", accessToken))
	r.Header.Add("Accept", "application/vnd.github.v3.raw")
	return nil
}

func authenticateGitOverHTTP(r *http.Request, accessToken string) error {
	logger.Infof("Adding basic auth to %s", r.RequestURI)
	r.SetBasicAuth(accessToken, "x-oauth-basic")
	return nil
}

func (h *GithubHandler) ProbablyAuthenticate(r *http.Request) error {
	accessToken, err := h.GithubAccessToken()
	if err != nil {
		return err
	}
	// todo: eventually we will want to strip out the org/user and repo
	// then we can do things like introduce repo-specific configurations
	requestURI := r.URL.RequestURI()
	rawRegex := regexp.MustCompile("^/raw")
	gitRegex := regexp.MustCompile(".*\\.git")
	if rawRegex.MatchString(requestURI) {
		return authenticateRawRequest(r, accessToken)
	} else if gitRegex.MatchString(requestURI) {
		return authenticateGitOverHTTP(r, accessToken)
	}
	return nil
}

func (h *GithubHandler) HandleReleaseAssets(w http.ResponseWriter, r *http.Request) (bool, error) {
	// https://github.build.ge.com/212595461/private_test/releases/download/1.0/private_test.tar.gz
	// $1 is the version, $2 is the asset name
	releaseRegex := regexp.MustCompile("/(?P<owner>[^/]+)/(?P<repo>[^/]+)/releases/download/(?P<tag>[^/]+)/(?P<assetName>.+)$")
	if releaseRegex.MatchString(r.RequestURI) {
		logger.Infof("retrieving release asset matching %s", r.RequestURI)
		ctx := context.Background()
		submatches := releaseRegex.FindStringSubmatch(r.RequestURI)
		owner := submatches[1]
		repo := submatches[2]
		tag := submatches[3]
		assetName := submatches[4]
		client, err := h.GithubClient()
		if err != nil {
			return false, errors.Wrap(err, "couldn't get GitHub client when trying to grab release assets")
		}
		logger.Infof("getting release for tag %s/%s/%s matching %s", owner, repo, tag, r.RequestURI)
		release, _, err := client.GetRepositories().GetReleaseByTag(ctx, owner, repo, tag)
		if err != nil {
			return false, errors.Wrap(err, "couldn't retrieve release for tag provided")
		}
		logger.Infof("INFO: getting assets for release release %s/%s/%d matching %s", owner, repo, release.GetID(), r.RequestURI)
		// if we have more than 50 release assets then idk what to tell you
		listOpts := &github.ListOptions{Page: 1, PerPage: 50}
		assets, _, err := client.GetRepositories().ListReleaseAssets(ctx, owner, repo, release.GetID(), listOpts)
		if err != nil {
			return false, errors.Wrap(err, "couldn't get list of assets for tag")
		}

		for _, asset := range assets {
			if asset.GetName() == assetName {
				logger.Infof("INFO: retrieving release %s/%s/%d matching %s", owner, repo, asset.GetID(), r.RequestURI)
				rc, redirectURL, err := client.GetRepositories().DownloadReleaseAsset(ctx, owner, repo, asset.GetID())
				if err != nil {
					return false, errors.Wrap(err, "couldn't download release asset")
				}
				if len(redirectURL) > 0 {
					http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
					return true, nil
				}
				io.Copy(w, rc)
				rc.Close()
				return true, nil
			}
		}
		return true, nil
	}
	return false, nil
}

func (h *GithubHandler) ProxyRequest(w http.ResponseWriter, r *http.Request) error {
	logger.Infof("INFO: Proxying request %s to: %s", r.RequestURI, h.GithubURL())
	err := h.ProbablyAuthenticate(r)
	if err != nil {
		return errors.Wrap(err, "encountered an error handling authentication of request for proxying")
	}
	path, err := url.Parse(h.GithubURL())
	if err != nil {
		return errors.Wrap(err, "couldn't parse github url when proxying request")
	}
	proxy := reverseproxy.NewReverseProxy(path)
	proxy.ErrorLog = log.New(os.Stdout, "", 0)

	proxy.ServeHTTP(w, r)
	logger.Infof("finished proxying request %s to: %s", r.RequestURI, h.GithubURL())
	return nil
}

func (h *GithubHandler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Infof("handling request: %s", r.RequestURI)

	didHandleReleaseAssets, err := h.HandleReleaseAssets(w, r)
	if err == nil && !didHandleReleaseAssets {
		err = h.ProxyRequest(w, r)
	}
	if err != nil {
		logger.Error(err)
	}
}

type stdoutLogger struct{}

func (s stdoutLogger) Error(v ...interface{}) error {
	log.Println(fmt.Sprintf("ERROR: %s", v...))
	return nil
}
func (s stdoutLogger) Warning(v ...interface{}) error {
	log.Println(fmt.Sprintf("WARNING: %s", v...))
	return nil
}
func (s stdoutLogger) Info(v ...interface{}) error {
	log.Println(fmt.Sprintf("INFO: %s", v...))
	return nil
}
func (s stdoutLogger) Errorf(format string, a ...interface{}) error {
	log.Println(fmt.Sprintf("ERROR: %s", fmt.Sprintf(format, a...)))
	return nil
}
func (s stdoutLogger) Warningf(format string, a ...interface{}) error {
	log.Println(fmt.Sprintf("WARNING: %s", fmt.Sprintf(format, a...)))
	return nil
}
func (s stdoutLogger) Infof(format string, a ...interface{}) error {
	log.Println(fmt.Sprintf("INFO: %s", fmt.Sprintf(format, a...)))
	return nil
}

type program struct {
	server *http.Server
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *program) run() {
	log.Println("INFO: Started Hubbard")
	handler := &GithubHandler{}
	p.server = &http.Server{
		Addr:    ":41968",
		Handler: http.HandlerFunc(handler.handleHTTP),
	}
	go func() {
		if err := p.server.ListenAndServe(); err != nil {
			// cannot panic, because this probably is an intentional close
			logger.Errorf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()
}
func (p *program) Stop(s service.Service) error {
	return p.server.Close()
}

func runService() {
	svcConfig := &service.Config{
		Name:        "Hubbard",
		DisplayName: "Hubbard",
		Description: "Reverse proxy for authenticated private GitHub integration",
	}
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}

func runServerInForeground() {
	log.Println("INFO: Started Hubbard")
	handler := &GithubHandler{}
	http.ListenAndServe(":41968", http.HandlerFunc(handler.handleHTTP))
}

func main() {
	runInForeground := len(os.Getenv("HUBBARD_FG")) > 0

	if runInForeground {
		logger = stdoutLogger{}
		log.SetOutput(os.Stdout)
		runServerInForeground()
	} else {
		runService()
	}
}
