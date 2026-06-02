package guikit

import (
	"net/http"
)

// ServeJS handles incoming requests for /guikit.js and serves the core real-time engine
func (p *Provider) ServeGK(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(`
(func () {
    // 1. Establish the Real-Time WebSocket Connection to the Core Engine
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = protocol + '//' + window.location.host + '/guikit-ws';
    const socket = new WebSocket(wsUrl);

    socket.onopen = () => {
        console.log('0Trust Cloud UI Engine: Active Connection Established');
    };

    socket.onclose = () => {
        console.warn('0Trust Cloud UI Engine: Connection Terminated');
    };

    socket.onerror = (err) => {
        console.error('0Trust Cloud UI Engine Connection Failure:', err);
    };

    // 2. Incoming Wire Protocol - Process Real-Time Server-Side DML Mutations
    socket.onmessage = (event) => {
        try {
            const dml = JSON.parse(event.data);
            const targetElement = document.getElementById(dml.id);
            
            if (!targetElement) {
                console.warn('DML Target element not found: ' + dml.id);
                return;
            }

            switch (dml.action) {
                case 'setValue':
                    targetElement.innerHTML = dml.value;
                    break;
                case 'append':
                    targetElement.insertAdjacentHTML('beforeend', dml.value);
                    break;
                case 'addClass':
                    targetElement.classList.add(dml.value);
                    break;
                case 'removeClass':
                    targetElement.classList.remove(dml.value);
                    break;
                default:
                    console.error('Unknown DML mutation instruction executed: ' + dml.action);
            }
        } catch (err) {
            console.error('Failed to parse incoming engine payload:', err);
        }
    };

    // 3. Outgoing Wire Protocol - Send Interaction Handlers to Backend
    window.guikitTrigger = function(handlerName) {
        if (socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ action: handlerName }));
        } else {
            console.error('Cannot execute action. Engine socket state is currently closed.');
        }
    };

    // 4. Capture Declarative Event Listeners on DOM Elements
    document.addEventListener('click', function(e) {
        const triggerElement = e.target.closest('[onclick], [data-click]');
        if (!triggerElement) return;

        const actionValue = triggerElement.getAttribute('onclick') || triggerElement.getAttribute('data-click');
        
        // Intercept bare structural naming metrics (e.g. "triggerGreeting")
        if (actionValue && !actionValue.includes('(') && !actionValue.includes(';')) {
            e.preventDefault();
            e.stopImmediatePropagation();
            window.guikitTrigger(actionValue);
        }
    }, true);
})();
`))
}
