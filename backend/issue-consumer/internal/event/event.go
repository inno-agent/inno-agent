package event

import (
	"encoding/json"
	"strings"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/gitflame"
)

type Envelope struct {
	DeliveryID string          `json:"delivery_id"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
}

type IssueEvent struct {
	Action     string       `json:"action"`
	Number     int64        `json:"number"`
	Index      int64        `json:"index"`
	Issue      issueDetails `json:"issue"`
	Repository repoDetails  `json:"repository"`
	Sender     userLogin    `json:"sender"`
	Assignee   userLogin    `json:"assignee"`
}

// IssueIndex returns the issue number, preferring nested issue fields.
func (e IssueEvent) IssueIndex() int64 {
	if e.Issue.Index != 0 {
		return e.Issue.Index
	}
	if e.Issue.Number != 0 {
		return e.Issue.Number
	}
	if e.Index != 0 {
		return e.Index
	}
	return e.Number
}

func (e IssueEvent) IssueBody() string {
	return gitflame.ParseIssueBody(e.Issue.Body)
}

func (e IssueEvent) RepoOwner() string {
	if login := e.Repository.Owner.Login; login != "" {
		return login
	}
	if e.Repository.FullName == "" {
		return ""
	}
	parts := strings.SplitN(e.Repository.FullName, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

func (e IssueEvent) RepoName() string {
	if e.Repository.Name != "" {
		return e.Repository.Name
	}
	if e.Repository.FullName == "" {
		return ""
	}
	parts := strings.SplitN(e.Repository.FullName, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

type issueDetails struct {
	Number    int64           `json:"number"`
	Index     int64           `json:"index"`
	Title     string          `json:"title"`
	Body      json.RawMessage `json:"body"`
	Assignee  userLogin       `json:"assignee"`
	Assignees []userLogin     `json:"assignees"`
	Labels    []Label         `json:"labels"`
}

type Label struct {
	Name string `json:"name"`
}

type repoDetails struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	DefaultBranch string    `json:"default_branch"`
	Owner         repoOwner `json:"owner"`
}

type repoOwner struct {
	Login string `json:"login"`
}

type userLogin struct {
	Login    string `json:"login"`
	Username string `json:"username"`
}

func (u userLogin) Name() string {
	if u.Login != "" {
		return u.Login
	}
	return u.Username
}
