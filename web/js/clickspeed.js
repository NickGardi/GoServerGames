class ClickSpeedClient {
    constructor() {
        this.ws = null;
        this.playerID = null;
        this.roomID = null;
        this.currentState = null;
        this.startTime = null;
        this.roundActive = false;
        this.waitingForTarget = false;
        this.hasClicked = false;
        this.currentTargetKey = null;
        this.scores = { player1: 0, player2: 0 };
        this.playerIDs = { player1: null, player2: null };
        this.playerNames = { player1: 'You', player2: 'Opponent' };
        
        this.connect();
    }

    connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
        };

        this.ws.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            this.handleMessage(msg);
        };

        this.ws.onclose = () => {
            console.log('WebSocket closed');
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }

    handleMessage(msg) {
        switch (msg.type) {
            case 'welcome':
                this.playerID = msg.playerId;
                this.roomID = msg.roomId;
                console.log('Welcome! Player ID:', this.playerID, 'Room:', this.roomID);
                break;
            case 'clickSpeedState':
                this.handleGameState(msg);
                break;
            case 'clickGameSummary':
                this.showGameSummary(msg);
                break;
        }
    }

    handleGameState(msg) {
        // Update scores and player names
        if (msg.scores) {
            msg.scores.forEach(score => {
                if (score.playerId === this.playerID) {
                    this.scores.player1 = score.score;
                    document.getElementById('player1Score').textContent = score.score;
                    document.getElementById('player1Name').textContent = 'You';
                    this.playerIDs.player1 = score.playerId;
                    this.playerNames.player1 = 'You';
                } else {
                    this.scores.player2 = score.score;
                    document.getElementById('player2Score').textContent = score.score;
                    document.getElementById('player2Name').textContent = score.name || 'Opponent';
                    this.playerIDs.player2 = score.playerId;
                    this.playerNames.player2 = score.name || 'Opponent';
                }
            });
        }

        // Create a unique key for this target position
        const targetKey = `${msg.targetX}-${msg.targetY}`;

        switch (msg.state) {
            case 'waiting':
                this.showArenaOverlay('Waiting for opponent...');
                break;
                
            case 'ready':
                this.hideTarget();
                this.hideResults();
                this.hasClicked = false;
                this.roundActive = false;
                this.waitingForTarget = false;
                this.currentTargetKey = null;
                this.showArenaOverlay('Get ready...');
                break;
                
            case 'playing':
                // Only start if this is a NEW target and we haven't clicked yet
                if (targetKey !== this.currentTargetKey && !this.hasClicked && !this.waitingForTarget) {
                    this.currentTargetKey = targetKey;
                    this.startWaitingForTarget(msg.targetX, msg.targetY, msg.radius);
                }
                break;
                
            case 'results':
                this.roundActive = false;
                this.waitingForTarget = false;
                this.hasClicked = false;
                this.currentTargetKey = null;
                if (msg.roundResult) {
                    this.showResults(msg.roundResult);
                }
                break;
        }
        
        this.currentState = msg.state;
    }

    startWaitingForTarget(targetX, targetY, radius) {
        if (this.waitingForTarget) return;
        
        this.waitingForTarget = true;
        this.hideTarget();
        this.hideResults();
        
        // Show countdown
        let countdown = 3;
        this.showArenaOverlay(`${countdown}`);
        
        const countdownInterval = setInterval(() => {
            countdown--;
            if (countdown > 0) {
                this.showArenaOverlay(`${countdown}`);
            } else {
                clearInterval(countdownInterval);
                this.showArenaOverlay('Click the target!');
                
                // Random delay between 1-3 seconds before target appears
                const delay = 1000 + Math.random() * 2000;
                
                setTimeout(() => {
                    if (this.currentState === 'playing' && !this.roundActive && !this.hasClicked) {
                        this.showTarget(targetX, targetY, radius);
                    }
                }, delay);
            }
        }, 1000);
    }

    showTarget(targetX, targetY, radius) {
        this.roundActive = true;
        this.waitingForTarget = false;
        this.hasClicked = false;
        this.startTime = Date.now();
        
        this.hideArenaOverlay();
        
        const target = document.getElementById('target');
        const arena = document.getElementById('clickArena');
        
        // Calculate pixel position from percentage
        const arenaRect = arena.getBoundingClientRect();
        const x = (targetX / 100) * arenaRect.width;
        const y = (targetY / 100) * arenaRect.height;
        
        target.style.left = `${x}px`;
        target.style.top = `${y}px`;
        target.style.width = `${radius * 2}px`;
        target.style.height = `${radius * 2}px`;
        target.style.display = 'block';
        target.classList.remove('clicked');
        
        // Reset animation
        target.style.animation = 'none';
        target.offsetHeight; // Force reflow
        target.style.animation = 'targetAppear 0.2s ease-out forwards';
        
        // Add click handler
        target.onclick = (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.handleTargetClick();
        };
    }

    handleTargetClick() {
        if (!this.roundActive || this.hasClicked) return;
        
        this.hasClicked = true;
        this.roundActive = false;
        const timeMs = Date.now() - this.startTime;
        
        // Visual feedback - turn green
        const target = document.getElementById('target');
        target.classList.add('clicked');
        target.onclick = null;
        
        // Send to server
        this.ws.send(JSON.stringify({
            type: 'clickSpeedSubmit',
            timeMs: timeMs
        }));
        
        // Show waiting message after animation
        setTimeout(() => {
            if (this.currentState === 'playing') {
                this.hideTarget();
                this.showArenaOverlay('Waiting for opponent...');
            }
        }, 300);
    }

    hideTarget() {
        const target = document.getElementById('target');
        target.style.display = 'none';
        target.onclick = null;
    }

    showArenaOverlay(text) {
        const overlay = document.getElementById('arenaOverlay');
        const statusText = document.getElementById('arenaStatus');
        statusText.textContent = text;
        overlay.style.display = 'flex';
    }

    hideArenaOverlay() {
        document.getElementById('arenaOverlay').style.display = 'none';
    }

    showResults(result) {
        this.roundActive = false;
        this.hideArenaOverlay();
        this.hideTarget();
        
        document.getElementById('resultsArea').style.display = 'block';
        
        const isPlayer1 = this.playerID === this.playerIDs.player1;
        const myTime = isPlayer1 ? result.player1TimeMs : result.player2TimeMs;
        const oppTime = isPlayer1 ? result.player2TimeMs : result.player1TimeMs;
        
        document.getElementById('yourTime').textContent = `${(myTime / 1000).toFixed(3)}s`;
        document.getElementById('opponentTime').textContent = `${(oppTime / 1000).toFixed(3)}s`;
        document.getElementById('opponentNameResult').textContent = `${this.playerNames.player2}:`;
        
        const resultTitle = document.getElementById('resultTitle');
        if (result.winnerId === this.playerID) {
            resultTitle.textContent = '🎯 You won this round!';
            resultTitle.style.color = '#10b981';
        } else if (result.winnerId > 0) {
            resultTitle.textContent = `${this.playerNames.player2} won this round`;
            resultTitle.style.color = '#ef4444';
        } else {
            resultTitle.textContent = "It's a tie!";
            resultTitle.style.color = '#f59e0b';
        }
    }

    hideResults() {
        document.getElementById('resultsArea').style.display = 'none';
    }

    showGameSummary(summary) {
        document.querySelector('.game-area').style.display = 'none';
        document.querySelector('.game-header').style.display = 'none';
        document.getElementById('gameSummary').style.display = 'block';

        const isPlayer1 = this.playerID === summary.player1Id;
        const myName = isPlayer1 ? summary.player1Name : summary.player2Name;
        const opponentName = isPlayer1 ? summary.player2Name : summary.player1Name;
        const myScore = isPlayer1 ? summary.player1Score : summary.player2Score;
        const opponentScore = isPlayer1 ? summary.player2Score : summary.player1Score;
        const myAvgTime = isPlayer1 ? summary.player1AvgTime : summary.player2AvgTime;
        const opponentAvgTime = isPlayer1 ? summary.player2AvgTime : summary.player1AvgTime;

        const winnerDiv = document.getElementById('summaryWinner');
        if (summary.winnerId === this.playerID) {
            winnerDiv.textContent = '🎯 You Won! 🎯';
            winnerDiv.className = 'summary-winner winner';
        } else if (summary.winnerId > 0) {
            winnerDiv.textContent = `${opponentName} Won!`;
            winnerDiv.className = 'summary-winner loser';
        } else {
            winnerDiv.textContent = "It's a Tie!";
            winnerDiv.className = 'summary-winner tie';
        }

        document.getElementById('summaryPlayer1Name').textContent = myName;
        document.getElementById('summaryPlayer1Score').textContent = `Score: ${myScore}`;
        document.getElementById('summaryPlayer1Avg').textContent = `Avg: ${(myAvgTime / 1000).toFixed(3)}s`;

        document.getElementById('summaryPlayer2Name').textContent = opponentName;
        document.getElementById('summaryPlayer2Score').textContent = `Score: ${opponentScore}`;
        document.getElementById('summaryPlayer2Avg').textContent = `Avg: ${(opponentAvgTime / 1000).toFixed(3)}s`;

        const roundsList = document.getElementById('roundsList');
        roundsList.innerHTML = '';
        
        summary.roundHistory.forEach((round) => {
            const roundDiv = document.createElement('div');
            roundDiv.className = 'round-item';
            
            const myTime = isPlayer1 ? round.player1TimeMs : round.player2TimeMs;
            const oppTime = isPlayer1 ? round.player2TimeMs : round.player1TimeMs;
            const iWon = round.winnerId === this.playerID;
            const theyWon = round.winnerId > 0 && round.winnerId !== this.playerID;
            
            roundDiv.innerHTML = `
                <div class="round-header">
                    <span class="round-number">Round ${round.roundNumber}</span>
                </div>
                <div class="round-times">
                    <span class="${iWon ? 'winner' : ''}">${myName}: ${(myTime / 1000).toFixed(3)}s</span>
                    <span class="${theyWon ? 'winner' : ''}">${opponentName}: ${(oppTime / 1000).toFixed(3)}s</span>
                </div>
            `;
            roundsList.appendChild(roundDiv);
        });
        
        // Setup back to lobby button
        const backBtn = document.getElementById('backToLobbyBtn');
        if (backBtn) {
            backBtn.onclick = () => {
                if (this.ws) {
                    this.ws.close();
                }
                window.location.replace('/lobby.html');
            };
        }
    }
}

// Start game when page loads
window.addEventListener('DOMContentLoaded', () => {
    window.clickSpeedClient = new ClickSpeedClient();
});
