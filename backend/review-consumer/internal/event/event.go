package event

import "encoding/json"

type Envelope struct {
	DeliveryID string          `json:"delivery_id"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
}

type PullRequestEvent struct {
	Action            string            `json:"action"`
	Number            int64             `json:"number"`
	PullRequest       prDetails         `json:"pull_request"`
	Repository        repoDetails       `json:"repository"`
	RequestedReviewer requestedReviewer `json:"requested_reviewer"`
	Sender            sender            `json:"sender"`
}

// Index returns the PR number, preferring the nested pull_request.number
// (which some GitFlame webhook payloads populate) and falling back to the
// top-level number field.
func (e PullRequestEvent) Index() int64 {
	if e.PullRequest.Number != 0 {
		return e.PullRequest.Number
	}

	return e.Number
}

type prDetails struct {
	Number int64  `json:"number"`
	Head   prHead `json:"head"`
}

type prHead struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type repoDetails struct {
	Name     string    `json:"name"`
	FullName string    `json:"full_name"`
	Owner    repoOwner `json:"owner"`
}

type repoOwner struct {
	Login string `json:"login"`
}

type requestedReviewer struct {
	Login string `json:"login"`
}

type sender struct {
	Login string `json:"login"`
}
