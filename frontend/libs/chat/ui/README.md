# Chat UI Structure

- `thread/` - top-level assistant thread layout: viewport, welcome state, scroll-to-bottom.
- `composer/` - message input, send/cancel controls, edit composer.
- `messages/` - user and assistant message shells.
- `actions/` - message action bars: copy, reload, edit, branch navigation.
- `parts/` - rendering for assistant message parts: text, reasoning, tool calls, indicators.
- `attachments/` - composer and message attachment previews.
- `markdown/` - markdown rendering for assistant text.
- `reasoning/` - reasoning disclosure UI.
- `tools/` - tool call grouping and fallback rendering.
- `common/` - small shared chat-only primitives.
