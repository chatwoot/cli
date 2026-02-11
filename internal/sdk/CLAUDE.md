# internal/sdk - Chatwoot API Client

HTTP client and service layer for read-only Chatwoot API access. Built with a factory pattern — each service (conversations, messages, contacts, etc.) is accessed via a method on the `Client` struct.

## Core Files

### client.go
HTTP client and base request/response handling. Provides:
- `Client` struct with BaseURL, AccountID, token
- Request building (GET, POST, PATCH with auth headers)
- Response parsing and error handling
- Factory methods: `Conversations()`, `Messages()`, `Contacts()`, etc.
- `GetRaw()` for non-account-scoped endpoints (e.g., `/api/v1/profile`)

### conversations.go
Conversation list and status management.
- `Conversation` struct with ID, status, assignee_type, labels, timestamps
- `ConversationsService.List()` — filtered by assignee_type, status, page
- `ConversationsService.Get()` — single conversation details
- `ConversationsService.ToggleStatus()` — change status with optional snooze timestamp
- API query: `GET /accounts/{id}/conversations` with filters
- API query: `POST /accounts/{id}/conversations/{id}/toggle_status`

### messages.go
Message history and compose.
- `Message` struct with ID, content, status, message_type, created_at, sender
- `MessagesService.List()` — conversation messages with pagination (beforeID)
- `MessagesService.Create()` — post message or private note
- Message types: 0=incoming, 1=outgoing, 2=activity
- Status values: "sent", "delivered", "read", "failed"
- API query: `GET /accounts/{id}/conversations/{id}/messages`
- API mutation: `POST /accounts/{id}/conversations/{id}/messages`

### contacts.go
Contact/customer data.
- `Contact` struct (basic fields: name, email, phone)
- `ContactFull` struct (extended with avatar, additional_attributes, etc.)
- `ContactsService.List()` — paginated contact list
- `ContactsService.Get()` — full contact details by ID
- Additional attributes include location (city, country), IP (created_at_ip), browser info
- API query: `GET /accounts/{id}/contacts`
- API query: `GET /accounts/{id}/contacts/{id}`

### labels.go
Conversation labels/tags.
- `Label` struct with ID, name, color
- `LabelsService.List()` — all labels for account
- Label colors used for visual organization in UI
- API query: `GET /accounts/{id}/labels`

### agents.go
Team member list.
- `Agent` struct with ID, name, email, availability_status
- `AgentsService.List()` — returns raw `[]Agent` array (not wrapped)
- Availability: "online", "offline", "away"
- API query: `GET /accounts/{id}/agents`

### inboxes.go
Inbox configuration.
- `Inbox` struct with ID, name, channel_type
- `InboxesService.List()` — inboxes for account
- Channel types: "Channel::Email", "Channel::WebWidget", etc.
- API query: `GET /accounts/{id}/inboxes`

### profile.go
Authenticated user profile.
- `Profile` struct with agent info, availability_status
- `ProfileService.Get()` — current user details
- Non-account-scoped endpoint: `GET /api/v1/profile`
- Accessed via `client.GetRaw()`

## API Patterns

**Pagination:**
- `List()` methods accept `page` parameter (0-indexed)
- Response includes `meta` struct with `current_page`, `page_count`, etc.
- For message history, use `beforeID` to load older messages

**Filtering:**
- Conversations: `assignee_type` ("me", "unassigned", all), `status` (open/resolved/pending/snoozed)
- Sorting: `sort_by=last_activity_at_desc` for reverse chronological
- Search: text-based filtering via `/search` endpoint (not yet implemented)

**Quirks:**
- Contacts list meta `current_page` returns **string**, not int
- Messages list meta `agent_last_seen_at` can be string
- Single contact GET wraps in `{payload: {contact}}` (payload field)
- Agents list returns **bare array**, not wrapped in payload
- Profile endpoint is `/api/v1/profile` (account-scoped), accessed via `GetRaw()`

## Request/Response Structure

**Request:**
```json
{
  "message": {
    "content": "Hello",
    "private": false
  }
}
```

**Success Response:**
```json
{
  "payload": {
    "id": 123,
    "content": "Hello",
    ...
  }
}
```

**Error Response:**
```json
{
  "error": "Unauthorized"
}
```

## Authentication

- Token via environment variable or config file
- Injected as `X-API-Token` header on all requests
- Token must have `conversation:read`, `message:read`, `message:write` scopes

## File Organization

- `client.go` — core HTTP client and factory
- `conversations.go`, `messages.go`, `contacts.go`, etc. — service implementations
- Each service in its own file for clarity and testability
- No authentication logic — delegated to Client

## TODO

- Implement `/search` endpoint for message full-text search
- Add support for conversation custom attributes
- Implement webhooks for real-time updates
- Add bulk action endpoints (mark multiple as read, etc.)
