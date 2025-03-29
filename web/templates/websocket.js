class WebSocketClient {
    constructor() {
        this.socket = null;
        this.messageHandlers = [];
    }

    connect(token) {
        if (this.token && this.socket.readyState === WebSocket.OPEN) {
            console.warn("WebSocket is already connected.");
            return;
        }

        token = localStorage.getItem("token");
        this.socket = new WebSocket("ws://localhost:2033/ws", [token])

        this.socket.onopen = () => {
            console.log("WebSocket connected");
        };

        this.socket.onmessage = (event) => {
            console.log("Raw WebSocket data:", event.data);

            try {
                const data = JSON.parse(event.data);
                console.log("WebSocket received:", data);
                this.messageHandlers.forEach((handler) => handler(data));
            } catch (error) {
                console.error("Error parsing WebSocket message:", error);
            }
        };

        this.socket.onerror = (error) => {
            console.error("WebSocket error:", error);
        };

        this.socket.onclose = (event) => {
            console.log("WebSocket closed", event.reason)
            this.socket = null;
        };
    }

    sendMessage(message) {
        if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
            console.error("WebSocket is not connected.");
            return;
        }

        try {
            this.socket.send(JSON.stringify(message));
            console.log("WebSocket sent:", message);

            this.messageHandlers.forEach((handler) => handler(message))
        } catch (error) {
            console.error("Error sending WebSocket message:", error);
        }
    }

    addMessageHandler(handler) {
        this.messageHandlers.push(handler);
    }

    close() {
        if (this.socket) {
            this.socket.close();
            this.socket = null;
        }
    }
}

const socket = new WebSocketClient();
export default socket;