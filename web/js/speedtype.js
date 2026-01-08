// Speed Type Game Client
class SpeedTypeClient {
    constructor() {
        this.ws = null;
        this.playerID = 0;
        this.currentWord = '';
        this.startTime = null;
        this.scores = { player1: 0, player2: 0 };
        this.playerNames = { player1: 'Player 1', player2: 'Player 2' };
        this.playerIDs = { player1: 0, player2: 0 };
        this.roundActive = false;
        this.readyForNextRound = false;
        this.opponentReady = false;
        this.initWebSocket();
        this.setupInput();
        this.setupReadyButton();
    }

    initWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.sendMessage({
                type: 'hello',
                name: 'Player',
                version: 1
            });
        };

        this.ws.onmessage = (event) => {
            try {
                // Server may send multiple JSON messages separated by newlines
                const messages = event.data.split('\n').filter(line => line.trim());
                for (const line of messages) {
                    if (line.trim()) {
                        try {
                            const msg = JSON.parse(line);
                            this.handleMessage(msg);
                        } catch (parseError) {
                            console.error('Error parsing JSON line:', line, parseError);
                        }
                    }
                }
            } catch (error) {
                console.error('Error processing message:', error, 'Raw data:', event.data);
            }
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        this.ws.onclose = () => {
            console.log('WebSocket closed');
            setTimeout(() => this.initWebSocket(), 1000);
        };
    }

    sendMessage(msg) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(msg));
        }
    }

    handleMessage(msg) {
        switch (msg.type) {
            case 'welcome':
                this.playerID = msg.playerId;
                if (msg.roomId) {
                    // Game started
                    this.hideStatusOverlay();
                }
                break;
            case 'speedTypeState':
                this.handleGameState(msg);
                break;
        }
    }

    handleGameState(msg) {
        // Update scores
        if (msg.scores) {
            msg.scores.forEach(score => {
                if (score.playerId === this.playerID) {
                    this.scores.player1 = score.score;
                    document.getElementById('player1Score').textContent = score.score;
                    document.getElementById('player1Name').textContent = score.name || 'You';
                } else {
                    this.scores.player2 = score.score;
                    document.getElementById('player2Score').textContent = score.score;
                    document.getElementById('player2Name').textContent = score.name || 'Opponent';
                }
            });
        }

        switch (msg.state) {
            case 'waiting':
                this.showStatusOverlay('Waiting for opponent...');
                break;
            case 'ready':
                this.hideStatusOverlay();
                this.showStatusOverlay('Get ready! Word starting soon...');
                break;
            case 'playing':
                this.hideStatusOverlay();
                this.startRound(msg.word);
                break;
            case 'results':
                this.showResults(msg.roundResult);
                break;
        }
    }

    startRound(word) {
        // Only reset if this is actually a new word/round
        if (word !== this.currentWord) {
            this.currentWord = word;
            this.roundActive = true;
            this.startTime = Date.now();
            this.readyForNextRound = false; // Reset ready state for new round
            this.opponentReady = false;
            
            document.getElementById('wordText').textContent = word;
            const input = document.getElementById('wordInput');
            input.value = '';
            input.style.color = ''; // Reset color
            input.classList.remove('correct', 'wrong');
            input.disabled = false;
            input.focus();
            document.getElementById('resultsArea').style.display = 'none';
            
            // Hide ready button until results are shown
            const readyBtn = document.getElementById('readyNextRoundBtn');
            if (readyBtn) {
                readyBtn.style.display = 'none';
            }
        }
    }

    setupInput() {
        const input = document.getElementById('wordInput');
        
        // Reset text color when user starts typing (after wrong answer)
        input.addEventListener('input', (e) => {
            // Remove red color when user starts typing
            if (input.style.color === 'red' || input.classList.contains('wrong')) {
                input.style.color = '';
                input.classList.remove('wrong');
            }
        });
        
        // Handle Enter key
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.keyCode === 13) {
                e.preventDefault();
                if (this.roundActive) {
                    this.submitWord();
                }
            }
        });
        
        // Prevent form submission
        document.addEventListener('submit', (e) => {
            e.preventDefault();
        });
    }
    
    setupReadyButton() {
        const readyBtn = document.getElementById('readyNextRoundBtn');
        if (readyBtn) {
            readyBtn.addEventListener('click', () => {
                if (!this.readyForNextRound) {
                    this.readyForNextRound = true;
                    this.updateReadyButton();
                    
                    // Send ready message to server
                    this.sendMessage({
                        type: 'readyForNextRound',
                        ready: true
                    });
                }
            });
        }
    }

    submitWord() {
        if (!this.roundActive || !this.startTime) return;

        const input = document.getElementById('wordInput');
        const typedWord = input.value.trim();
        const timeMs = Date.now() - this.startTime;

        if (typedWord !== this.currentWord) {
            // Wrong word - show red text, allow retry
            input.style.color = 'red';
            input.classList.add('wrong');
            // Don't clear input, let them see what they typed wrong
            return;
        }

        // Correct word - show green text, don't clear input
        input.style.color = '#10b981'; // Green color
        input.classList.add('correct');
        this.roundActive = false;
        input.disabled = true;
        // DON'T clear input - keep the green text visible

        this.sendMessage({
            type: 'speedTypeSubmit',
            word: typedWord,
            timeMs: timeMs
        });
    }

    showResults(result) {
        if (!result) return;

        const resultsArea = document.getElementById('resultsArea');
        resultsArea.style.display = 'block';

        // Get times - result has player1TimeMs and player2TimeMs
        const player1Time = result.player1TimeMs || 0;
        const player2Time = result.player2TimeMs || 0;
        
        // Determine which time is "yours" based on player IDs
        let yourTime = 0;
        let opponentTime = 0;
        
        if (this.playerIDs.player1 === this.playerID) {
            yourTime = player1Time;
            opponentTime = player2Time;
        } else if (this.playerIDs.player2 === this.playerID) {
            yourTime = player2Time;
            opponentTime = player1Time;
        } else {
            // Fallback - use winnerId to determine
            yourTime = (result.winnerId === this.playerID) ? player1Time : player2Time;
            opponentTime = (result.winnerId === this.playerID) ? player2Time : player1Time;
        }

        // Display times (show 0.00s if no time recorded yet)
        document.getElementById('yourTime').textContent = yourTime > 0 ? (yourTime / 1000).toFixed(2) + 's' : '0.00s';
        document.getElementById('opponentTime').textContent = opponentTime > 0 ? (opponentTime / 1000).toFixed(2) + 's' : '0.00s';

        const winnerDiv = document.getElementById('resultWinner');
        if (result.winnerId && result.winnerId === this.playerID) {
            winnerDiv.textContent = 'You won this round!';
            winnerDiv.className = 'result-winner winner';
        } else if (result.winnerId && result.winnerId > 0) {
            winnerDiv.textContent = `${this.playerNames.player1 === 'You' ? this.playerNames.player2 : this.playerNames.player1} won this round`;
            winnerDiv.className = 'result-winner loser';
        } else {
            winnerDiv.textContent = 'Round ended - no winner';
            winnerDiv.className = 'result-winner';
        }

        // Clear input for next round
        const input = document.getElementById('wordInput');
        input.value = '';
        input.style.color = ''; // Reset color
        input.classList.remove('correct', 'wrong');
        input.disabled = true; // Disable until both players ready

        // Reset ready state
        this.readyForNextRound = false;
        this.opponentReady = false;

        // Check for game end (first to 10)
        if (this.scores.player1 >= 10 || this.scores.player2 >= 10) {
            const winnerName = this.scores.player1 >= 10 
                ? (this.playerIDs.player1 === this.playerID ? 'You' : this.playerNames.player1)
                : (this.playerIDs.player2 === this.playerID ? 'You' : this.playerNames.player2);
            winnerDiv.textContent = `${winnerName} won the game! (First to 10)`;
            input.disabled = true;
            
            // Hide ready button on game end
            const readyBtn = document.getElementById('readyNextRoundBtn');
            if (readyBtn) {
                readyBtn.style.display = 'none';
            }
        } else {
            // Show ready button for next round
            const readyBtn = document.getElementById('readyNextRoundBtn');
            if (readyBtn) {
                readyBtn.style.display = 'block';
                readyBtn.textContent = 'Ready for Next Round';
                readyBtn.disabled = false;
                readyBtn.classList.remove('ready', 'both-ready');
            }
            
            // Hide old countdown message
            document.getElementById('nextRoundBtn').style.display = 'none';
        }
    }

    showStatusOverlay(message) {
        const overlay = document.getElementById('statusOverlay');
        const messageEl = document.getElementById('statusMessage');
        messageEl.textContent = message;
        overlay.style.display = 'flex';
    }

    hideStatusOverlay() {
        document.getElementById('statusOverlay').style.display = 'none';
    }
    
    updateReadyButton() {
        const readyBtn = document.getElementById('readyNextRoundBtn');
        if (!readyBtn) return;
        
        if (this.readyForNextRound && this.opponentReady) {
            // Both ready - button shows waiting for game to start
            readyBtn.textContent = 'Both Ready! Starting next round...';
            readyBtn.disabled = true;
            readyBtn.classList.add('both-ready');
        } else if (this.readyForNextRound) {
            // You're ready, waiting for opponent
            readyBtn.textContent = 'Waiting for opponent...';
            readyBtn.disabled = true;
            readyBtn.classList.add('ready');
        } else if (this.opponentReady) {
            // Opponent is ready, you're not
            readyBtn.textContent = 'Opponent Ready - Click to Continue';
            readyBtn.disabled = false;
            readyBtn.classList.remove('ready', 'both-ready');
        } else {
            // Neither ready
            readyBtn.textContent = 'Ready for Next Round';
            readyBtn.disabled = false;
            readyBtn.classList.remove('ready', 'both-ready');
        }
    }
}

// Start game when page loads
window.addEventListener('DOMContentLoaded', () => {
    new SpeedTypeClient();
});

