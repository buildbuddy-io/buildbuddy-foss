package testgit

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/tables"
	"github.com/buildbuddy-io/buildbuddy/server/testutil/testfs"
	"github.com/buildbuddy-io/buildbuddy/server/testutil/testshell"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
)

const (
	// FakeWebhookID is the fake webhook ID returned by the provider when
	// creating a webhook.
	FakeWebhookID = "fake-webhook-id"
)

type Status struct {
	AccessToken string
	RepoURL     string
	CommitSHA   string
	Payload     any
}

// FakeProvider implements the git provider interface for tests.
type FakeProvider struct {
	// Captured values

	RegisteredWebhookURL  string
	UnregisteredWebhookID string
	Statuses              chan *Status

	// Faked values

	RegisterWebhookError error
	WebhookData          *interfaces.WebhookData
	FileContents         map[string]string
	TrustedUsers         []string
}

func NewFakeProvider() *FakeProvider {
	return &FakeProvider{
		Statuses: make(chan *Status, 1024),
	}
}

func (p *FakeProvider) MatchRepoURL(u *url.URL) bool {
	return true
}
func (p *FakeProvider) MatchWebhookRequest(req *http.Request) bool {
	return true
}
func (p *FakeProvider) ParseWebhookData(req *http.Request) (*interfaces.WebhookData, error) {
	if p.WebhookData == nil {
		return nil, status.UnimplementedError("Not implemented")
	}
	return p.WebhookData, nil
}
func (p *FakeProvider) RegisterWebhook(ctx context.Context, accessToken, repoURL, webhookURL string) (string, error) {
	if p.RegisterWebhookError != nil {
		return "", p.RegisterWebhookError
	}
	p.RegisteredWebhookURL = webhookURL
	return FakeWebhookID, nil
}
func (p *FakeProvider) UnregisterWebhook(ctx context.Context, accessToken, repoURL, webhookID string) error {
	p.UnregisteredWebhookID = webhookID
	return nil
}
func (p *FakeProvider) GetFileContents(ctx context.Context, accessToken, repoURL, filePath, ref string) ([]byte, error) {
	contents, ok := p.FileContents[filePath]
	if !ok {
		return nil, status.NotFoundError("Not found")
	}
	return []byte(contents), nil
}
func (p *FakeProvider) IsTrusted(ctx context.Context, accessToken, repoURL, user string) (bool, error) {
	for _, u := range p.TrustedUsers {
		if u == user {
			return true, nil
		}
	}
	return false, nil
}
func (p *FakeProvider) CreateStatus(ctx context.Context, accessToken, repoURL, commitSHA string, payload any) error {
	p.Statuses <- &Status{accessToken, repoURL, commitSHA, payload}
	return nil
}

// MakeTempRepo initializes a Git repository with the given file contents, and
// creates an initial commit of those files. Contents are specified as a map of
// file paths to file contents. Parent directories are created automatically.
// The repository contents are automatically deleted in the test cleanup phase.
// Returns the path to the repo and the SHA of the initial commit.
func MakeTempRepo(t testing.TB, contents map[string]string) (path, commitSHA string) {
	path = testfs.MakeTempDir(t)
	testfs.WriteAllFileContents(t, path, contents)
	for fileRelPath := range contents {
		filePath := filepath.Join(path, fileRelPath)
		// Make shell scripts executable.
		if strings.HasSuffix(filePath, ".sh") {
			if err := os.Chmod(filePath, 0750); err != nil {
				t.Fatal(err)
			}
		}
	}
	Init(t, path)
	headCommitSHA := strings.TrimSpace(testshell.Run(t, path, `git rev-parse HEAD`))
	return path, headCommitSHA
}

// Init takes a directory which does not already contain a .git dir,
// and initializes the directory with an initial commit of all the existing
// files.
func Init(t testing.TB, dir string) string {
	testshell.Run(t, dir, `git -c init.defaultBranch=master init`)
	configure(t, dir)
	testshell.Run(t, dir, `git add . && git commit -m "Initial commit"`)
	return strings.TrimSpace(testshell.Run(t, dir, `git rev-parse HEAD`))
}

// CommitAll adds and commits all changes in the given git repo path,
// returning the SHA of the new commit.
func CommitAll(t testing.TB, dir, message string) string {
	testshell.Run(t, dir, fmt.Sprintf(`git add . && git commit -m %q`, message))
	commitSHA := strings.TrimSpace(testshell.Run(t, dir, "git rev-parse HEAD"))
	return commitSHA
}

func ConfigureRemoteOrigin(t testing.TB, dir, url string) {
	testshell.Run(t, dir, `git remote add origin `+url)
}

func CurrentBranch(t testing.TB, dir string) string {
	output := testshell.Run(t, dir, `git rev-parse --abbrev-ref HEAD`)
	return strings.TrimSpace(output)
}

func CurrentCommitSHA(t testing.TB, dir string) string {
	output := testshell.Run(t, dir, `git rev-parse HEAD`)
	return strings.TrimSpace(output)
}

// MakeTempRepoClone makes a clone of the git repo at the given path, and cleans
// up the copy after the test is complete.
func MakeTempRepoClone(t testing.TB, path string) string {
	copyPath := testfs.MakeTempDir(t)
	testshell.Run(t, copyPath, fmt.Sprintf(`git clone file://%q .`, path))
	configure(t, copyPath)
	return copyPath
}

// CommitFiles writes the given file contents and creates a new commit with the changes.
// Contents are specified as a map of file path to file contents.
// Returns the sha of the new commit
func CommitFiles(t testing.TB, repoPath string, contents map[string]string) string {
	testfs.WriteAllFileContents(t, repoPath, contents)
	testshell.Run(t, repoPath, `git add . && git commit -m "Initial commit"`)
	commitSHA := strings.TrimSpace(testshell.Run(t, repoPath, `git rev-parse HEAD`))
	return commitSHA
}

func configure(t testing.TB, repoPath string) {
	testshell.Run(t, repoPath, `
		git config user.name "Test"
		git config user.email "test@buildbuddy.io"
	`)
}

// FakeGitHubApp implements the github app interface for tests.
type FakeGitHubApp struct {
	interfaces.GitHubApp
	Token     string
	MockAppID int64
}

func (a *FakeGitHubApp) GetRepositoryInstallationToken(ctx context.Context, repo *tables.GitRepository) (string, error) {
	return a.Token, nil
}

func (a *FakeGitHubApp) AppID() int64 {
	return a.MockAppID
}
