// Lobby/Game Selection
class LobbyClient {
    constructor() {
        this.ws = null;
        this.playerID = 0;
        this.players = [];
        this.selectedGame = null;
        this.selectedBy = null; // { playerId, name }
        this.isReady = false;
        this.reconnecting = false;
        this.initWebSocket();
        this.setupGameSelection();
        this.setupReadyButton();
    }

    initWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        // Close existing connection if any
        if (this.ws && this.ws.readyState !== WebSocket.CLOSED) {
            this.ws.close();
        }
        
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            // Send hello message immediately
            this.sendMessage({
                type: 'hello',
                name: 'Player', // Will be replaced with actual name from session
                version: 1
            });
        };

        this.ws.onmessage = (event) => {
            // Server may send multiple JSON messages separated by newlines
            const data = event.data;
            if (typeof data === 'string') {
                // Split by newlines and process each message
                const messages = data.trim().split('\n');
                for (const messageStr of messages) {
                    if (!messageStr.trim()) continue;
                    try {
                        const msg = JSON.parse(messageStr);
                        this.handleMessage(msg);
                    } catch (error) {
                        console.error('Error parsing message:', error, 'Raw data:', messageStr);
                    }
                }
            } else {
                // Handle binary or other data types
                try {
                    const msg = JSON.parse(data);
                    this.handleMessage(msg);
                } catch (error) {
                    console.error('Error parsing message:', error, 'Raw data:', data);
                }
            }
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        this.ws.onclose = () => {
            console.log('WebSocket closed');
            // Only reconnect if we're not already reconnecting
            if (!this.reconnecting) {
                this.reconnecting = true;
                setTimeout(() => {
                    this.reconnecting = false;
                    this.initWebSocket();
                }, 1000);
            }
        };
    }

    sendMessage(msg) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(msg));
        }
    }

    handleMessage(msg) {
        console.log('Received message:', msg.type, msg);
        switch (msg.type) {
            case 'welcome':
                this.playerID = msg.playerId;
                console.log('Welcome received, playerID:', this.playerID, 'lobby:', msg.lobby);
                if (msg.lobby) {
                    console.log('Updating lobby from welcome message with', msg.lobby.players?.length || 0, 'players');
                    this.updateLobby(msg.lobby);
                } else {
                    console.warn('Welcome message received but no lobby data!');
                }
                break;
            case 'lobby':
                console.log('Lobby update received, lobby:', msg.lobby);
                // Handle both msg.lobby (from SnapMessage) and direct lobby object
                const lobbyData = msg.lobby || msg;
                if (lobbyData && (lobbyData.players || lobbyData.Players)) {
                    console.log('Updating lobby from lobby message with', (lobbyData.players || lobbyData.Players).length, 'players');
                    this.updateLobby(lobbyData);
                } else {
                    console.warn('Lobby message received but no player data!');
                }
                break;
            case 'gameSelected':
                this.handleGameSelection(msg);
                break;
            case 'gameStart':
                console.log('Game starting:', msg.gameType);
                if (this.ws) {
                    this.ws.close();
                }
                // Redirect based on game type
                const gamePages = {
                    'speedtype': '/speedtype.html',
                    'mathsprint': '/mathsprint.html',
                    'clickspeed': '/clickspeed.html'
                };
                const page = gamePages[msg.gameType];
                if (page) {
                    window.location.replace(page);
                } else {
                    console.error('Unknown game type:', msg.gameType);
                }
                break;
            case 'speedTypeState':
            case 'mathSprintState':
            case 'clickSpeedState':
                // Ignore game state messages when in lobby
                break;
            default:
                console.log('Unknown message type:', msg.type);
        }
    }

    updateLobby(lobby) {
        if (!lobby) {
            console.log('updateLobby called with null/undefined lobby');
            return;
        }
        
        console.log('updateLobby called with:', lobby);
        const oldSelectedGame = this.selectedGame;
        // Handle both camelCase and PascalCase property names from server
        this.players = lobby.players || lobby.Players || [];
        this.selectedGame = lobby.selectedGame || lobby.SelectedGame || null;
        this.selectedBy = lobby.selectedBy || lobby.SelectedBy || null;
        const lobbyState = lobby.state || lobby.State || 'waiting';
        
        console.log('Updated lobby state - players:', this.players.length, 'selectedGame:', this.selectedGame, 'selectedBy:', this.selectedBy, 'state:', lobbyState);
        
        // Check if game is starting - redirect based on selected game
        if (lobbyState === 'starting' && this.selectedGame) {
            const gamePages = {
                'speedtype': '/speedtype.html',
                'mathsprint': '/mathsprint.html',
                'clickspeed': '/clickspeed.html'
            };
            const page = gamePages[this.selectedGame];
            if (page) {
                console.log('Game starting, redirecting to:', page);
                setTimeout(() => {
                    window.location.replace(page);
                }, 100);
                return;
            }
        }
        
        // Update players list
        this.updatePlayers();
        
        // Update ready button state based on current player's ready status
        const myPlayer = this.players.find(p => p.id === this.playerID);
        if (myPlayer) {
            this.isReady = myPlayer.ready;
            const readyBtn = document.getElementById('readyBtn');
            if (readyBtn) {
                readyBtn.textContent = this.isReady ? 'Unready' : 'Ready';
                readyBtn.classList.toggle('ready-active', this.isReady);
            }
        }
        
        // If selected game changed, update UI
        if (oldSelectedGame !== this.selectedGame) {
            // Reset ready status if game changed
            if (oldSelectedGame !== null && oldSelectedGame !== this.selectedGame) {
                this.isReady = false;
                const readyBtn = document.getElementById('readyBtn');
                if (readyBtn) {
                    readyBtn.textContent = 'Ready';
                    readyBtn.classList.remove('ready-active');
                }
            }
            this.updateGameSelectionStatus();
        } else {
            // Always update to ensure visual state is correct
            this.updateGameSelectionStatus();
        }
        
        // Update ready section
        this.updateReadySection();
    }

    updatePlayers() {
        const playersList = document.getElementById('playersList');
        playersList.innerHTML = '';

        if (this.players.length === 0) {
            playersList.innerHTML = '<div class="player-item"><span>Waiting for players...</span></div>';
            return;
        }

        this.players.forEach((player) => {
            const item = document.createElement('div');
            item.className = 'player-item';
            // Handle both camelCase and PascalCase
            const playerID = player.id || player.ID || 0;
            const playerName = player.name || player.Name || 'Unknown';
            const isReady = player.ready || player.Ready || false;
            const isMe = playerID === this.playerID;
            item.innerHTML = `
                <div class="player-avatar">${playerName.charAt(0).toUpperCase()}</div>
                <div style="flex: 1;">
                    <div class="player-name">
                        ${playerName}${isMe ? ' <span style="font-size: 0.9em; color: #10b981;">(You)</span>' : ''}
                    </div>
                    ${isReady ? '<div style="font-size: 0.85em; color: #10b981; margin-top: 4px;">✓ Ready</div>' : ''}
                </div>
            `;
            playersList.appendChild(item);
        });
        
        // Show waiting message if only one player
        if (this.players.length === 1) {
            const waitingItem = document.createElement('div');
            waitingItem.className = 'player-item';
            waitingItem.style.opacity = '0.6';
            waitingItem.innerHTML = `
                <div class="player-avatar" style="background: #ccc;">?</div>
                <span class="player-name">Waiting for second player...</span>
            `;
            playersList.appendChild(waitingItem);
        }
    }

    updateReadySection() {
        const readySection = document.getElementById('readySection');
        
        if (this.selectedGame) {
            readySection.style.display = 'block';
        } else {
            readySection.style.display = 'none';
        }
    }

    setupGameSelection() {
        const gameCards = document.querySelectorAll('.game-card:not(.disabled)');
        gameCards.forEach(card => {
            card.addEventListener('click', () => {
                const gameType = card.dataset.game;
                this.selectGame(gameType);
            });
        });
    }

    setupReadyButton() {
        const readyBtn = document.getElementById('readyBtn');
        readyBtn.addEventListener('click', () => {
            if (!this.selectedGame) {
                alert('Please select a game first');
                return;
            }
            
            this.isReady = !this.isReady;
            this.sendMessage({
                type: 'ready',
                ready: this.isReady
            });
            
            // Update button text
            readyBtn.textContent = this.isReady ? 'Unready' : 'Ready';
            readyBtn.classList.toggle('ready-active', this.isReady);
        });
    }

    selectGame(gameType) {
        // Remove previous selection
        document.querySelectorAll('.game-card').forEach(card => {
            card.classList.remove('selected');
            const status = card.querySelector('.game-status');
            status.classList.remove('selected', 'waiting');
            status.querySelector('.status-text').textContent = 'Waiting for selection...';
        });

        // Mark selected with "You selected..." text
        const card = document.querySelector(`[data-game="${gameType}"]`);
        if (card) {
            card.classList.add('selected');
            const status = card.querySelector('.game-status');
            status.classList.add('selected');
            status.querySelector('.status-text').textContent = 'You selected this game';
        }

        this.selectedGame = gameType;
        this.sendMessage({
            type: 'selectGame',
            gameType: gameType
        });
    }

    updateGameSelectionStatus() {
        const gameCards = document.querySelectorAll('.game-card:not(.disabled)');
        
        gameCards.forEach(card => {
            // Remove old event listeners by cloning
            const newCard = card.cloneNode(true);
            card.parentNode.replaceChild(newCard, card);
            
            // Both players can select games (if no game selected yet or if it's the selected game)
            const gameType = newCard.dataset.game;
            const isSelected = this.selectedGame === gameType;
            
            // Update visual selection state
            if (isSelected) {
                newCard.classList.add('selected');
                const status = newCard.querySelector('.game-status');
                if (status) {
                    status.classList.add('selected');
                    const statusText = status.querySelector('.status-text');
                    
                    // Show who selected the game
                    if (this.selectedBy) {
                        if (this.selectedBy.playerId === this.playerID) {
                            statusText.textContent = 'You selected this game';
                        } else {
                            statusText.textContent = `${this.selectedBy.name} selected this game`;
                        }
                    } else {
                        statusText.textContent = 'Selected';
                    }
                }
            } else {
                newCard.classList.remove('selected');
                const status = newCard.querySelector('.game-status');
                if (status) {
                    status.classList.remove('selected');
                    status.querySelector('.status-text').textContent = 'Waiting for selection...';
                }
            }
            
            if (this.selectedGame && !isSelected) {
                // Disable other games if one is selected
                newCard.style.cursor = 'not-allowed';
                newCard.style.opacity = '0.5';
            } else {
                // Allow selection
                newCard.style.cursor = 'pointer';
                newCard.style.opacity = '1';
                newCard.addEventListener('click', () => {
                    this.selectGame(gameType);
                });
            }
        });
    }

    handleGameSelection(msg) {
        // Update UI when a player selects a game
        this.selectedGame = msg.gameType;
        
        // Get the player who selected it from the players list
        const selectingPlayer = this.players.find(p => p.id === msg.playerId);
        if (selectingPlayer) {
            this.selectedBy = {
                playerId: msg.playerId,
                name: selectingPlayer.name
            };
        }
        
        // Update game selection status and ready section
        this.updateGameSelectionStatus();
        this.updateReadySection();
        
        // Reset ready status
        this.isReady = false;
        const readyBtn = document.getElementById('readyBtn');
        if (readyBtn) {
            readyBtn.textContent = 'Ready';
            readyBtn.classList.remove('ready-active');
        }
    }
}

// Start lobby when page loads
window.addEventListener('DOMContentLoaded', () => {
    new LobbyClient();
});
