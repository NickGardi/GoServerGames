document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('loginForm');
    const errorDiv = document.getElementById('error');
    const roomCodeInput = document.getElementById('roomCode');

    // Remove non-alphanumeric characters as user types (allow typing, just filter)
    roomCodeInput.addEventListener('input', function(e) {
        let value = e.target.value.replace(/[^A-Za-z0-9]/g, '');
        if (e.target.value !== value) {
            e.target.value = value;
        }
    });
    
    // Uppercase on blur (when user leaves the field)
    roomCodeInput.addEventListener('blur', function(e) {
        e.target.value = e.target.value.toUpperCase();
    });

    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        const username = document.getElementById('username').value.trim();
        const password = document.getElementById('password').value;
        const roomCode = roomCodeInput.value.trim().toUpperCase().replace(/[^A-Z0-9]/g, '');

        if (!username) {
            errorDiv.textContent = 'Username is required';
            errorDiv.style.display = 'block';
            return;
        }

        if (!password) {
            errorDiv.textContent = 'Password is required';
            errorDiv.style.display = 'block';
            return;
        }

        if (!roomCode || roomCode.length === 0) {
            errorDiv.textContent = 'Room code is required';
            errorDiv.style.display = 'block';
            return;
        }

        errorDiv.textContent = '';
        errorDiv.style.display = 'none';

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
                errorDiv.style.display = 'block';
                console.error('Login failed:', errorText);
            }
        } catch (error) {
            errorDiv.textContent = 'Connection error. Please try again.';
            errorDiv.style.display = 'block';
            console.error('Login error:', error);
        }
    });
});
