# internal/tui - Interactive TUI for Chatwoot

Terminal UI implementation using Bubbletea framework with multi-pane layout for browsing conversations, viewing messages, and managing statuses.

## Architecture

**3-Column Layout:**
- **Conversations pane** (left) — filterable list of conversations with assignee/status tabs
- **Messages pane** (middle) — chat bubbles with message history and scroll support
- **Info pane** (right) — conversation metadata (contact, status, assignee, labels)

**Overlay Modals:**
- **Reply editor** (Shift+R) — compose and send messages to conversations
- **Private note editor** (Shift+P) — add internal notes with yellow accent
- **Command palette** (Ctrl+K) — fuzzy-searchable actions for status changes, refresh, quit

## Core Files

### tui.go
Main Bubbletea model and update/view logic. Handles:
- State management (conversations, messages, info pane, overlays)
- Message routing (KeyMsg → handleKey → pane-specific handlers)
- Layout computation (header, body columns, footer)
- Overlay centering and rendering

### messages.go
Message pane component. Handles:
- Message rendering (chat bubbles with sender, timestamp, status icons)
- Scroll logic (up/down with bounds checking)
- Line counting for scroll bounds (accounts for multi-line wrapped content)
- Placeholder with Chatwoot logo when no messages loaded

### conversations.go
Conversation list component. Handles:
- Conversation rows (single-line format with ID, name, timestamp)
- Tab filtering (Mine/Unassigned/All via assignee_type)
- Status filtering (Open/Resolved/Pending/Snoozed)
- Text-based search filtering
- Selection highlight

### reply.go
Modal editor for composing messages and private notes. Handles:
- Textarea with focus and cursor blinking
- Mode toggle (reply vs. private note) — changes header text and border color
- Send state (displays "Sending..." during API call)
- View renders centered floating box

### palette.go
Command palette with fuzzy search. Handles:
- Status actions (Reopen, Mark as resolved, Mark as pending, Snooze variants)
- App actions (Open in browser, Refresh data, Quit)
- Fuzzy filtering by action name
- Cursor navigation and selection
- Dynamic action set based on current conversation status

### styles.go
Color definitions and style builders. Provides:
- Adaptive colors (light/dark terminal support)
- Status colors (open=green, resolved=blue, pending=orange, snoozed=gray)
- Pane border styles (active vs. inactive)
- Logo string embedded from `logo.txt`
- Status dot rendering function

### keys.go
Key bindings and help text. Defines:
- Arrow keys, Tab, / for navigation/filtering
- R = reply, P = private note
- Ctrl+K = command palette
- o = open in browser, r = refresh, q = quit
- Help text with styled keys (bright) and labels (muted)

### fetch.go
Async command builders for API calls. Provides:
- fetchConversations() — filters by assignee_type and status
- fetchMessages() — loads messages for selected conversation
- fetchContact() — fetches full contact details (location, IP)
- sendMessage() — posts message or private note
- toggleStatus() — changes conversation status with optional snooze timestamp
- autoRefreshTick() — 30-second auto-refresh timer

### conversations.go (list component)
Not a full SDK service — local state management for the conversation list pane.

## Message Flow

1. **Startup** → `tui.Run()` creates Model, launches Bubbletea program
2. **Init** → fetchConversations() triggered
3. **Update loop**:
   - KeyMsg → handleKey() → route to pane handler or global key handler
   - conversationsMsg → update list, fetch contact for selected
   - messagesMsg → load into message pane, scroll to bottom
   - contactMsg → cache contact details for info pane
   - replyMsg → reload messages after send
   - toggleStatusMsg → refresh conversation list
4. **View** → render header + 3-column body + footer, overlay modals if active

## Layout Math

- Terminal width = W, height = H
- Header: barContentW = W - 2 (lipgloss border adds 2 visual)
- Body height: bodyH = H - 8 (header 3 + footer 3 + borders 2)
- Columns: convW = 40, msgW = remaining, infoW = 35 (if space)
- All panes set via SetSize() in WindowSizeMsg handler (persists across renders)

## State Management

Model struct holds:
- `convList`, `msgPane` — pane components with internal state
- `reply`, `palette` — overlay components (active/inactive flags)
- `activePane` — routes keys (0=conversations, 1=messages)
- `contact` — cached contact for selected conversation
- `loading`, `err` — spinner state and error display

Key invariant: `View()` runs on value receiver copy, so state updates must happen in `Update()` or `Init()`, not during render.

## Color Scheme

- **Accent** (blue): `#1a73e8` (light), `#8ab4f8` (dark)
- **Muted** (gray): `#666666` (light), `#888888` (dark)
- **Open** (green), **Resolved** (blue), **Pending** (orange), **Snoozed** (gray)
- **Private note** (yellow/amber): `#b5851e` (light), `#8a6d3b` (dark)
- **Selected** (light blue): `#e8f0fe` (light), `#1e3a5f` (dark)

## TODO

- Paginate messages using beforeID for older messages on scroll
- Implement message reactions/emoji picker
- Add keyboard shortcut for marking conversations as spam
- Multi-select conversations for bulk actions
