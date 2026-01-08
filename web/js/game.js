// Game client for browser
class GameClient {
    constructor() {
        this.canvas = document.getElementById('gameCanvas');
        this.renderer = new Renderer(this.canvas);
        this.ws = null;
        this.playerID = 0;
        this.currentSnap = null;
        this.inputSeq = 0;
        this.keys = {};
        this.lastKeys = {}; // Track previous key states
        this.mouseX = 0;
        this.lastMouseX = 0;
        this.lastYaw = 0;
        this.yaw = 0;
        this.ready = false;
        this.lastInputSendTime = 0;
        this.inputSendInterval = 1000 / 20; // Send at 20Hz (50ms intervals)
        this.lastMouseDown = false;

        this.initWebSocket();
        this.initInput();
        this.initLobby();
        this.gameLoop();
    }

    initWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        console.log('Attempting WebSocket connection to:', wsUrl);
        this.ws = new WebSocket(wsUrl);
        
        console.log('WebSocket object created, readyState:', this.ws.readyState);

        this.ws.onopen = () => {
            console.log('WebSocket connected! ReadyState:', this.ws.readyState);
            // Send hello message immediately after connection
            // Use a small delay to ensure connection is fully ready
            setTimeout(() => {
                console.log('Attempting to send hello message...');
                const helloMsg = {
                    type: 'hello',
                    name: 'Player',
                    version: 1
                };
                console.log('Hello message to send:', helloMsg);
                this.sendMessage(helloMsg);
                console.log('Hello message send attempted');
            }, 100);
        };

        this.ws.onmessage = (event) => {
            console.log('Raw message received:', event.data);
            const messages = event.data.split('\n');
            for (const msg of messages) {
                if (msg.trim()) {
                    try {
                        const parsed = JSON.parse(msg);
                        console.log('Parsed message:', parsed);
                        this.handleMessage(parsed);
                    } catch (e) {
                        console.error('Error parsing message:', e, 'Raw:', msg);
                    }
                }
            }
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            console.error('WebSocket readyState:', this.ws.readyState);
        };

        this.ws.onclose = (event) => {
            console.log('WebSocket closed. Code:', event.code, 'Reason:', event.reason, 'WasClean:', event.wasClean);
            setTimeout(() => this.initWebSocket(), 1000); // Reconnect
        };
    }

    sendMessage(msg) {
        if (!this.ws) {
            console.error('Cannot send message - WebSocket is null');
            return;
        }
        
        const states = ['CONNECTING', 'OPEN', 'CLOSING', 'CLOSED'];
        console.log('sendMessage called. readyState:', this.ws.readyState, states[this.ws.readyState]);
        
        if (this.ws.readyState === WebSocket.OPEN) {
            const jsonMsg = JSON.stringify(msg);
            console.log('Sending message:', jsonMsg);
            try {
                this.ws.send(jsonMsg);
                console.log('Message sent successfully');
            } catch (e) {
                console.error('Error sending message:', e);
            }
        } else {
            console.error('Cannot send message - WebSocket not open. State:', this.ws.readyState, states[this.ws.readyState]);
        }
    }

    handleMessage(msg) {
        console.log('Handling message type:', msg.type, msg);
        switch (msg.type) {
            case 'welcome':
                this.playerID = msg.playerId;
                console.log('Received welcome, playerID:', this.playerID);
                if (msg.lobby) {
                    console.log('Lobby in welcome:', msg.lobby);
                    this.updateLobby(msg.lobby);
                }
                break;

            case 'lobby':
                console.log('Lobby update received:', msg.lobby);
                this.updateLobby(msg.lobby);
                break;

            case 'snap':
                this.currentSnap = msg;
                // Check if game started (no lobby in snap means game is active)
                if (!msg.lobby && this.currentSnap.players && this.currentSnap.players.length > 0) {
                    console.log('Game started, hiding lobby');
                    document.getElementById('lobbyOverlay').classList.add('hidden');
                }
                break;
        }
    }

    initLobby() {
        const readyBtn = document.getElementById('readyBtn');
        readyBtn.addEventListener('click', () => {
            this.ready = !this.ready;
            this.sendMessage({
                type: 'ready',
                ready: this.ready
            });
            readyBtn.textContent = this.ready ? 'Not Ready' : 'Ready';
            readyBtn.classList.toggle('ready', this.ready);
        });
    }

    updateLobby(lobby) {
        console.log('Updating lobby UI:', lobby);
        const playersList = document.getElementById('playersList');
        const readyBtn = document.getElementById('readyBtn');

        if (!lobby || !lobby.players || lobby.players.length === 0) {
            console.log('No players in lobby');
            playersList.innerHTML = '<div class="player-item"><span class="player-name">Waiting for players...</span></div>';
            readyBtn.disabled = true;
            return;
        }

        console.log(`Updating lobby with ${lobby.players.length} players`);
        playersList.innerHTML = '';
        for (const player of lobby.players) {
            const item = document.createElement('div');
            item.className = 'player-item';
            item.innerHTML = `
                <span class="player-name">${player.name}</span>
                <span class="player-status ${player.ready ? 'ready' : 'waiting'}">
                    ${player.ready ? 'Ready' : 'Waiting...'}
                </span>
            `;
            playersList.appendChild(item);
        }

        readyBtn.disabled = lobby.players.length < 2;
        console.log(`Ready button disabled: ${readyBtn.disabled}, players: ${lobby.players.length}`);
        document.getElementById('lobbyOverlay').classList.remove('hidden');
    }

    initInput() {
        // Keyboard
        document.addEventListener('keydown', (e) => {
            this.keys[e.key.toLowerCase()] = true;
            if (e.key === 'Escape') {
                // Could show menu or disconnect
            }
        });

        document.addEventListener('keyup', (e) => {
            this.keys[e.key.toLowerCase()] = false;
        });

        // Mouse movement with pointer lock support
        document.addEventListener('mousemove', (e) => {
            if (document.pointerLockElement === this.canvas) {
                // Pointer is locked, use movementX
                const delta = (e.movementX || 0) * 0.15;
                this.yaw += delta;
                while (this.yaw < 0) this.yaw += 360;
                while (this.yaw >= 360) this.yaw -= 360;
            } else {
                // Pointer not locked, use clientX (fallback)
                if (this.lastMouseX !== 0) {
                    const delta = (e.clientX - this.lastMouseX) * 0.15;
                    this.yaw += delta;
                    while (this.yaw < 0) this.yaw += 360;
                    while (this.yaw >= 360) this.yaw -= 360;
                }
                this.lastMouseX = e.clientX;
            }
        });

        // Mouse click (shoot and pointer lock)
        this.mouseDown = false;
        this.canvas.addEventListener('mousedown', () => {
            this.mouseDown = true;
            if (document.pointerLockElement !== this.canvas) {
                this.canvas.requestPointerLock();
            }
        });
        
        document.addEventListener('mouseup', () => {
            this.mouseDown = false;
        });
    }

    sendInput() {
        const now = Date.now();
        
        // Throttle input sending to 20Hz (50ms intervals)
        if (now - this.lastInputSendTime < this.inputSendInterval) {
            return;
        }
        
        const yawDelta = this.yaw - this.lastYaw;
        
        // Check if any movement key is currently pressed
        const hasKeysPressed = this.keys['w'] || this.keys['s'] || this.keys['a'] || this.keys['d'];
        
        // Check if keys changed state (pressed or released)
        const keysChanged = 
            (this.keys['w'] !== this.lastKeys['w']) ||
            (this.keys['s'] !== this.lastKeys['s']) ||
            (this.keys['a'] !== this.lastKeys['a']) ||
            (this.keys['d'] !== this.lastKeys['d']);
        
        // Check if mouse button state changed
        const mouseChanged = this.mouseDown !== this.lastMouseDown;
        
        // Send if:
        // - Keys are currently pressed (for continuous movement)
        // - Keys just changed state (for immediate response)
        // - Mouse state changed
        // - Significant yaw movement (>1 degree)
        const shouldSend = hasKeysPressed || keysChanged || mouseChanged || Math.abs(yawDelta) > 1.0;

        if (shouldSend) {
            const input = {
                type: 'input',
                seq: this.inputSeq++,
                up: this.keys['w'] || false,
                down: this.keys['s'] || false,
                left: this.keys['a'] || false,
                right: this.keys['d'] || false,
                yawDelta: yawDelta,
                shoot: this.mouseDown || false,
                clientTimeMs: now
            };
            this.sendMessage(input);
            this.lastInputSendTime = now;
            
            // Update last states
            this.lastKeys = {...this.keys};
            this.lastMouseDown = this.mouseDown;
            this.lastYaw = this.yaw;
        }
    }

    gameLoop() {
        try {
            // Send input
            this.sendInput();

            // Render
            if (this.currentSnap && this.currentSnap.players) {
                const myPlayer = this.currentSnap.players.find(p => p.id === this.playerID);
                if (myPlayer) {
                    // Extract scores for HUD
                    const scores = (this.currentSnap.players || []).map(p => ({
                        id: p.id,
                        score: p.score || 0
                    }));

                    this.renderer.drawFPSView(
                        myPlayer.x,
                        myPlayer.y,
                        myPlayer.yaw,
                        this.currentSnap.walls || [],
                        this.currentSnap.players || [],
                        this.playerID,
                        scores
                    );
                }
            } else {
                // Clear screen if no snap data yet
                const ctx = this.renderer.ctx;
                ctx.fillStyle = 'rgb(255, 255, 255)';
                ctx.fillRect(0, 0, this.renderer.canvas.width, this.renderer.canvas.height);
            }
        } catch (error) {
            console.error('Error in game loop:', error);
        }

        requestAnimationFrame(() => this.gameLoop());
    }
}

// Start game when page loads
window.addEventListener('DOMContentLoaded', () => {
    new GameClient();
});

