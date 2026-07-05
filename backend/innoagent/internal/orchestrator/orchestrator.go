package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"innoagent/internal/catalog"
	"innoagent/internal/llm"

	"go.uber.org/zap"
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
	logger         *zap.Logger
}

func New(provider llm.Provider, routerProvider llm.Provider, routes []RouteInfo, models []string, logger *zap.Logger) *AIOrchestrator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AIOrchestrator{
		provider:       provider,
		routerProvider: routerProvider,
		routes:         routes,
		models:         models,
		logger:         logger,
	}
}

func (o *AIOrchestrator) route(ctx context.Context, messages []llm.Message) string {
	if len(messages) == 0 {
		return o.models[0]
	}

	routesJSON, err := json.Marshal(o.routes)
	if err != nil {
		o.logger.Warn("auto route fallback: failed to marshal routes", zap.Error(err), zap.String("fallback_model", o.models[0]))
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
		o.logger.Warn("auto route fallback: failed to marshal conversation", zap.Error(err), zap.String("fallback_model", o.models[0]))
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
		o.logger.Warn("auto route fallback: router call failed", zap.Error(err), zap.String("fallback_model", o.models[0]))
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
			o.logger.Warn("auto route fallback: router returned non-json", zap.String("response", response), zap.String("fallback_model", o.models[0]))
			return o.models[0]
		}
	}

	chosen := routeResp.Route
	for _, m := range o.models {
		if m == chosen {
			o.logger.Info("auto routed", zap.String("model", chosen))
			return chosen
		}
	}

	o.logger.Warn("auto route fallback: router returned unknown route", zap.String("route", chosen), zap.String("fallback_model", o.models[0]))
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
