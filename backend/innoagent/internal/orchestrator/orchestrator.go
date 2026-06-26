package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"innoagent/internal/catalog"
	"innoagent/internal/llm"
)

// RouteInfo describes a model route the router can choose from.
type RouteInfo struct {
	Name        string // model ID (used as route name)
	Description string // what this model handles
}

type AIOrchestrator struct {
	provider       llm.Provider
	routerProvider llm.Provider
	routes         []RouteInfo
	models         []string
}

func New(provider llm.Provider, routerProvider llm.Provider, routes []RouteInfo, models []string) *AIOrchestrator {
	return &AIOrchestrator{
		provider:       provider,
		routerProvider: routerProvider,
		routes:         routes,
		models:         models,
	}
}

func (o *AIOrchestrator) route(ctx context.Context, messages []llm.Message) string {
	if len(messages) == 0 {
		return o.models[0]
	}

	routesJSON, err := json.Marshal(o.routes)
	if err != nil {
		log.Printf("auto: failed to marshal routes: %v, falling back to %s", err, o.models[0])
		return o.models[0]
	}

	// Collect user messages for the <conversation> section.
	conversation := make([]llm.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == "user" {
			conversation = append(conversation, m)
		}
	}
	if len(conversation) == 0 {
		conversation = append(conversation, messages[len(messages)-1])
	}

	conversationJSON, err := json.Marshal(conversation)
	if err != nil {
		log.Printf("auto: failed to marshal conversation: %v, falling back to %s", err, o.models[0])
		return o.models[0]
	}

	prompt := fmt.Sprintf(
		"<routes>\n%s\n</routes>\n\n<conversation>\n%s\n</conversation>\n\nOutput the route name in JSON format: {\"route\": \"route_name\"}",
		string(routesJSON),
		string(conversationJSON),
	)

	routerMessages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	response, err := o.routerProvider.Chat(ctx, routerMessages, "")
	if err != nil {
		log.Printf("auto: router call failed: %v, falling back to %s", err, o.models[0])
		return o.models[0]
	}

	var routeResp struct {
		Route string `json:"route"`
	}
	trimmed := strings.TrimSpace(response)
	if err := json.Unmarshal([]byte(trimmed), &routeResp); err != nil {
		// arch-router may emit single-quoted JSON like {'route': 'model'}.
		// Only normalize on failure so valid JSON containing apostrophes is
		// never corrupted.
		normalized := strings.ReplaceAll(trimmed, "'", "\"")
		if err := json.Unmarshal([]byte(normalized), &routeResp); err != nil {
			log.Printf("auto: router returned non-JSON %q, falling back to %s", response, o.models[0])
			return o.models[0]
		}
	}

	chosen := routeResp.Route
	for _, m := range o.models {
		if m == chosen {
			log.Printf("auto: routed to %s", chosen)
			return chosen
		}
	}

	log.Printf("auto: router returned unknown route %q, falling back to %s", chosen, o.models[0])
	return o.models[0]
}

// resolveModel maps the requested model name to a concrete model ID.
// "auto" triggers routing; "" and any unresolvable case fall back to the
// first concrete model from LLM_MODELS — never back to "auto", avoiding
// infinite routing loops.
func (o *AIOrchestrator) resolveModel(ctx context.Context, messages []llm.Message, modelName string) string {
	if modelName == catalog.AutoID {
		return o.route(ctx, messages)
	}
	if modelName == "" {
		return o.models[0]
	}
	return modelName
}

func (o *AIOrchestrator) Ask(ctx context.Context, messages []llm.Message, modelName string) (string, error) {
	resolved := o.resolveModel(ctx, messages, modelName)
	return o.provider.Chat(ctx, messages, resolved)
}

func (o *AIOrchestrator) AskStream(ctx context.Context, messages []llm.Message, modelName string) (<-chan string, error) {
	resolved := o.resolveModel(ctx, messages, modelName)
	return o.provider.Stream(ctx, messages, resolved)
}
