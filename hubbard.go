package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/cssivision/reverseproxy"
	github "github.com/google/go-github/github"
	"github.com/kardianos/service"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	yaml "gopkg.in/yaml.v2"
)

var logger service.Logger

// GithubHandlerInterface will enable better testing in the future
type GithubHandlerInterface interface {
	GithubURL() string
	GithubAccessToken() (string, error)
	GithubClient() (*github.Client, error)
	ProbablyAuthenticate(r *http.Request) error
	GetRelease(ctx context.Context, owner string, repo string, tag string) (*github.RepositoryRelease, error)
	GetAssetList(ctx context.Context, owner string, repo string, release *github.RepositoryRelease) ([]*github.ReleaseAsset, error)
	DownloadReleaseAsset(ctx context.Context, owner string, repo string, asset *github.ReleaseAsset) (rc io.ReadCloser, redirectURL string, err error)
}

// GithubHandler is designed to manage interactions with GitHub
// it stores details about what GitHub instance to interact with, the correct
// credentials, and a memoized client.
type GithubHandler struct {
	githubURL         string
	githubAccessToken string
	githubClient      *github.Client
}

// GithubClient makes sure we have a client instance with the correct credentials
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

// GithubURL is a memoized accessor for retrieving the GitHub URL from the environment
func (h *GithubHandler) GithubURL() string {
	if len(h.githubURL) == 0 {
		gh := viper.Get("GITHUB_URL")
		if gh == nil {
			gh = "https://github.com"
		}
		h.githubURL = gh.(string)
	}
	return h.githubURL
}

// GithubAccessToken makes sure we have an access token, will throw an error if it can't find one
func (h *GithubHandler) GithubAccessToken() (string, error) {
	if len(h.githubAccessToken) == 0 {
		ghat := viper.Get("GITHUB_ACCESS_TOKEN")
		if ghat == nil {
			return "", errors.New("couldn't find an access token")
		}
		h.githubAccessToken = ghat.(string)
	}
	return h.githubAccessToken, nil
}

// Authenticating a raw request requires headers for access token and format
func authenticateRawRequest(r *http.Request, accessToken string) error {
	r.Header.Add("Authorization", fmt.Sprintf("token %s", accessToken))
	r.Header.Add("Accept", "application/vnd.github.v3.raw")
	return nil
}

// Authenticating over HTTP just requires setting basic auth correctly
func authenticateGitOverHTTP(r *http.Request, accessToken string) error {
	logger.Infof("adding basic auth to %s", r.RequestURI)
	r.SetBasicAuth(accessToken, "x-oauth-basic")
	return nil
}

// ProbablyAuthenticate will handle authentication for an HTTP request
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

// GetRelease will retrieve information about a release from the GitHub API
func (h *GithubHandler) GetRelease(ctx context.Context, owner string, repo string, tag string) (*github.RepositoryRelease, error) {
	client, err := h.GithubClient()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get GitHub client when trying to grab release assets")
	}

	release, _, err := client.GetRepositories().GetReleaseByTag(ctx, owner, repo, tag)
	return release, err
}

// GetAssetList will retrieve a list of repository release assets
func (h *GithubHandler) GetAssetList(ctx context.Context, owner string, repo string, release *github.RepositoryRelease) ([]*github.ReleaseAsset, error) {
	client, err := h.GithubClient()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get GitHub client when trying to grab asset list for release")
	}
	// if we have more than 50 release assets then idk what to tell you
	listOpts := &github.ListOptions{Page: 1, PerPage: 50}
	assets, _, err := client.GetRepositories().ListReleaseAssets(ctx, owner, repo, release.GetID(), listOpts)
	return assets, err
}

// DownloadReleaseAsset wraps calls to GitHub client for easier testing
func (h *GithubHandler) DownloadReleaseAsset(ctx context.Context, owner string, repo string, asset *github.ReleaseAsset) (rc io.ReadCloser, redirectURL string, err error) {
	client, err := h.GithubClient()
	if err != nil {
		return nil, "", errors.Wrap(err, "couldn't get GitHub client when trying to download release asset")
	}
	return client.GetRepositories().DownloadReleaseAsset(ctx, owner, repo, asset.GetID())
}

// HandleReleaseAssets will download an authenticated release asset using the GitHub API
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
		logger.Infof("getting release for tag %s/%s/%s matching %s", owner, repo, tag, r.RequestURI)
		release, err := h.GetRelease(ctx, owner, repo, tag)
		if err != nil {
			return false, errors.Wrap(err, "couldn't retrieve release for tag provided")
		}
		logger.Infof("getting assets for release release %s/%s/%d matching %s", owner, repo, release.GetID(), r.RequestURI)
		assets, err := h.GetAssetList(ctx, owner, repo, release)
		if err != nil {
			return false, errors.Wrap(err, "couldn't get list of assets for tag")
		}

		for _, asset := range assets {
			if asset.GetName() == assetName {
				logger.Infof("retrieving release %s/%s/%d matching %s", owner, repo, asset.GetID(), r.RequestURI)
				rc, redirectURL, err := h.DownloadReleaseAsset(ctx, owner, repo, asset)
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

// ProxyRequest will add authentication logic to a request and pass it to the correct GitHub URL
func (h *GithubHandler) ProxyRequest(w http.ResponseWriter, r *http.Request) error {
	logger.Infof("proxying request %s to: %s", r.RequestURI, h.GithubURL())
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

// HandleHTTP delegates retrieval of release assets or request proxying
func (h *GithubHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Infof("handling request: %s", r.RequestURI)

	didHandleReleaseAssets, err := h.HandleReleaseAssets(w, r)
	if err == nil && !didHandleReleaseAssets {
		err = h.ProxyRequest(w, r)
	}
	if err != nil {
		logger.Error(err)
	}
}

// StdoutLogger implements thte service.Logger interface
type StdoutLogger struct{}

// Error logs an error
func (s StdoutLogger) Error(v ...interface{}) error {
	log.Println(fmt.Sprintf("ERROR: %s", v...))
	return nil
}

// Warning logs a warning
func (s StdoutLogger) Warning(v ...interface{}) error {
	log.Println(fmt.Sprintf("WARNING: %s", v...))
	return nil
}

// Info logs at info level
func (s StdoutLogger) Info(v ...interface{}) error {
	log.Println(fmt.Sprintf("INFO: %s", v...))
	return nil
}

// Errorf logs an error with format
func (s StdoutLogger) Errorf(format string, a ...interface{}) error {
	log.Println(fmt.Sprintf("ERROR: %s", fmt.Sprintf(format, a...)))
	return nil
}

// Warningf logs a warning with format
func (s StdoutLogger) Warningf(format string, a ...interface{}) error {
	log.Println(fmt.Sprintf("WARNING: %s", fmt.Sprintf(format, a...)))
	return nil
}

// Infof logs to info with format
func (s StdoutLogger) Infof(format string, a ...interface{}) error {
	log.Println(fmt.Sprintf("INFO: %s", fmt.Sprintf(format, a...)))
	return nil
}

// ProxyService lets us handle the proxy interaction as a service
type ProxyService struct {
	server *http.Server
}

// Start will start the http server running
func (p *ProxyService) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

// logic for running the server in the background
func (p *ProxyService) run() {
	logger.Info("started Hubbard")
	handler := &GithubHandler{}
	p.server = &http.Server{
		Addr:    ":41968",
		Handler: http.HandlerFunc(handler.HandleHTTP),
	}
	go func() {
		if err := p.server.ListenAndServe(); err != nil {
			// cannot panic, because this probably is an intentional close
			logger.Errorf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()
}

// Stop will let us stop the program by closing the server
func (p *ProxyService) Stop(s service.Service) error {
	return p.server.Close()
}

func runService() {
	svcConfig := &service.Config{
		Name:        "Hubbard",
		DisplayName: "Hubbard",
		Description: "Reverse proxy for authenticated private GitHub integration",
	}
	prg := &ProxyService{}
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
	logger.Info("Started Hubbard")
	handler := &GithubHandler{}
	http.ListenAndServe(":41968", http.HandlerFunc(handler.HandleHTTP))
}

func main() {
	RootCmd.AddCommand(RunFgCmd)
	ConfigureCmd.Flags().String("github-url", "", "URL of GitHub API")
	ConfigureCmd.Flags().String("github-access-token", "", "The access token for the GitHub API")
	RootCmd.AddCommand(ConfigureCmd)
	RootCmd.Execute()
}

func initConfig() {
	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	viper.SetConfigType("yaml")
	viper.AddConfigPath(home)
	viper.AddConfigPath("/etc/hubbard")
	viper.SetConfigName(".hubbard")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println(err)
		viper.AutomaticEnv()
	}
}

// RunFgCmd lets us run in the foreground
var RunFgCmd = &cobra.Command{
	Use:   "run-fg",
	Short: "Runs hubbard in the foreground",
	Run: func(cmd *cobra.Command, args []string) {
		initConfig()
		logger = StdoutLogger{}
		log.SetOutput(os.Stdout)
		runServerInForeground()
	},
}

type HubbardConfig struct {
	GithubURL         string `yaml:"GITHUB_URL"`
	GithubAccessToken string `yaml:"GITHUB_ACCESS_TOKEN"`
}

// ConfigureCmd lets us configure hubbard
var ConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "configures hubard",
	Run: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("github-url", cmd.Flags().Lookup("github-url"))
		viper.BindPFlag("github-access-token", cmd.Flags().Lookup("github-access-token"))
		url := viper.GetString("github-url")
		if len(url) == 0 {
			panic("Need to set flag --github-url!")
		}

		token := viper.GetString("github-access-token")
		if len(token) == 0 {
			panic("need to set flag --github-access-token!")
		}
		bytes, err := yaml.Marshal(&HubbardConfig{
			GithubURL:         url,
			GithubAccessToken: token,
		})
		if err != nil {
			panic(err)
		}
		ioutil.WriteFile("/etc/hubbard/.hubbard.yml", bytes, 0644)
	},
}

// RootCmd runs the server
var RootCmd = &cobra.Command{
	Use:   "Hubbard",
	Short: "Hubbard is a proxy for handling authenticated interactions with a GitHub server",
	Long:  `https://github.build.ge.com/SECC/hubbard`,
	Run: func(cmd *cobra.Command, args []string) {
		runService()
	},
}
