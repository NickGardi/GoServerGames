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
        this.currentState = '';
        this.countdownActive = false;
        this.initWebSocket();
        this.setupInput();
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
                    this.hideStatusOverlay();
                } else {
                    // No roomId means we're being returned to login
                    console.log('No roomId - redirecting to login...');
                    window.location.replace('/');
                }
                break;
            case 'redirect':
                // Server is redirecting us to login
                console.log('Redirecting to login:', msg.url);
                window.location.replace(msg.url || '/');
                break;
            case 'lobby':
            case 'lobbyUpdate':
                // Received lobby update while in game - redirect to login
                console.log('Received lobby update, redirecting to login...');
                window.location.replace('/');
                break;
            case 'speedTypeState':
                this.handleGameState(msg);
                if (msg.state === "waiting" || msg.state === "ready") {
                    document.getElementById('gameSummary').style.display = 'none';
                    document.querySelector('.game-area').style.display = 'block';
                    document.querySelector('.game-header').style.display = 'block';
                }
                break;
            case 'gameSummary':
                this.showGameSummary(msg);
                break;
        }
    }

    handleGameState(msg) {
        // Update scores and player names/IDs
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

        // Show results if we receive a results state with roundResult
        if (msg.state === 'results' && msg.roundResult) {
            this.currentState = 'results';
            this.countdownActive = false;
            this.showResults(msg.roundResult);
            return;
        }

        // Handle state transitions
        if (msg.state !== this.currentState) {
            this.currentState = msg.state;

            switch (msg.state) {
                case 'waiting':
                    if (document.getElementById('gameSummary').style.display !== 'block') {
                        this.showStatusOverlay('Waiting for opponent...');
                    }
                    this.countdownActive = false;
                    break;
                case 'ready':
                    this.hideStatusOverlay();
                    this.showStatusOverlay('Get ready! Word starting soon...');
                    this.countdownActive = false;
                    this.roundActive = false;
                    this.currentWord = ''; // Reset for new round
                    break;
                case 'playing':
                    if (!this.roundActive) {
                        document.getElementById('wordDisplay').style.display = 'none';
                        document.getElementById('wordInput').style.display = 'none';
                        document.querySelector('.input-area').style.display = 'none';
                    }
                    if (msg.word && msg.word !== this.currentWord && !this.countdownActive) {
                        this.showCountdownBeforeRound(msg.word);
                    } else if (msg.word && !this.countdownActive && !this.roundActive) {
                        this.startRound(msg.word);
                    }
                    break;
                case 'results':
                    this.countdownActive = false;
                    if (msg.roundResult) {
                        this.showResults(msg.roundResult);
                    }
                    break;
            }
        } else if (msg.state === 'playing' && this.currentState === 'playing') {
            if (msg.word && msg.word !== this.currentWord && !this.roundActive && !this.countdownActive) {
                document.getElementById('wordDisplay').style.display = 'none';
                document.getElementById('wordInput').style.display = 'none';
                document.querySelector('.input-area').style.display = 'none';
                this.showCountdownBeforeRound(msg.word);
            }
        }
    }

    startRound(word) {
        if (word !== this.currentWord) {
            this.currentWord = word;
            this.roundActive = true;
            this.startTime = Date.now();
            this.countdownActive = false;
            
            // Show word and input
            document.getElementById('wordDisplay').style.display = 'block';
            document.getElementById('wordText').textContent = word;
            document.getElementById('wordInput').style.display = 'block';
            document.querySelector('.input-area').style.display = 'block';
            document.getElementById('resultsArea').style.display = 'none';
            
            this.hideStatusOverlay();
            
            // Setup input
            const input = document.getElementById('wordInput');
            input.value = '';
            input.style.color = '';
            input.classList.remove('correct', 'wrong');
            input.disabled = false;
            input.focus();
        }
    }

    showCountdownBeforeRound(word) {
        if (this.countdownActive) return;
        
        this.countdownActive = true;
        let countdown = 3;
        const overlay = document.getElementById('statusOverlay');
        const messageEl = document.getElementById('statusMessage');
        
        document.getElementById('resultsArea').style.display = 'none';
        document.getElementById('wordDisplay').style.display = 'none';
        document.getElementById('wordInput').style.display = 'none';
        document.querySelector('.input-area').style.display = 'none';
        
        messageEl.textContent = `Next round starting in ${countdown}...`;
        overlay.style.display = 'flex';
        
        const countdownInterval = setInterval(() => {
            if (countdown > 0) {
                countdown--;
                if (countdown > 0) {
                    messageEl.textContent = `Next round starting in ${countdown}...`;
                }
            } else {
                clearInterval(countdownInterval);
                this.countdownActive = false;
                this.hideStatusOverlay();
                document.getElementById('wordDisplay').style.display = 'block';
                document.getElementById('wordInput').style.display = 'block';
                document.querySelector('.input-area').style.display = 'block';
                this.startRound(word);
            }
        }, 1000);
    }

    setupInput() {
        const input = document.getElementById('wordInput');
        
        input.addEventListener('input', () => {
            if (input.style.color === 'red' || input.classList.contains('wrong')) {
                input.style.color = '';
                input.classList.remove('wrong');
            }
        });
        
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.keyCode === 13) {
                e.preventDefault();
                if (this.roundActive) {
                    this.submitWord();
                }
            }
        });
        
        document.addEventListener('submit', (e) => {
            e.preventDefault();
        });
    }

    submitWord() {
        if (!this.roundActive || !this.startTime) return;

        const input = document.getElementById('wordInput');
        const typedWord = input.value.trim();
        const timeMs = Date.now() - this.startTime;

        if (typedWord !== this.currentWord) {
            input.style.color = 'red';
            input.classList.add('wrong');
            return;
        }

        // Correct word
        input.style.color = '#10b981';
        input.classList.add('correct');
        this.roundActive = false;
        input.disabled = true;

        this.sendMessage({
            type: 'speedTypeSubmit',
            word: typedWord,
            timeMs: timeMs
        });
    }

    showResults(result) {
        if (!result) return;

        this.roundActive = false;
        this.countdownActive = false;
        this.currentWord = ''; // Reset so next round's word is detected as new
        
        document.getElementById('wordDisplay').style.display = 'block';
        this.hideStatusOverlay();

        const resultsArea = document.getElementById('resultsArea');
        resultsArea.style.display = 'block';

        const player1Time = result.player1TimeMs || 0;
        const player2Time = result.player2TimeMs || 0;
        
        let yourTime = 0;
        let opponentTime = 0;
        
        if (this.playerIDs.player1 === this.playerID) {
            yourTime = player1Time;
            opponentTime = player2Time;
        } else if (this.playerIDs.player2 === this.playerID) {
            yourTime = player2Time;
            opponentTime = player1Time;
        } else {
            yourTime = (result.winnerId === this.playerID) ? player1Time : player2Time;
            opponentTime = (result.winnerId === this.playerID) ? player2Time : player1Time;
        }

        document.getElementById('yourTime').textContent = yourTime > 0 ? (yourTime / 1000).toFixed(2) + 's' : '0.00s';
        document.getElementById('opponentTime').textContent = opponentTime > 0 ? (opponentTime / 1000).toFixed(2) + 's' : '0.00s';

        const winnerDiv = document.getElementById('resultWinner');
        if (result.winnerId && result.winnerId === this.playerID) {
            winnerDiv.textContent = 'You won this round!';
            winnerDiv.className = 'result-winner winner';
        } else if (result.winnerId && result.winnerId > 0) {
            let opponentName = 'Opponent';
            if (this.playerIDs.player1 === result.winnerId) {
                opponentName = this.playerNames.player1;
            } else if (this.playerIDs.player2 === result.winnerId) {
                opponentName = this.playerNames.player2;
            }
            winnerDiv.textContent = `${opponentName} won this round`;
            winnerDiv.className = 'result-winner loser';
        } else {
            winnerDiv.textContent = 'Round ended - no winner';
            winnerDiv.className = 'result-winner';
        }

        const input = document.getElementById('wordInput');
        input.value = '';
        input.style.color = '';
        input.classList.remove('correct', 'wrong');
        input.disabled = true;

        // Auto-hide results after 3 seconds (server will start next round)
        setTimeout(() => {
            if (document.getElementById('gameSummary').style.display !== 'block') {
                document.getElementById('resultsArea').style.display = 'none';
            }
        }, 3000);
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

    showGameSummary(summary) {
        document.querySelector('.game-area').style.display = 'none';
        document.querySelector('.game-header').style.display = 'none';
        document.getElementById('resultsArea').style.display = 'none';
        document.getElementById('statusOverlay').style.display = 'none';
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
            winnerDiv.textContent = '🎉 You Won! 🎉';
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
        document.getElementById('summaryPlayer1Avg').textContent = `Avg Time: ${(myAvgTime / 1000).toFixed(2)}s`;

        document.getElementById('summaryPlayer2Name').textContent = opponentName;
        document.getElementById('summaryPlayer2Score').textContent = `Score: ${opponentScore}`;
        document.getElementById('summaryPlayer2Avg').textContent = `Avg Time: ${(opponentAvgTime / 1000).toFixed(2)}s`;

        const roundsList = document.getElementById('roundsList');
        roundsList.innerHTML = '';
        
        summary.roundHistory.forEach((round) => {
            const roundDiv = document.createElement('div');
            roundDiv.className = 'round-item';
            
            const myTime = isPlayer1 ? round.player1TimeMs : round.player2TimeMs;
            const oppTime = isPlayer1 ? round.player2TimeMs : round.player1TimeMs;
            const roundWinner = round.winnerId === this.playerID ? 'You' : 
                              (round.winnerId > 0 ? opponentName : 'Tie');
            
            roundDiv.innerHTML = `
                <div class="round-header">
                    <span class="round-number">Round ${round.roundNumber}</span>
                    <span class="round-word">Word: "${round.word}"</span>
                </div>
                <div class="round-times">
                    <div class="round-time-item">
                        <span class="round-time-label">${myName}:</span>
                        <span class="round-time-value">${(myTime / 1000).toFixed(2)}s</span>
                    </div>
                    <div class="round-time-item">
                        <span class="round-time-label">${opponentName}:</span>
                        <span class="round-time-value">${(oppTime / 1000).toFixed(2)}s</span>
                    </div>
                </div>
                <div class="round-winner">Winner: ${roundWinner}</div>
            `;
            roundsList.appendChild(roundDiv);
        });
        
        // Setup back to lobby button
        const backBtn = document.getElementById('backToLobbyBtn');
        if (backBtn) {
            backBtn.onclick = () => {
                // Close WebSocket and redirect to lobby
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
    window.speedTypeClient = new SpeedTypeClient();
});
