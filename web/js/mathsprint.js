class MathSprintClient {
    constructor() {
        this.ws = null;
        this.playerID = null;
        this.roomID = null;
        this.currentState = null;
        this.currentQuestion = '';
        this.startTime = null;
        this.roundActive = false;
        this.countdownActive = false;
        this.hasSubmitted = false;
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

        this.setupInput();
    }

    setupInput() {
        const input = document.getElementById('answerInput');
        input.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && this.roundActive && !this.hasSubmitted) {
                this.submitAnswer();
            }
        });
    }

    handleMessage(msg) {
        switch (msg.type) {
            case 'welcome':
                this.playerID = msg.playerId;
                this.roomID = msg.roomId;
                console.log('Welcome! Player ID:', this.playerID, 'Room:', this.roomID);
                break;
            case 'mathSprintState':
                this.handleGameState(msg);
                break;
            case 'mathGameSummary':
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

        switch (msg.state) {
            case 'waiting':
                this.showStatusOverlay('Waiting for opponent...');
                break;
                
            case 'ready':
                this.hideStatusOverlay();
                this.countdownActive = false;
                this.roundActive = false;
                this.hasSubmitted = false;
                this.currentQuestion = '';
                document.getElementById('questionDisplay').style.display = 'block';
                document.getElementById('questionText').textContent = 'Get ready...';
                document.querySelector('.input-area').style.display = 'none';
                document.getElementById('resultsArea').style.display = 'none';
                break;
                
            case 'playing':
                // Only start if we have a NEW question and haven't submitted yet
                if (msg.question && msg.question !== this.currentQuestion && !this.hasSubmitted && !this.countdownActive) {
                    this.currentQuestion = msg.question;
                    this.showCountdownBeforeRound(msg.question);
                }
                break;
                
            case 'results':
                this.countdownActive = false;
                this.roundActive = false;
                this.hasSubmitted = false;
                if (msg.roundResult) {
                    this.showResults(msg.roundResult);
                }
                break;
        }
        
        this.currentState = msg.state;
    }

    showCountdownBeforeRound(question) {
        if (this.countdownActive) return;
        
        this.countdownActive = true;
        let countdown = 3;
        
        // Show question faded during countdown
        document.getElementById('questionDisplay').style.display = 'block';
        document.getElementById('questionText').textContent = question;
        document.getElementById('questionText').style.opacity = '0.3';
        document.querySelector('.input-area').style.display = 'none';
        document.getElementById('resultsArea').style.display = 'none';
        
        this.showStatusOverlay(`${countdown}`);
        
        const countdownInterval = setInterval(() => {
            countdown--;
            if (countdown > 0) {
                this.showStatusOverlay(`${countdown}`);
            } else {
                clearInterval(countdownInterval);
                document.getElementById('questionText').style.opacity = '1';
                this.hideStatusOverlay();
                this.countdownActive = false;
                this.startRound(question);
            }
        }, 1000);
    }

    startRound(question) {
        this.roundActive = true;
        this.hasSubmitted = false;
        this.startTime = Date.now();
        
        // Show question and input
        document.getElementById('questionDisplay').style.display = 'block';
        document.getElementById('questionText').textContent = question;
        document.getElementById('questionText').style.opacity = '1';
        document.querySelector('.input-area').style.display = 'block';
        document.getElementById('resultsArea').style.display = 'none';
        
        this.hideStatusOverlay();
        
        // Setup input
        const input = document.getElementById('answerInput');
        input.value = '';
        input.classList.remove('correct', 'wrong');
        input.disabled = false;
        input.focus();
    }

    submitAnswer() {
        if (!this.roundActive || this.hasSubmitted) return;
        
        const input = document.getElementById('answerInput');
        const answer = parseInt(input.value, 10);
        
        if (isNaN(answer)) return;
        
        const timeMs = Date.now() - this.startTime;
        
        this.ws.send(JSON.stringify({
            type: 'mathSprintSubmit',
            answer: answer,
            timeMs: timeMs
        }));
        
        // Mark as submitted - keep input visible with green highlight
        this.hasSubmitted = true;
        this.roundActive = false;
        input.disabled = true;
        input.classList.add('correct'); // Show green - server will validate
        
        // Show waiting message below but keep the input visible
        document.querySelector('.input-hint').textContent = 'Waiting for opponent...';
    }

    showResults(result) {
        this.roundActive = false;
        this.hideStatusOverlay();
        
        // Hide question and input, show results
        document.getElementById('questionDisplay').style.display = 'none';
        document.querySelector('.input-area').style.display = 'none';
        document.getElementById('resultsArea').style.display = 'block';
        
        // Reset input hint for next round
        document.querySelector('.input-hint').textContent = 'Press Enter to submit';
        
        const isPlayer1 = this.playerID === this.playerIDs.player1;
        const myTime = isPlayer1 ? result.player1TimeMs : result.player2TimeMs;
        const oppTime = isPlayer1 ? result.player2TimeMs : result.player1TimeMs;
        
        document.getElementById('yourTime').textContent = `${(myTime / 1000).toFixed(2)}s`;
        document.getElementById('opponentTime').textContent = `${(oppTime / 1000).toFixed(2)}s`;
        document.getElementById('opponentNameResult').textContent = `${this.playerNames.player2}:`;
        document.getElementById('correctAnswer').textContent = `Answer: ${result.correctAnswer}`;
        
        const resultTitle = document.getElementById('resultTitle');
        if (result.winnerId === this.playerID) {
            resultTitle.textContent = '🎉 You won this round!';
            resultTitle.style.color = '#10b981';
        } else if (result.winnerId > 0) {
            resultTitle.textContent = `${this.playerNames.player2} won this round`;
            resultTitle.style.color = '#ef4444';
        } else {
            resultTitle.textContent = "It's a tie!";
            resultTitle.style.color = '#f59e0b';
        }
    }

    showStatusOverlay(text) {
        const overlay = document.getElementById('statusOverlay');
        const statusText = document.getElementById('statusText');
        statusText.textContent = text;
        overlay.style.display = 'flex';
    }

    hideStatusOverlay() {
        document.getElementById('statusOverlay').style.display = 'none';
    }

    showGameSummary(summary) {
        document.querySelector('.game-area').style.display = 'none';
        document.querySelector('.game-header').style.display = 'none';
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
        document.getElementById('summaryPlayer1Avg').textContent = `Avg: ${(myAvgTime / 1000).toFixed(2)}s`;

        document.getElementById('summaryPlayer2Name').textContent = opponentName;
        document.getElementById('summaryPlayer2Score').textContent = `Score: ${opponentScore}`;
        document.getElementById('summaryPlayer2Avg').textContent = `Avg: ${(opponentAvgTime / 1000).toFixed(2)}s`;

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
                    <span class="round-word">${round.question} = ${round.answer}</span>
                </div>
                <div class="round-times">
                    <span class="${iWon ? 'winner' : ''}">${myName}: ${(myTime / 1000).toFixed(2)}s</span>
                    <span class="${theyWon ? 'winner' : ''}">${opponentName}: ${(oppTime / 1000).toFixed(2)}s</span>
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
    window.mathSprintClient = new MathSprintClient();
});
