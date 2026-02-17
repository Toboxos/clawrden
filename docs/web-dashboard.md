# Web Dashboard

The Clawrden web dashboard provides a real-time interface for monitoring and controlling the warden supervisor.

## Features

### 📊 Status Overview
- **Pending Approvals**: Live count of requests awaiting human approval
- **Commands Today**: Total commands executed today
- **Uptime**: Current warden uptime
- **Status Badge**: Visual indicator of warden health

### ✅ HITL Approval Interface
- **Real-time Queue**: Automatically refreshes every 2 seconds
- **One-Click Actions**: Approve or deny with a single click
- **Request Details**: View command, arguments, working directory, and user ID
- **Auto-refresh Toggle**: Enable/disable automatic updates

### 📜 Command History
- **Recent Commands**: Last 20 commands executed
- **Full Audit Trail**: Complete history with timestamps
- **Decision Tracking**: See which commands were allowed/denied/approved
- **Performance Metrics**: Exit codes and execution duration

## Accessing the Dashboard

### Local Development
```bash
# Start the warden
./bin/clawrden-warden --api :8080

# Open in browser
open http://localhost:8080
```

### Custom Port
```bash
# Use a different port
./bin/clawrden-warden --api :3000

# Access at
open http://localhost:3000
```

### Remote Access
```bash
# Bind to all interfaces (WARNING: no authentication!)
./bin/clawrden-warden --api 0.0.0.0:8080

# Access from another machine
open http://<server-ip>:8080
```

## User Interface

### Layout

```
┌─────────────────────────────────────────────────┐
│  🛡️ Clawrden              [Running]            │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐     │
│  │ Pending  │  │ Commands │  │ Uptime   │     │
│  │    2     │  │   42     │  │  4h 23m  │     │
│  └──────────┘  └──────────┘  └──────────┘     │
│                                                 │
├─────────────────────────────────────────────────┤
│  Pending Approvals         [Auto] [Refresh]    │
├─────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────┐   │
│  │ npm install express                     │   │
│  │ Path: /app    User: uid:1000           │   │
│  │ [✓ Approve]  [✗ Deny]                   │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
├─────────────────────────────────────────────────┤
│  Recent Commands                   [Refresh]    │
├─────────────────────────────────────────────────┤
│  Time     Command       Decision  Exit  Dur    │
│  14:23:45 echo hello    allow     0     12ms   │
│  14:23:47 npm install   allow     0     5432ms │
│  14:23:50 sudo rm -rf   deny      -     -      │
└─────────────────────────────────────────────────┘
```

### Color Scheme

The dashboard uses a dark theme optimized for extended viewing:

- **Background**: Dark slate (#0f172a)
- **Cards**: Lighter slate (#1e293b)
- **Accent Colors**:
  - Blue: Actions and links (#3b82f6)
  - Green: Success and approvals (#10b981)
  - Red: Denials and errors (#ef4444)
  - Yellow: Warnings and pending (#f59e0b)

## Workflow

### Approving a Request

1. Request appears in "Pending Approvals" section
2. Review command details:
   - Command and arguments
   - Working directory
   - User ID (UID/GID)
   - Request ID
3. Click **✓ Approve** to allow execution
4. Request disappears from queue
5. Command executes immediately
6. Result appears in history

### Denying a Request

1. Request appears in "Pending Approvals" section
2. Review command details
3. Click **✗ Deny** to reject execution
4. Request disappears from queue
5. Agent receives exit code 1
6. Denial logged in audit history

### Monitoring History

The Recent Commands table shows:
- **Time**: When the command was executed
- **Command**: Full command with arguments
- **Decision**: Policy decision (allow/deny/ask)
- **Exit**: Command exit code
- **Duration**: Execution time in milliseconds

**Decision Badges:**
- 🟢 **allow**: Automatically approved by policy
- 🔴 **deny**: Automatically rejected by policy
- 🟡 **ask**: Required human approval

## Auto-Refresh

The dashboard automatically polls the API every 2 seconds for:
- Pending request count
- Queue updates
- Status changes

**Toggle auto-refresh:**
- Click the switch next to "Auto-refresh"
- When disabled, use **Refresh** buttons to update manually

## API Integration

The dashboard uses the following API endpoints:

```javascript
GET  /api/status              // Warden health
GET  /api/queue               // Pending requests
POST /api/queue/:id/approve   // Approve request
POST /api/queue/:id/deny      // Deny request
GET  /api/history             // Audit log
```

All responses are JSON. See [API Reference](../README.md#http-api-reference) for details.

## Customization

The dashboard is a single HTML file with embedded CSS and JavaScript:

**Location:** `internal/warden/web/dashboard.html`

**Customization Options:**
1. Edit CSS variables in `:root` selector
2. Modify auto-refresh interval (default: 2000ms)
3. Change history limit (default: 20 entries)
4. Customize color scheme
5. Add additional metrics

**Example: Change refresh interval:**
```javascript
// In dashboard.html, find:
autoRefreshInterval = setInterval(() => {
    loadStatus();
    loadQueue();
}, 2000);  // Change this value (milliseconds)
```

## Security Considerations

⚠️ **Important:** The dashboard has **no authentication**!

**Recommendations:**
1. Only bind to localhost (`:8080`) in development
2. Use a reverse proxy (nginx/Caddy) with auth in production
3. Enable TLS/HTTPS for remote access
4. Use firewall rules to restrict access
5. Consider OAuth/SSO integration for production

**Production Setup Example:**
```nginx
# nginx reverse proxy with basic auth
location / {
    proxy_pass http://localhost:8080;
    auth_basic "Clawrden Dashboard";
    auth_basic_user_file /etc/nginx/.htpasswd;
}
```

## Browser Support

Tested on:
- ✅ Chrome/Chromium 90+
- ✅ Firefox 88+
- ✅ Safari 14+
- ✅ Edge 90+

**Required Features:**
- ES6 JavaScript (fetch, async/await)
- CSS Grid and Flexbox
- CSS Custom Properties (variables)

## Troubleshooting

### Dashboard not loading
```bash
# Check if warden is running
curl http://localhost:8080/api/status

# Check warden logs
./bin/clawrden-warden --api :8080
```

### Empty queue/history
```bash
# Verify API endpoints
curl http://localhost:8080/api/queue
curl http://localhost:8080/api/history
```

### Auto-refresh not working
- Check browser console for errors
- Verify network connectivity
- Ensure warden is running and accessible

### Can't approve/deny
- Check browser console for errors
- Verify request ID is valid
- Check warden logs for errors

## Future Enhancements

Planned features:
- 🔔 Desktop notifications for new requests
- 🔍 Search and filter history
- 📊 Metrics and charts (commands over time)
- 👥 Multi-user support with authentication
- 🌙 Light/dark theme toggle
- 📱 Mobile-responsive layout
- ⚡ WebSocket real-time updates (instead of polling)
- 📥 Export audit log (CSV/JSON)

## Demo

Run the demo script to see the dashboard in action:

```bash
./scripts/demo.sh
```

This will:
1. Build all binaries
2. Start the warden
3. Open the dashboard in your browser
4. Provide example CLI commands

---

**The dashboard provides a user-friendly interface for managing autonomous agents with human oversight.** 🛡️
