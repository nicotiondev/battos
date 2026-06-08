-- name: CreateNovaConversation :one
INSERT INTO novacore_conversations (id, user_id)
VALUES (lower(hex(randomblob(16))), ?)
RETURNING id, user_id, started_at, ended_at, message_count, total_input_tokens, total_output_tokens, total_cost_usd;

-- name: GetNovaConversation :one
SELECT id, user_id, started_at, ended_at, message_count, total_input_tokens, total_output_tokens, total_cost_usd
FROM novacore_conversations
WHERE id = ?;

-- name: ListNovaConversations :many
SELECT id, user_id, started_at, ended_at, message_count, total_input_tokens, total_output_tokens, total_cost_usd
FROM novacore_conversations
ORDER BY started_at DESC;

-- name: CreateNovaMessage :one
INSERT INTO novacore_messages (id, conversation_id, role, content, tool_calls, tool_result, tokens_in, tokens_out)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?, ?)
RETURNING id, conversation_id, role, content, tool_calls, tool_result, tokens_in, tokens_out, created_at;

-- name: ListNovaMessagesByConversation :many
SELECT id, conversation_id, role, content, tool_calls, tool_result, tokens_in, tokens_out, created_at
FROM novacore_messages
WHERE conversation_id = ?
ORDER BY created_at ASC;

-- name: UpdateNovaConversationStats :one
UPDATE novacore_conversations
SET message_count = message_count + 1,
    total_input_tokens = total_input_tokens + ?,
    total_output_tokens = total_output_tokens + ?,
    total_cost_usd = total_cost_usd + ?
WHERE id = ?
RETURNING id, user_id, started_at, ended_at, message_count, total_input_tokens, total_output_tokens, total_cost_usd;
