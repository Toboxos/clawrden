🛡️ Project Clawrden: Technical Requirements Document v1.2
==========================================================

**The Hypervisor for Autonomous Agents**

1\. Executive Summary
---------------------

Clawrden is a sidecar-based governance architecture designed to operationalize "wild" autonomous agents (e.g., OpenDevin, AutoGPT) within a Zero Trust environment. It intercepts agent actions at the binary level, routing them to a privileged supervisor ("The Warden") for policy checks, human-in-the-loop (HITL) approval, and safe execution.

**Core Philosophy:**

*   **Zero Trust:** The Agent ("The Prisoner") is treated as compromised.
    
*   **Transparent Interception:** The Agent believes it is running local commands; in reality, it is triggering remote procedure calls (RPCs).
    
*   **Hot-Pluggable Capabilities:** Tools are injected dynamically via volume mounts without restarting the agent.
    
*   **Universal Compatibility (Go-Based):** The interception layer utilizes statically compiled Go binaries, ensuring compatibility with _any_ Linux-based Docker image (Alpine, Debian, Distroless, Scratch) without requiring external runtimes like Python or Node.js.
    

2\. System Architecture
-----------------------

The system is composed of three distinct Docker-based components running in a mesh.

### A. The Prisoner (Agent Container)

*   **Role:** The untrusted AI logic (LLM + Framework).
    
*   **State:**
    
    *   **Network:** No direct internet access (Firewalled).
        
    *   **Filesystem:** Standard OS, but critical binaries (docker, npm, kubectl) are shadowed by **Clawrden Shims**.
        
    *   **Mounts:**
        
        *   /app: Shared workspace (Read/Write).
            
        *   /clawrden/bin: Read-Only volume containing the **Go Shims** (mounted to PATH).
            
        *   /var/run/clawrden: Directory for the communication socket.
            
*   **Identity:** Runs as a specific non-root user (UID/GID) defined during the install-clawrden process.
    

### B. The Warden (Supervisor Container)

*   **Role:** The privileged executor and policy enforcer.
    
*   **Language:** **Go (Golang)** for high-concurrency handling of socket streams and native Docker SDK integration.
    
*   **Privileges:**
    
    *   Full Internet Access.
        
    *   Docker Socket Access (/var/run/docker.sock).
        
    *   Root-level control over the shared /app workspace.
        
*   **Core Logic:**
    
    1.  Listens on a Unix Domain Socket via Go net.Listen.
        
    2.  Evaluates requests against a Policy Engine (Allow/Ask/Deny).
        
    3.  Executes approved commands via **Mirror** or **Ghost** strategies.
        
    4.  Streams stdout/stderr back to the Prisoner.
        

### C. The Intercom (Communication Layer)

*   **Mechanism:** Unix Domain Socket (preferred) shared via volume /var/run/clawrden.
    
*   **Protocol:** JSON-over-Socket.
    
*   **Payload:** Strict JSON structure including Command, Arguments, ENV, CWD, and **Identity Context** (UID/GID).
    

### D. The User Interface

*   **History:** Logs all commands passed to the Warden.
    
*   **Web Dashboard:** HTTP interface to view the live state.
    
*   **Acceptance Layer:** "Pending" queue for HITL approval.
    
*   **Chat-Integration (Optional):** Slack/Telegram bots.
    

3\. Functional Specifications
-----------------------------

### 3.1. "Ghost Binary" Injection (The Go Shim)

**Requirement:** Agents must be able to use tools (npm, docker) that do not exist in their container.

*   **Implementation Strategy:** "Universal Static Binary".
    
*   **Language:** **Go**.
    
*   **Build Artifact:** A single, statically linked binary (CGO\_ENABLED=0) approx. 3-5MB in size.
    
*   **Mechanism:**
    
    1.  The shim binary is generic; it determines which tool it is impersonating by checking os.Args\[0\] (e.g., if called as npm, it acts as npm).
        
    2.  The Host mounts the compiled binary to /clawrden/bin/npm, /clawrden/bin/docker, etc.
        
*   **Shim Logic (shim.go):**
    
    *   Capture os.Args.
        
    *   Capture os.Getwd(), os.Environ(), os.Getuid(), os.Getgid().
        
    *   Serialize to JSON.
        
    *   Dial the Warden socket (net.Dial("unix", ...)).
        
    *   Stream socket response to os.Stdout/os.Stderr using io.Copy.
        
    *   Exit with the exact exit code returned by the Warden.
        

### 3.2. Execution Strategies

The Warden (written in Go) manages these strategies using the official Docker SDK for Go.

#### Mode A: The Boomerang (Mirror Execution)

*   **Use Case:** Safe, local logic (ls, grep, cat).
    
*   **Logic:**
    
    1.  Warden validates command.
        
    2.  Warden executes command _back inside_ the Prisoner container using ContainerExecCreate.
        
    3.  **Critical Security:** The execution impersonates the **original caller** (UID/GID) to enforce Linux file permissions.
        

#### Mode B: The Ghost Protocol (Ephemeral Execution)

*   **Use Case:** Heavy tools or external access (npm install, terraform).
    
*   **Logic:**
    
    1.  Warden spins up a temporary container (e.g., node:18-alpine).
        
    2.  Mounts the shared /app volume.
        
    3.  Executes the command.
        
    4.  **Artifact Fix:** Upon completion, Warden runs chown -R : on generated files.
        
    5.  Streams logs to Prisoner.
        

### 3.3. Hot-Pluggable Capabilities

*   **Workflow:**
    
    1.  User installs a plugin on the Warden.
        
    2.  Warden symlinks the generic **Go Shim** to a new name (e.g., stripe) in the shared volume.
        
    3.  **Result:** The file instantly appears in /clawrden/bin/stripe inside the Prisoner.
        

### 3.4. The "Clawrdenize" Installation Mechanism

**Requirement:** A standardized script to inject the Go binary and setup paths.

*   FROM ubuntu:latest# No Python or Node dependencies required!COPY install-clawrden.sh /tmp/COPY bin/clawrden-shim /tmp/clawrden-shimRUN chmod +x /tmp/install-clawrden.sh && \\ /tmp/install-clawrden.sh --user 1000 --lock-binaries "npm,docker"
    
*   **Script Responsibilities:**
    
    1.  **Directory Structure:** Creates /clawrden/bin and /var/run/clawrden.
        
    2.  **Binary Installation:** Copies the static /tmp/clawrden-shim to the target location.
        
    3.  **Path Precedence:** Modifies /etc/profile and ~/.bashrc to prepend /clawrden/bin.
        
    4.  **Binary Locking:** Renames original binaries (e.g., mv /usr/bin/npm /usr/bin/npm.original) to prevent bypass.
        

4\. Security & Policy Requirements
----------------------------------

### 4.1. Identity Context

*   The Go Shim MUST capture the UID/GID of the invoking process.
    
*   The Warden MUST use this context to prevent privilege escalation (e.g., preventing a non-root agent from running apt-get install via the shim unless strictly allowed).
    

### 4.2. The Kill Switch

*   The Warden must utilize the Docker SDK to strictly enforce timeouts and provide an API endpoint to ContainerPause or ContainerKill the Prisoner immediately.
    

### 4.3. Human-in-the-Loop (HITL)

*   **Policy Engine:** A Go struct parsing policy.yaml.
    
*   **Concurrency:** The Warden must handle pending requests in a thread-safe map/queue, waiting for a signal from the Web UI (via Go Channels) before proceeding or returning exit code 1.
    

5\. Development Roadmap (Phase 1)
---------------------------------

### Step 1: The Base Infrastructure

*   Create docker-compose.yml.
    
*   Set up a Go module for the project (go mod init clawrden).
    

### Step 2: The Universal Shim (shim.go)

*   Develop the shim.go application.
    
*   Implement JSON serialization of OS context.
    
*   Implement Unix Socket client logic.
    
*   **Action:** Compile static binary: CGO\_ENABLED=0 go build -o bin/shim cmd/shim/main.go.
    

### Step 3: The Warden Core (warden.go)

*   Implement the Socket Server (net.Listen).
    
*   Integrate docker/docker/client SDK.
    
*   Implement the "Mirror" logic.
    

### Step 4: The Installer

*   Write install-clawrden.sh to handle path manipulation and binary copying.
    
*   Verify it works on alpine, ubuntu, and python:slim images.
    

### Step 5: The Interface

*   Build a simple CLI (clawrden-cli) in Go to view the Warden's internal state/queue.