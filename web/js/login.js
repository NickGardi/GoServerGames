document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('loginForm');
    const errorDiv = document.getElementById('error');

    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        const username = document.getElementById('username').value.trim();
        const password = document.getElementById('password').value;
        const roomCode = document.getElementById('roomCode').value.trim().toUpperCase();

        if (!username) {
            errorDiv.textContent = 'Username is required';
            return;
        }

        if (!roomCode) {
            errorDiv.textContent = 'Room code is required';
            return;
        }

        errorDiv.textContent = '';

        try {
            const response = await fetch('/api/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, password, roomCode }),
            });

            if (response.ok) {
                const data = await response.json();
                // Redirect to lobby page
                window.location.href = '/lobby.html';
            } else {
                const errorText = await response.text();
                errorDiv.textContent = errorText || 'Login failed';
            }
        } catch (error) {
            errorDiv.textContent = 'Connection error. Please try again.';
            console.error('Login error:', error);
        }
    });
});
