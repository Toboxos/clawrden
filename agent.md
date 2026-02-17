📝 Addendum: Developer Implementation Guidelines (v1.2)
=======================================================

**Language Target: Go (Golang)**

These guidelines ensure that the Clawrden system is resilient, handles real-world terminal interactions (like Ctrl+C), and maintains zero-dependency compatibility across any Docker image.

### 1\. The "Universal Static Shim" Pattern (Go)

**Instruction:** Do not write separate binaries for npm, docker, or pip.

*   **Strategy:** Compile one single Go binary named clawrden-shim.
    
*   **Polymorphism:** Use the binary name to determine the tool context.
    
    *   **Logic:** toolName := filepath.Base(os.Args\[0\])
        
    *   **Linking:** The install-clawrden.sh script creates symlinks (or copies) of this one binary into /clawrden/bin/ with the names of the tools it needs to intercept.
        
*   **Build Constraint:** Always compile with CGO\_ENABLED=0 and -ldflags="-s -w" to ensure a small, statically linked binary that runs on Alpine (musl) and Ubuntu (glibc) alike.
    

### 2\. Signal Handling & The "Ctrl+C" Problem

**Instruction:** The Shim must propagate signals to the Warden to prevent orphaned containers.

*   **Scenario:** An agent triggers a long-running npm install. The user interrupts the process via the UI or CLI.
    
*   **Requirement:**
    
    1.  The Shim must use os/signal to listen for syscall.SIGINT and syscall.SIGTERM.
        
    2.  Upon receiving a signal, the Shim sends a "Cancel" frame over the Unix Socket before exiting.
        
    3.  **Warden Logic:** The Warden must use context.WithCancel(context.Background()). When the socket closes or a cancel signal arrives, it calls the cancel function, which must trigger docker stop or container.Kill() on the associated Ghost/Mirror process.
        

### 3\. Real-Time Streaming (Zero-Latency Feedback)

**Instruction:** Use io.Copy and immediate flushing. Do not buffer the command output.

*   **Bad Pattern:** Warden waits for the command to finish, captures the log string, and sends it as one big JSON blob. (The Agent will time out or think the system is hung).
    
*   **Required Pattern:** **Raw Stream Forwarding.**
    
    *   **Warden:** Use container.ExecAttach or container.Attach. This provides a raw net.Conn or io.Reader.
        
    *   **Shim:** Use io.Copy(os.Stdout, socketConn) and io.Copy(os.Stderr, socketConn).
        
    *   **Protocol:** Use a simple framing protocol (e.g., 1 byte for stream type \[1=stdout, 2=stderr\], 4 bytes for length, then the payload) or raw pass-through if the socket is dedicated to a single execution.
        

### 4\. The "Relative Path" Trap

**Instruction:** The Warden must translate the Prisoner's internal paths to the Host's perspective.

*   **Scenario:** Agent is in /app/backend. It runs ls.
    
*   **The Trap:** If the Warden executes ls in a Ghost container that only mounts /app to its root, the Ghost container might start in / and see the wrong files.
    
*   **Warden Logic:**
    
    1.  Receive Cwd from Shim (e.g., /app/backend).
        
    2.  Validate: strings.HasPrefix(payload.Cwd, "/app"). If false, **REJECT** (Security boundary).
        
    3.  Set WorkingDir in the Config of the ContainerCreate or ExecCreate call to the exact path provided by the shim.
        

### 5\. API / Protocol Specification (Unix Socket)

**Instruction:** Use a simple JSON-Header + Raw-Stream body.

*   { "command": "npm", "args": \["install", "express"\], "cwd": "/app", "env": \["NODE\_ENV=development"\], "identity": { "uid": 1000, "gid": 1000 }}
    
*   **Response Response:** The Warden should respond with a 1-byte "Ack" (0=Allowed, 1=Denied, 2=Pending HITL) followed by the stream of execution.
    

### 6\. Security Hardening (Environment Scrubbing)

**Instruction:** The Warden must be a "Confused Deputy" firewall.

*   **The Risk:** An agent might try to inject alias ls='rm -rf /'.
    
*   **The Fix:**
    
    1.  The Shim sends the full environment, but the Warden **must** filter it.
        
    2.  **Allowlist:** Only pass through specific variables (e.g., PATH, LANG, TERM, NODE\_ENV).
        
    3.  **Blocklist:** Explicitly strip LD\_PRELOAD, DOCKER\_HOST, and KUBECONFIG to prevent the Prisoner from hijacking the execution context of the Warden.
        

### 7\. Recommended Go Project Structure

To maintain clean separation between the Shim (Prisoner-side) and Warden (Host-side):

```
/clawrden  
├── cmd/  
│   ├── shim/             # Main entry point for the shim binary  
│   │   └── main.go       # Minimal: calls internal/shim  
│   └── warden/           # Main entry point for the supervisor  
│       └── main.go       # Minimal: calls internal/warden  
├── internal/  
│   ├── shim/             # Logic for socket dialing and signal wrapping  
│   ├── warden/           # The Socket Server and Policy Engine  
│   └── executor/         # Docker SDK wrappers (Mirror vs Ghost)  
├── pkg/  
│   └── protocol/         # Shared JSON structs and constants  
├── scripts/  
│   └── install-clawrden.sh  
├── docker-compose.yml  
└── go.mod   
```

**Note:** When compiling the shim, use GOOS=linux and GOARCH=amd64 (or arm64) to ensure the binary is ready for the container environment regardless of the developer's host OS.

### 8. Version Control

**Instruction:** Use Git for version control.

*   Initialize a git repository if one does not exist.
*   Create meaningful commit messages that describe the changes clearly.
*   Use a `.gitignore` file to exclude build artifacts (bin/), vendor directories, and local configuration.

### 9. System Environment
**Instruction:** You operate in a nix environment. This means that you should use nix commands to manage the environment. Maintain a flake.nix defining your development & production env.


### 10. Documentation
**Instruction:** Document your work while you are working on it and keep the documentation up to date. The documentation should be in the `docs/` directory and should be in markdown format. 

Also maintain a `wip.md` file where you keep track of what you are currently working on and what you plan to work on next. This file works as a "scratchpad" for your thoughts and plans. It is not meant to be a formal document, but rather a place to jot down ideas and plans as you work on them.
