package event

import "encoding/json"

type Envelope struct {
	DeliveryID string          `json:"delivery_id"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
}

type IssueEvent struct {
	Action     string       `json:"action"`
	Number     int64        `json:"number"`
	Issue      issueDetails `json:"issue"`
	Repository repoDetails  `json:"repository"`
	Sender     userLogin    `json:"sender"`
	Assignee   userLogin    `json:"assignee"`
}

func (e IssueEvent) Index() int64 {
	if e.Issue.Number != 0 {
		return e.Issue.Number
	}
	return e.Number
}

type issueDetails struct {
	Number    int64       `json:"number"`
	Title     string      `json:"title"`
	Body      string      `json:"body"`
	Assignee  userLogin   `json:"assignee"`
	Assignees []userLogin `json:"assignees"`
	Labels    []Label     `json:"labels"`
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
	Login string `json:"login"`
}
