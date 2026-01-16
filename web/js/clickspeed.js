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
        // Update scores and player names - ensure each player sees personalized view
        if (msg.scores && msg.scores.length > 0) {
            // Server sends scores in order: [Players[0], Players[1]]
            // Always map server's Players[0] to player1 (left) and Players[1] to player2 (right)
            if (msg.scores[0]) {
                const isMe = msg.scores[0].playerId === this.playerID;
                this.scores.player1 = msg.scores[0].score;
                document.getElementById('player1Score').textContent = msg.scores[0].score;
                // Show "You" for current player, actual name for opponent
                document.getElementById('player1Name').textContent = isMe ? 'You' : msg.scores[0].name;
                this.playerIDs.player1 = msg.scores[0].playerId;
                // Store actual name (not "You") for use in results - CRITICAL for correct opponent name
                this.playerNames.player1 = msg.scores[0].name;
            }
            
            if (msg.scores[1]) {
                const isMe = msg.scores[1].playerId === this.playerID;
                this.scores.player2 = msg.scores[1].score;
                document.getElementById('player2Score').textContent = msg.scores[1].score;
                // Show "You" for current player, actual name for opponent
                document.getElementById('player2Name').textContent = isMe ? 'You' : msg.scores[1].name;
                this.playerIDs.player2 = msg.scores[1].playerId;
                // Store actual name (not "You") for use in results - CRITICAL for correct opponent name
                this.playerNames.player2 = msg.scores[1].name;
            }
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
                    this.startWaitingForTarget(msg.targetX, msg.targetY, msg.radius, msg.targetAppearDelayMs);
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

    startWaitingForTarget(targetX, targetY, radius, targetAppearDelayMs) {
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
                
                // Use server-controlled delay (default to 2000ms if not provided)
                const delay = targetAppearDelayMs || 2000;
                
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
        
        // Determine which player we are based on player IDs
        const isPlayer1Slot = this.playerID === this.playerIDs.player1;
        
        // The server sends Player1TimeMs and Player2TimeMs based on room slots (Players[0] and Players[1])
        let myTime, oppTime, opponentName;
        
        if (isPlayer1Slot) {
            // We are player1 (left side) - server slot 0
            myTime = result.player1TimeMs;
            oppTime = result.player2TimeMs;
            // Get opponent name from stored playerNames (which has actual names, not "You")
            opponentName = this.playerNames.player2 || 'Opponent';
        } else {
            // We are player2 (right side) - server slot 1
            myTime = result.player2TimeMs;
            oppTime = result.player1TimeMs;
            // Get opponent name from stored playerNames (which has actual names, not "You")
            opponentName = this.playerNames.player1 || 'Opponent';
        }
        
        document.getElementById('yourTime').textContent = `${(myTime / 1000).toFixed(3)}s`;
        document.getElementById('opponentTime').textContent = `${(oppTime / 1000).toFixed(3)}s`;
        document.getElementById('opponentNameResult').textContent = `${opponentName}:`;
        
        const resultTitle = document.getElementById('resultTitle');
        // Use server's winnerId directly - ensures both clients show same result
        if (!result.winnerId || result.winnerId === 0) {
            resultTitle.textContent = "It's a tie!";
            resultTitle.style.color = '#f59e0b';
        } else if (result.winnerId === this.playerID) {
            resultTitle.textContent = '🎯 You won this round!';
            resultTitle.style.color = '#10b981';
        } else {
            // Opponent won - use actual username from stored names
            resultTitle.textContent = `${opponentName} won this round`;
            resultTitle.style.color = '#ef4444';
        }
    }

    hideResults() {
        document.getElementById('resultsArea').style.display = 'none';
    }

    showGameSummary(summary) {
        // Hide game area completely
        document.querySelector('.game-area').style.display = 'none';
        document.querySelector('.game-header').style.display = 'none';
        
        // Show summary - CRITICAL to ensure it's visible
        const summaryDiv = document.getElementById('gameSummary');
        if (summaryDiv) {
            summaryDiv.style.display = 'block';
            summaryDiv.style.visibility = 'visible';
            summaryDiv.style.opacity = '1';
        } else {
            console.error('Game summary div not found!');
        }

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
            
            // Server sends player1TimeMs and player2TimeMs based on room slots (Players[0] and Players[1])
            // We need to map these correctly based on which player we are
            // If we're player1 (summary.player1Id === this.playerID), then player1TimeMs is ours
            const myTime = isPlayer1 ? round.player1TimeMs : round.player2TimeMs;
            const oppTime = isPlayer1 ? round.player2TimeMs : round.player1TimeMs;
            
            // Use server's winnerId directly (don't recalculate) - ensures both clients show same result
            const iWon = round.winnerId === this.playerID;
            const theyWon = round.winnerId > 0 && round.winnerId !== this.playerID;
            const winnerText = iWon ? 'You' : (theyWon ? opponentName : 'Tie');
            
            roundDiv.innerHTML = `
                <div class="round-header">
                    <span class="round-number">Round ${round.roundNumber}</span>
                </div>
                <div class="round-times">
                    <div class="round-time-item">
                        <span class="round-time-label">${myName}</span>
                        <span class="round-time-value ${iWon ? 'winner-text' : ''}">${(myTime / 1000).toFixed(3)}s</span>
                    </div>
                    <div class="round-time-item">
                        <span class="round-time-label">${opponentName}</span>
                        <span class="round-time-value ${theyWon ? 'winner-text' : ''}">${(oppTime / 1000).toFixed(3)}s</span>
                    </div>
                </div>
                <div class="round-winner">Winner: ${winnerText}</div>
            `;
            roundsList.appendChild(roundDiv);
        });
        
        // CRITICAL: Setup back to lobby button - MUST be visible and working
        setTimeout(() => {
            const backBtn = document.getElementById('backToLobbyBtn');
            if (backBtn) {
                console.log('Setting up Back to Lobby button');
                
                // Remove any existing listeners by cloning
                const newBackBtn = backBtn.cloneNode(true);
                backBtn.parentNode.replaceChild(newBackBtn, backBtn);
                
                // Add click handler
                newBackBtn.onclick = (e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    console.log('Back to Lobby button clicked - redirecting...');
                    if (this.ws) {
                        this.ws.close();
                    }
                    window.location.href = '/lobby.html';
                };
                
                // Force visibility with multiple methods
                newBackBtn.style.display = 'block';
                newBackBtn.style.visibility = 'visible';
                newBackBtn.style.opacity = '1';
                newBackBtn.disabled = false;
                newBackBtn.hidden = false;
                
                // Ensure parent section is visible
                const parentSection = newBackBtn.closest('.play-again-section');
                if (parentSection) {
                    parentSection.style.display = 'block';
                    parentSection.style.visibility = 'visible';
                    parentSection.style.opacity = '1';
                }
                
                // Log for debugging
                console.log('Back to Lobby button setup complete');
                console.log('Button visible:', newBackBtn.offsetParent !== null);
                console.log('Button display:', window.getComputedStyle(newBackBtn).display);
            } else {
                console.error('Back to Lobby button element not found in DOM!');
            }
        }, 100); // Small delay to ensure DOM is ready
    }
}

// Start game when page loads
window.addEventListener('DOMContentLoaded', () => {
    window.clickSpeedClient = new ClickSpeedClient();
});
